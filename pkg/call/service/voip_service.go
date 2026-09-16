package call_service

import (
	"context"
	"fmt"
	"strings"
	"time"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	"github.com/EvolutionAPI/evolution-go/pkg/voip/core"
	voip_registry "github.com/EvolutionAPI/evolution-go/pkg/voip/registry"
	"go.mau.fi/whatsmeow/types"
)

// OfferCallStruct is the body for POST /call/offer.
type OfferCallStruct struct {
	// Recipient phone number with country code (digits only), or a full JID.
	Number string `json:"number" example:"5511999999999"`
	// Video places a video call instead of a voice call. WhatsApp requires a
	// video track for these, so voice is the supported path.
	Video bool `json:"video,omitempty" example:"false"`
}

// CallActionStruct is the body for accept/terminate/hangup by call id.
type CallActionStruct struct {
	// CallID as returned by POST /call/offer or the CallOffer webhook.
	CallID string `json:"callId" example:"A1B2C3D4E5F6"`
	// Reason is optional; defaults to user_ended (terminate) or declined (reject).
	Reason string `json:"reason,omitempty" example:"user_ended"`
}

// OfferCall places an outgoing WhatsApp call and returns the call snapshot.
//
// The call rings on the peer's device. Audio is silence-keepalive until PCM is
// fed into the call, which the HTTP API does not currently expose.
func (c *callService) OfferCall(data *OfferCallStruct, instance *instance_model.Instance) (*voip_registry.CallSnapshot, error) {
	// Validate the request before touching the client: starting an instance can
	// take seconds, and a malformed number should not pay that cost or be
	// reported as a connection problem.
	if data == nil || strings.TrimSpace(data.Number) == "" {
		return nil, fmt.Errorf("number is required")
	}

	peer, err := parsePeerJID(data.Number)
	if err != nil {
		return nil, err
	}

	if peer.Server == types.GroupServer {
		return nil, fmt.Errorf("group calls are not supported")
	}

	client, err := c.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	// A connected-but-unpaired client can open the websocket yet cannot query or
	// call anything; without this guard the USync lookup below blocks until its
	// own timeout instead of failing fast.
	if !client.IsLoggedIn() {
		return nil, fmt.Errorf("instance is not logged in to WhatsApp; scan the QR code first")
	}

	// Verify the number is actually on WhatsApp before ringing — otherwise the
	// offer is sent into the void and the caller gets no useful error. A lookup
	// failure is not fatal; the call attempt still proceeds.
	lookupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if resp, err := client.IsOnWhatsApp(lookupCtx, []string{peer.User}); err == nil &&
		len(resp) > 0 && !resp[0].IsIn {
		return nil, fmt.Errorf("number %s is not registered on WhatsApp", peer.User)
	}

	snap, err := c.whatsmeowService.CallRegistry().StartCall(
		context.Background(), instance.Id, client, peer, data.Video,
	)
	if err != nil {
		c.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Failed to place call: %v", instance.Id, err)
		return nil, err
	}

	c.loggerWrapper.GetLogger(instance.Id).LogInfo(
		"[%s] Outgoing call %s placed to %s", instance.Id, snap.CallID, peer.String(),
	)

	return snap, nil
}

// AttachAudio wires a live audio stream to a call.
//
// The media stack always encodes and decodes MLow over SRTP; what was missing
// was anything feeding it microphone audio or consuming the peer's. This hands
// the caller both directions for the lifetime of one connection.
func (c *callService) AttachAudio(callID string, instance *instance_model.Instance, onPeerAudio func([]float32)) (*voip_registry.AudioBridge, error) {
	if callID == "" {
		return nil, fmt.Errorf("callId is required")
	}
	if _, err := c.ensureClientConnected(instance.Id); err != nil {
		return nil, err
	}
	return c.whatsmeowService.CallRegistry().AttachAudio(instance.Id, callID, onPeerAudio)
}

// AcceptCall answers a ringing inbound call.
func (c *callService) AcceptCall(data *CallActionStruct, instance *instance_model.Instance) error {
	if data == nil || data.CallID == "" {
		return fmt.Errorf("callId is required")
	}
	if _, err := c.ensureClientConnected(instance.Id); err != nil {
		return err
	}
	return c.whatsmeowService.CallRegistry().Accept(context.Background(), instance.Id, data.CallID)
}

// TerminateCall hangs up a call that is ringing or active.
func (c *callService) TerminateCall(data *CallActionStruct, instance *instance_model.Instance) error {
	if data == nil || data.CallID == "" {
		return fmt.Errorf("callId is required")
	}
	if _, err := c.ensureClientConnected(instance.Id); err != nil {
		return err
	}
	return c.whatsmeowService.CallRegistry().Terminate(
		context.Background(), instance.Id, data.CallID, core.EndCallReason(data.Reason),
	)
}

// RejectCallByID declines a ringing inbound call tracked by the VoIP registry.
func (c *callService) RejectCallByID(data *CallActionStruct, instance *instance_model.Instance) error {
	if data == nil || data.CallID == "" {
		return fmt.Errorf("callId is required")
	}
	if _, err := c.ensureClientConnected(instance.Id); err != nil {
		return err
	}
	return c.whatsmeowService.CallRegistry().Reject(
		context.Background(), instance.Id, data.CallID, core.EndCallReason(data.Reason),
	)
}

// ListCalls returns the live calls of an instance.
func (c *callService) ListCalls(instance *instance_model.Instance) []voip_registry.CallSnapshot {
	return c.whatsmeowService.CallRegistry().Active(instance.Id)
}

// CallHistory returns recently finished calls, oldest first.
func (c *callService) CallHistory(instance *instance_model.Instance) []voip_registry.CallSnapshot {
	return c.whatsmeowService.CallRegistry().History(instance.Id)
}

// GetCall returns one call by id, live or finished.
func (c *callService) GetCall(callID string, instance *instance_model.Instance) (*voip_registry.CallSnapshot, error) {
	snap, ok := c.whatsmeowService.CallRegistry().Get(instance.Id, callID)
	if !ok {
		return nil, voip_registry.ErrUnknownCall
	}
	return snap, nil
}

// parsePeerJID accepts a bare number or a full JID and normalizes it.
func parsePeerJID(number string) (types.JID, error) {
	number = strings.TrimSpace(number)

	if strings.Contains(number, "@") {
		jid, err := types.ParseJID(number)
		if err != nil {
			return types.EmptyJID, fmt.Errorf("invalid JID %q: %w", number, err)
		}
		return jid, nil
	}

	var digits strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	if digits.Len() < 8 {
		return types.EmptyJID, fmt.Errorf("invalid phone number %q: expected country code + number, digits only", number)
	}

	return types.NewJID(digits.String(), types.DefaultUserServer), nil
}
