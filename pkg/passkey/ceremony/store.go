// Package ceremony holds the in-memory state of WhatsApp passkey (WebAuthn)
// pairing ceremonies. It is written by the whatsmeow event goroutine and read
// by the public HTTP polling endpoint, so every access is mutex-guarded.
//
// The lifecycle mirrors the browser-extension contract (tools/passkey-helper):
// the extension polls GET /passkey-ceremony/{token} and drives the ceremony
// through the stages below.
package ceremony

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// Stage is the ceremony state the extension polls for. The values are the
// literal strings the extension's content.js switches on.
const (
	StageChallenge            = "challenge"             // publicKey available; extension runs navigator.credentials.get()
	StageAwaitingConfirmation = "awaiting_confirmation" // response sent to WhatsApp, waiting for the confirmation code
	StageConfirmation         = "confirmation"          // code available; user must verify + confirm
	StageConfirmed            = "confirmed"             // confirmation sent, pairing completing
	StageError                = "error"                 // something failed
)

// defaultTTL mirrors the whatsmeow passkey handoff validity window (~5 min).
const defaultTTL = 5 * time.Minute

// State is the snapshot the poll endpoint serializes back to the extension.
// The extension reads json.data || json, so a flat object is fine; keys match
// what content.js expects.
//
// PublicKey is stored pre-serialized (json.RawMessage) because whatsmeow's
// types.WebAuthnPublicKey already carries the exact json tags the extension
// needs (challenge, rpId, allowCredentials[].{id,type,transports},
// userVerification, base64url-unpadded via jsonbytes). The event goroutine
// json.Marshals the WebAuthnPublicKey once and hands the bytes here — no
// field-by-field remap, no base64 re-encoding mismatch.
type State struct {
	Stage         string          `json:"stage"`
	PublicKey     json.RawMessage `json:"publicKey,omitempty"`
	Code          string          `json:"code,omitempty"`
	SkipHandoffUX bool            `json:"skipHandoffUX"`
	Error         string          `json:"error,omitempty"`
}

type entry struct {
	instanceID string
	state      State
	expiresAt  time.Time
}

// Store keeps token -> ceremony entry. It is shared (pointer) across the
// value-receiver whatsmeowService copies and the HTTP handler, so its internal
// map is protected by a RWMutex.
type Store struct {
	mu      sync.RWMutex
	byToken map[string]*entry // ephemeral ceremony token -> entry
	byInst  map[string]string // instanceID -> active token (for event-goroutine writes)
}

// NewStore builds an empty ceremony store.
func NewStore() *Store {
	return &Store{
		byToken: make(map[string]*entry),
		byInst:  make(map[string]string),
	}
}

func newToken() string {
	b := make([]byte, 32)
	// crypto/rand.Read never returns a short read on supported platforms.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) prune(now time.Time) {
	for tok, e := range s.byToken {
		if now.After(e.expiresAt) {
			delete(s.byToken, tok)
			if s.byInst[e.instanceID] == tok {
				delete(s.byInst, e.instanceID)
			}
		}
	}
}

// Start begins (or restarts) a ceremony for an instance with the challenge
// publicKey (pre-serialized JSON), mints a fresh ephemeral token, and returns
// it. Any previous ceremony for the same instance is replaced.
func (s *Store) Start(instanceID string, pk json.RawMessage) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.prune(now)

	// Drop any previous token for this instance.
	if old, ok := s.byInst[instanceID]; ok {
		delete(s.byToken, old)
	}

	tok := newToken()
	s.byToken[tok] = &entry{
		instanceID: instanceID,
		state:      State{Stage: StageChallenge, PublicKey: pk, SkipHandoffUX: false},
		expiresAt:  now.Add(defaultTTL),
	}
	s.byInst[instanceID] = tok
	return tok
}

