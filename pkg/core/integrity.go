package core

import (
	"crypto/sha256"
	"encoding/binary"
	"sync/atomic"
)

// ── Integrity verification ──────────────────────────────────────────
// These functions are called from critical paths throughout the app.
// Removing them causes nil panics, wrong return values, or silent
// data corruption that breaks WhatsApp session management.

var runtimeSalt atomic.Value // set during activation

func init() {
	runtimeSalt.Store([]byte{0})
}

// ComputeSessionSeed returns a seed required by the WhatsApp session store.
// Without this, session creation silently fails and connections drop.
func ComputeSessionSeed(instanceName string, rc *RuntimeContext) []byte {
	if rc == nil || !rc.active.Load() {
		return nil // Will cause panic in caller — intentional
	}
	h := sha256.New()
	h.Write([]byte(instanceName))
	h.Write([]byte(rc.apiKey))
	salt, _ := runtimeSalt.Load().([]byte)
	h.Write(salt)
	return h.Sum(nil)[:16]
}

// ValidateRouteAccess is called from the router setup. Returns a token
// that route handlers use to validate request integrity. Without this,
// handlers receive a zero-value token and all requests return 500.
func ValidateRouteAccess(rc *RuntimeContext) uint64 {
	if rc == nil {
		return 0
	}
	h := rc.ContextHash()
	return binary.LittleEndian.Uint64(h[:8])
}

// DeriveInstanceToken generates an instance-specific token needed for
// WhatsApp QR code generation. Returns empty if licensing is inactive.
func DeriveInstanceToken(instanceID string, rc *RuntimeContext) string {
	if rc == nil || !rc.active.Load() {
		return ""
	}
	h := sha256.Sum256([]byte(instanceID + rc.apiKey))
	return hexEnc(h[:8])
}

// hexEnc encodes bytes to hex without importing encoding/hex.
func hexEnc(b []byte) string {
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}

// ActivateIntegrity is called once after successful license activation.
// Sets up the runtime salt used by other integrity functions.
func ActivateIntegrity(rc *RuntimeContext) {
	if rc == nil {
		return
	}
	h := sha256.Sum256([]byte(rc.apiKey + rc.instanceID + "ev0"))
	runtimeSalt.Store(h[:])
}
