// Package registry tracks the live WhatsApp calls of every instance.
//
// A CallManager drives exactly one call end to end, so an instance placing or
// receiving several calls needs one manager per call. This package owns that
// bookkeeping, keyed by instance id and then by call id, and keeps a short
// history so the API can report calls that have already ended.
package registry

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/EvolutionAPI/evolution-go/pkg/voip/call"
	"github.com/EvolutionAPI/evolution-go/pkg/voip/core"
	"github.com/EvolutionAPI/evolution-go/pkg/voip/signaling"
	"github.com/EvolutionAPI/evolution-go/pkg/voip/transport"
	"github.com/EvolutionAPI/evolution-go/pkg/voip/wa"
	"github.com/EvolutionAPI/evolution-go/pkg/voip/wanode"
	"go.mau.fi/whatsmeow"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

// historyLimit caps the per-instance ring of finished calls.
const historyLimit = 50

// CallSnapshot is a serializable view of a call, safe to hand to HTTP handlers
// and webhooks without exposing the live CallManager.
type CallSnapshot struct {
	CallID      string  `json:"callId"`
	InstanceID  string  `json:"instanceId"`
	PeerJID     string  `json:"peerJid"`
	Number      string  `json:"number"`
	Direction   string  `json:"direction"`
	MediaType   string  `json:"mediaType"`
	State       string  `json:"state"`
	EndReason   string  `json:"endReason,omitempty"`
	CreatedAt   string  `json:"createdAt"`
	AcceptedAt  *string `json:"acceptedAt,omitempty"`
	EndedAt     *string `json:"endedAt,omitempty"`
	DurationSec int     `json:"durationSeconds"`
}

func snapshot(instanceID string, info *call.CallInfo) CallSnapshot {
	snap := CallSnapshot{
		CallID:      info.CallID,
		InstanceID:  instanceID,
		PeerJID:     info.PeerJid,
		Number:      wanode.MustJID(info.PeerJid).User,
		Direction:   string(info.Direction),
		MediaType:   string(info.MediaType),
		State:       string(info.StateData.State),
		EndReason:   string(info.StateData.EndReason),
		CreatedAt:   info.CreatedAt.Format(time.RFC3339),
		DurationSec: info.StateData.DurationSecs,
	}
	if t := info.StateData.AcceptedAt; t != nil {
		s := t.Format(time.RFC3339)
		snap.AcceptedAt = &s
	}
	if t := info.StateData.EndedAt; t != nil {
		s := t.Format(time.RFC3339)
		snap.EndedAt = &s
	}
	return snap
}

// StateListener is notified whenever a call changes state, so the caller can
// fan the change out to webhooks.
type StateListener func(instanceID string, snap CallSnapshot)

type activeCall struct {
	manager *call.CallManager
	info    *call.CallInfo
}

type instanceCalls struct {
	active  map[string]*activeCall
	history []CallSnapshot
}

// Registry owns every instance's live and recent calls.
type Registry struct {
	mu       sync.RWMutex
	byInst   map[string]*instanceCalls
	log      *slog.Logger
	listener StateListener

	// maxPerInstance bounds concurrent calls per instance; further inbound
	// offers are rejected instead of piling up.
	maxPerInstance int

	// proxyFor resolves an instance's media proxy. Injected so this package
	// does not depend on the instance repository.
	proxyFor func(instanceID string) *transport.ProxyConfig

	// audioAttached tracks which calls already have an audio bridge, keyed by
	// instance+call. A CallManager exposes a single OnPeerAudio sink, so a
	// second attach would silently steal the stream from the first client.
	audioMu       sync.Mutex
	audioAttached map[string]bool
}

// SetProxyResolver installs the lookup used to route call media through an
// instance's configured proxy.
func (r *Registry) SetProxyResolver(fn func(instanceID string) *transport.ProxyConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.proxyFor = fn
}

func NewRegistry(log *slog.Logger) *Registry {
	if log == nil {
		log = slog.Default()
	}
	return &Registry{
		byInst:         make(map[string]*instanceCalls),
		log:            log,
		maxPerInstance: 5,
	}
}

// SetStateListener installs the callback used to publish call state changes.
func (r *Registry) SetStateListener(fn StateListener) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listener = fn
}