// setStateByInstance updates the state of the active ceremony for an instance.
// No-op if the instance has no active ceremony.
func (s *Store) setStateByInstance(instanceID string, mutate func(*State)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tok, ok := s.byInst[instanceID]
	if !ok {
		return
	}
	e, ok := s.byToken[tok]
	if !ok {
		return
	}
	mutate(&e.state)
	e.expiresAt = time.Now().Add(defaultTTL)
}

// SetConfirmation moves the ceremony to the confirmation stage with a code.
// skipHandoffUX is intentionally forced to false by callers (DOC2 §4.2: never
// auto-confirm), so the extension always shows the manual "Confirmar" button.
func (s *Store) SetConfirmation(instanceID, code string, skipHandoffUX bool) {
	s.setStateByInstance(instanceID, func(st *State) {
		st.Stage = StageConfirmation
		st.Code = code
		st.SkipHandoffUX = skipHandoffUX
		st.Error = ""
	})
}

// SetAwaitingConfirmation is set right after the WebAuthn response is submitted.
func (s *Store) SetAwaitingConfirmation(instanceID string) {
	s.setStateByInstance(instanceID, func(st *State) {
		st.Stage = StageAwaitingConfirmation
		st.PublicKey = nil
		st.Error = ""
	})
}

// SetConfirmed marks the confirmation as sent; pairing is finishing.
func (s *Store) SetConfirmed(instanceID string) {
	s.setStateByInstance(instanceID, func(st *State) {
		st.Stage = StageConfirmed
		st.Error = ""
	})
}

// SetError records a ceremony error.
func (s *Store) SetError(instanceID, msg string) {
	s.setStateByInstance(instanceID, func(st *State) {
		st.Stage = StageError
		st.Error = msg
	})
}

// Clear removes the ceremony for an instance (e.g. on PairSuccess). After this,
// the poll endpoint returns not-found, and the extension treats a cleared state
// after having started as "pairing concluded".
func (s *Store) Clear(instanceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if tok, ok := s.byInst[instanceID]; ok {
		delete(s.byToken, tok)
		delete(s.byInst, instanceID)
	}
}

// Lookup resolves a ceremony token to its instance id and current state.
// ok is false if the token is unknown or expired.
func (s *Store) Lookup(token string) (instanceID string, state State, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune(time.Now())
	e, found := s.byToken[token]
	if !found {
		return "", State{}, false
	}
	return e.instanceID, e.state, true
}

// InstanceForToken resolves just the instance id for a token (used by the
// response/confirm endpoints to reach the right whatsmeow client).
func (s *Store) InstanceForToken(token string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.byToken[token]
	if !ok {
		return "", false
	}
	if time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.instanceID, true
}

// StateByInstance returns the active ceremony token and state for an instance,
// or ok=false if there is no non-expired ceremony. Used by the authenticated
// instance/QR poll so the manager can render the passkey UI (the token lets it
// rebuild the #wapk openUrl).
func (s *Store) StateByInstance(instanceID string) (token string, state State, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tok, has := s.byInst[instanceID]
	if !has {
		return "", State{}, false
	}
	e, has := s.byToken[tok]
	if !has {
		return "", State{}, false
	}
	if time.Now().After(e.expiresAt) {
		return "", State{}, false
	}
	return tok, e.state, true
}

// HasActiveByInstance reports whether an instance has a non-expired passkey
// ceremony in progress that is NOT yet in a terminal (error) stage. The QR
// rotation uses this to avoid tearing down the socket/client while a passkey
// ceremony — which is driven by a human in a browser and can easily outlast the
// QR rotation window — is still in flight. Without this, teardownQR would delete
// the client mid-ceremony and the /response and /confirm steps would fail.
func (s *Store) HasActiveByInstance(instanceID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tok, ok := s.byInst[instanceID]
	if !ok {
		return false
	}
	e, ok := s.byToken[tok]
	if !ok {
		return false
	}
	if time.Now().After(e.expiresAt) {
		return false
	}
	// A ceremony in the error stage is terminal — QR teardown may proceed.
	return e.state.Stage != StageError
}
