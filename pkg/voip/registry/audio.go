package registry

import (
	"errors"
	"sync"

	"github.com/EvolutionAPI/evolution-go/pkg/voip/call"
)

// The media stack carries 16 kHz mono PCM in both directions. These are the
// only numbers a caller needs to produce or consume audio correctly.
const (
	// AudioSampleRate is the sample rate of every PCM buffer crossing the bridge.
	AudioSampleRate = 16000
	// AudioChannels is the channel count — mono.
	AudioChannels = 1
)

var (
	// ErrCallNotLive is returned when the call has ended or never existed.
	ErrCallNotLive = errors.New("call is not live")
	// ErrAudioAlreadyAttached guards against two clients fighting over one call's
	// audio: OnPeerAudio is a single sink, so a second attach would silently
	// steal the stream from the first.
	ErrAudioAlreadyAttached = errors.New("another client is already streaming this call's audio")
)

// AudioBridge is a live audio attachment to one call.
//
// It is the piece that was missing for calls to carry sound: the media stack
// already encodes and decodes MLow over SRTP, but nothing fed it microphone
// audio or consumed what the peer sent. A bridge wires both directions for the
// lifetime of one client connection.
type AudioBridge struct {
	registry   *Registry
	instanceID string
	callID     string
	manager    *call.CallManager

	mu     sync.Mutex
	closed bool
}

// AttachAudio connects an audio sink to a live call and returns the bridge used
// to push audio back into it.
//
// onPeerAudio is invoked from the media goroutine with 16 kHz mono PCM decoded
// from the peer. It must not block: anything slow belongs behind a buffer owned
// by the caller, or the whole call's receive path stalls.
func (r *Registry) AttachAudio(instanceID, callID string, onPeerAudio func([]float32)) (*AudioBridge, error) {
	manager, ok := r.manager(instanceID, callID)
	if !ok {
		return nil, ErrCallNotLive
	}

	r.audioMu.Lock()
	defer r.audioMu.Unlock()

	if r.audioAttached == nil {
		r.audioAttached = make(map[string]bool)
	}
	key := instanceID + "\x00" + callID
	if r.audioAttached[key] {
		return nil, ErrAudioAlreadyAttached
	}
	r.audioAttached[key] = true

	manager.OnPeerAudio = onPeerAudio

	return &AudioBridge{
		registry:   r,
		instanceID: instanceID,
		callID:     callID,
		manager:    manager,
	}, nil
}

// Write injects 16 kHz mono PCM into the call, as if it came from a microphone.
// It is a no-op once the bridge is closed, so a late write from a draining
// reader goroutine cannot reach a call that has already ended.
func (b *AudioBridge) Write(pcm []float32) {
	if len(pcm) == 0 {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}

	b.manager.FeedCapturedPCM(pcm)
}

// Close detaches the sink and frees the call for another client. It is safe to
// call more than once.
func (b *AudioBridge) Close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	b.mu.Unlock()

	// Drop the sink first so no further peer audio reaches a dead connection.
	b.manager.OnPeerAudio = nil

	b.registry.audioMu.Lock()
	delete(b.registry.audioAttached, b.instanceID+"\x00"+b.callID)
	b.registry.audioMu.Unlock()
}