func (r *Registry) bucket(instanceID string) *instanceCalls {
	b, ok := r.byInst[instanceID]
	if !ok {
		b = &instanceCalls{active: make(map[string]*activeCall)}
		r.byInst[instanceID] = b
	}
	return b
}

// create registers a new CallManager for a call id and wires its callbacks.
func (r *Registry) create(instanceID, callID string, cli *whatsmeow.Client) *call.CallManager {
	manager := call.NewCallManager(wa.NewSocket(cli), r.log)

	// Route media through the instance proxy when one is configured.
	r.mu.RLock()
	resolver := r.proxyFor
	r.mu.RUnlock()
	if resolver != nil {
		if proxy := resolver(instanceID); proxy != nil {
			manager.SetProxy(proxy)
		}
	}

	record := func(info *call.CallInfo) {
		if info == nil {
			return
		}

		r.mu.Lock()
		if ac, ok := r.bucket(instanceID).active[callID]; ok {
			ac.info = info
		}
		listener := r.listener
		r.mu.Unlock()

		snap := snapshot(instanceID, info)
		if listener != nil {
			listener(instanceID, snap)
		}

		if info.IsEnded() {
			r.finish(instanceID, callID, snap)
		}
	}

	manager.OnStateChange = record
	manager.OnIncoming = record
	manager.OnEnded = record

	r.mu.Lock()
	r.bucket(instanceID).active[callID] = &activeCall{manager: manager}
	r.mu.Unlock()

	return manager
}

// finish moves a call from the active map into the history ring.
func (r *Registry) finish(instanceID, callID string, snap CallSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	b := r.bucket(instanceID)
	delete(b.active, callID)

	b.history = append(b.history, snap)
	if len(b.history) > historyLimit {
		b.history = b.history[len(b.history)-historyLimit:]
	}
}

// StartCall places an outgoing call and returns its snapshot.
func (r *Registry) StartCall(ctx context.Context, instanceID string, cli *whatsmeow.Client, peer types.JID, isVideo bool) (*CallSnapshot, error) {
	callID := signaling.GenerateCallID()
	manager := r.create(instanceID, callID, cli)

	if err := manager.StartCall(ctx, callID, peer, isVideo); err != nil {
		r.mu.Lock()
		delete(r.bucket(instanceID).active, callID)
		r.mu.Unlock()
		return nil, err
	}

	info := manager.CurrentCall()
	if info == nil {
		return nil, errNoCall
	}

	snap := snapshot(instanceID, info)
	return &snap, nil
}

// Accept answers a ringing inbound call.
func (r *Registry) Accept(ctx context.Context, instanceID, callID string) error {
	manager, ok := r.manager(instanceID, callID)
	if !ok {
		return errUnknownCall
	}
	return manager.AcceptCall(ctx, callID)
}

// Reject declines a ringing inbound call.
func (r *Registry) Reject(ctx context.Context, instanceID, callID string, reason core.EndCallReason) error {
	manager, ok := r.manager(instanceID, callID)
	if !ok {
		return errUnknownCall
	}
	if reason == "" {
		reason = core.EndCallReasonDeclined
	}
	return manager.RejectCall(ctx, callID, reason)
}

// Terminate hangs up a call that is ringing or already active.
func (r *Registry) Terminate(ctx context.Context, instanceID, callID string, reason core.EndCallReason) error {
	manager, ok := r.manager(instanceID, callID)
	if !ok {
		return errUnknownCall
	}
	if reason == "" {
		reason = core.EndCallReasonUserEnded
	}
	return manager.EndCall(ctx, reason)
}

func (r *Registry) manager(instanceID, callID string) (*call.CallManager, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.byInst[instanceID]
	if !ok {
		return nil, false
	}
	ac, ok := b.active[callID]
	if !ok {
		return nil, false
	}
	return ac.manager, true
}

// Active returns the live calls of an instance.
func (r *Registry) Active(instanceID string) []CallSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.byInst[instanceID]
	if !ok {
		return []CallSnapshot{}
	}

	out := make([]CallSnapshot, 0, len(b.active))
	for _, ac := range b.active {
		if ac.info != nil {
			out = append(out, snapshot(instanceID, ac.info))
			continue
		}
		if info := ac.manager.CurrentCall(); info != nil {
			out = append(out, snapshot(instanceID, info))
		}
	}
	return out
}

