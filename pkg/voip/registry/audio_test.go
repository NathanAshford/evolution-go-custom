package registry

import (
	"testing"

	"github.com/EvolutionAPI/evolution-go/pkg/voip/call"
)

// newRegistryWithLiveCall builds a registry holding one live call whose manager
// is inert, which is enough to exercise the attach/detach bookkeeping without
// touching the network or the media stack.
func newRegistryWithLiveCall(instanceID, callID string) (*Registry, *call.CallManager) {
	manager := &call.CallManager{}
	r := &Registry{
		byInst: map[string]*instanceCalls{
			instanceID: {
				active: map[string]*activeCall{
					callID: {manager: manager},
				},
			},
		},
	}
	return r, manager
}

func TestAttachAudioWiresPeerSink(t *testing.T) {
	r, manager := newRegistryWithLiveCall("i1", "c1")

	got := make(chan []float32, 1)
	bridge, err := r.AttachAudio("i1", "c1", func(pcm []float32) { got <- pcm })
	if err != nil {
		t.Fatalf("AttachAudio: %v", err)
	}
	defer bridge.Close()

	if manager.OnPeerAudio == nil {
		t.Fatal("OnPeerAudio was not installed on the manager")
	}

	manager.OnPeerAudio([]float32{0.5, -0.5})
	select {
	case pcm := <-got:
		if len(pcm) != 2 {
			t.Errorf("got %d samples, want 2", len(pcm))
		}
	default:
		t.Error("peer audio never reached the sink")
	}
}

func TestAttachAudioUnknownCall(t *testing.T) {
	r, _ := newRegistryWithLiveCall("i1", "c1")

	if _, err := r.AttachAudio("i1", "outra", nil); err != ErrCallNotLive {
		t.Errorf("unknown call: got %v, want ErrCallNotLive", err)
	}
	if _, err := r.AttachAudio("outra", "c1", nil); err != ErrCallNotLive {
		t.Errorf("unknown instance: got %v, want ErrCallNotLive", err)
	}
}

// Two clients on one call would silently fight over a single OnPeerAudio sink,
// the second stealing audio from the first. The second attach must be refused.
func TestAttachAudioRejectsSecondClient(t *testing.T) {
	r, _ := newRegistryWithLiveCall("i1", "c1")

	first, err := r.AttachAudio("i1", "c1", nil)
	if err != nil {
		t.Fatalf("first attach: %v", err)
	}

	if _, err := r.AttachAudio("i1", "c1", nil); err != ErrAudioAlreadyAttached {
		t.Fatalf("second attach: got %v, want ErrAudioAlreadyAttached", err)
	}

	// Closing frees the call for the next client.
	first.Close()
	second, err := r.AttachAudio("i1", "c1", nil)
	if err != nil {
		t.Fatalf("attach after close: %v", err)
	}
	second.Close()
}

// A reader goroutine draining its buffer can call Write after the call ended;
// that must not reach the manager.
func TestBridgeWriteAfterCloseIsNoop(t *testing.T) {
	r, manager := newRegistryWithLiveCall("i1", "c1")

	bridge, err := r.AttachAudio("i1", "c1", nil)
	if err != nil {
		t.Fatalf("AttachAudio: %v", err)
	}

	bridge.Close()
	if manager.OnPeerAudio != nil {
		t.Error("Close left the peer sink installed")
	}

	bridge.Write([]float32{1, 2, 3}) // must not panic or reach the manager
	bridge.Close()                   // idempotent
}

func TestBridgeWriteEmptyIsNoop(t *testing.T) {
	r, _ := newRegistryWithLiveCall("i1", "c1")

	bridge, err := r.AttachAudio("i1", "c1", nil)
	if err != nil {
		t.Fatalf("AttachAudio: %v", err)
	}
	defer bridge.Close()

	bridge.Write(nil)
	bridge.Write([]float32{})
}