// History returns recently finished calls, newest last.
func (r *Registry) History(instanceID string) []CallSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.byInst[instanceID]
	if !ok {
		return []CallSnapshot{}
	}

	out := make([]CallSnapshot, len(b.history))
	copy(out, b.history)
	return out
}

// Get returns a single call, looking in the active set first and then history.
func (r *Registry) Get(instanceID, callID string) (*CallSnapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	b, ok := r.byInst[instanceID]
	if !ok {
		return nil, false
	}

	if ac, ok := b.active[callID]; ok {
		info := ac.info
		if info == nil {
			info = ac.manager.CurrentCall()
		}
		if info != nil {
			snap := snapshot(instanceID, info)
			return &snap, true
		}
	}

	for i := len(b.history) - 1; i >= 0; i-- {
		if b.history[i].CallID == callID {
			snap := b.history[i]
			return &snap, true
		}
	}

	return nil, false
}

// DropInstance forgets everything about an instance (used on delete/logout).
func (r *Registry) DropInstance(instanceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byInst, instanceID)
}

// --- inbound event routing ---

// WrapCallNode rebuilds the <call from="..."> envelope that the signaling layer
// expects; whatsmeow delivers only the inner node on call events.
func WrapCallNode(from types.JID, inner *waBinary.Node) *waBinary.Node {
	content := []waBinary.Node{}
	if inner != nil {
		content = append(content, *inner)
	}
	return &waBinary.Node{
		Tag:     "call",
		Attrs:   waBinary.Attrs{"from": from},
		Content: content,
	}
}

func callIDFromNode(node *waBinary.Node) string {
	info := signaling.ExtractNodeInfo(node)
	if info == nil {
		return ""
	}
	return info.CallID
}

// HandleOffer routes an inbound call offer, creating a manager for it. Offers
// beyond the per-instance limit are rejected so a flood can't exhaust memory.
func (r *Registry) HandleOffer(ctx context.Context, instanceID string, cli *whatsmeow.Client, from types.JID, data *waBinary.Node) {
	node := WrapCallNode(from, data)
	callID := callIDFromNode(node)
	if callID == "" {
		return
	}

	r.mu.RLock()
	count := 0
	if b, ok := r.byInst[instanceID]; ok {
		count = len(b.active)
	}
	r.mu.RUnlock()

	if r.maxPerInstance > 0 && count >= r.maxPerInstance {
		r.rejectOffer(ctx, cli, node, from)
		return
	}

	manager := r.create(instanceID, callID, cli)
	manager.HandleCallOffer(ctx, node, from)
}

func (r *Registry) rejectOffer(ctx context.Context, cli *whatsmeow.Client, node *waBinary.Node, from types.JID) {
	info := signaling.ExtractNodeInfo(node)
	if info == nil {
		return
	}

	creator := wanode.AttrString(info.InnerNode.Attrs, "call-creator")
	if creator == "" {
		creator = from.String()
	}

	reject := signaling.BuildRejectStanza(from, info.CallID, wanode.MustJID(creator))
	_ = wa.NewSocket(cli).SendNode(ctx, reject)
	r.log.Info("inbound call rejected: instance at capacity", "call_id", info.CallID)
}

// HandleAccept, HandleTransport and HandleTerminate forward signaling nodes to
// the manager that owns the call. Nodes for unknown calls are dropped.
func (r *Registry) HandleAccept(ctx context.Context, instanceID string, from types.JID, data *waBinary.Node) {
	node := WrapCallNode(from, data)
	if manager, ok := r.manager(instanceID, callIDFromNode(node)); ok {
		manager.HandleCallAccept(ctx, node, from)
	}
}

func (r *Registry) HandleTransport(ctx context.Context, instanceID string, from types.JID, data *waBinary.Node) {
	node := WrapCallNode(from, data)
	if manager, ok := r.manager(instanceID, callIDFromNode(node)); ok {
		manager.HandleCallTransport(ctx, node, from)
	}
}

func (r *Registry) HandleTerminate(instanceID string, from types.JID, data *waBinary.Node) {
	node := WrapCallNode(from, data)
	if manager, ok := r.manager(instanceID, callIDFromNode(node)); ok {
		manager.HandleCallTerminate(node)
	}
}
