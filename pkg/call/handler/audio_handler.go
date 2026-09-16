package call_handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/EvolutionAPI/evolution-go/pkg/voip/media"
	voip_registry "github.com/EvolutionAPI/evolution-go/pkg/voip/registry"
)

// The bridge is deliberately tolerant on the way in and strict on the way out:
// a client that stops reading must not be allowed to stall the call's media
// goroutine, so its outbound queue is bounded and overflow drops audio.
const (
	audioSendQueue   = 64
	audioWriteWait   = 5 * time.Second
	audioPongWait    = 60 * time.Second
	audioPingPeriod  = 25 * time.Second
	audioMaxReadSize = 64 * 1024
)

// audioUpgrader accepts any origin: this endpoint is authenticated by the same
// instance token as the rest of the API, and browsers are not the only client.
var audioUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// StreamAudio streams a call's audio in both directions over a WebSocket.
//
// @Summary Stream call audio
// @Description Opens a WebSocket carrying the call's audio in both directions as raw
// @Description 16 kHz mono signed 16-bit little-endian PCM in binary frames.
// @Description Binary frames sent by the client are injected into the call as if they
// @Description came from a microphone; binary frames sent by the server are the peer's
// @Description audio. Only one client may stream a given call at a time.
// @Tags Call
// @Param callId path string true "Call id returned by /call/offer"
// @Success 101 {string} string "Switching protocols"
// @Failure 400 {object} gin.H "Call is not live"
// @Failure 409 {object} gin.H "Another client is already streaming this call"
// @Security ApiKeyAuth
// @Router /call/audio/{callId} [get]
func (g *callHandler) StreamAudio(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	callID := ctx.Param("callId")
	if callID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}

	// Everything that can fail with a readable HTTP status has to happen before
	// the upgrade — once the connection is hijacked, only close codes are left.
	outbound := make(chan []byte, audioSendQueue)
	bridge, err := g.callService.AttachAudio(callID, instance, func(pcm []float32) {
		if len(pcm) == 0 {
			return
		}
		select {
		case outbound <- media.PCMFloat32ToInt16LE(pcm):
		default:
			// Slow client: drop this frame rather than block the media path.
		}
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == voip_registry.ErrAudioAlreadyAttached {
			status = http.StatusConflict
		}
		ctx.JSON(status, gin.H{"error": err.Error()})
		return
	}

	conn, err := audioUpgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		bridge.Close()
		return
	}

	done := make(chan struct{})

	// Writer: peer audio out, plus pings so a dead peer is noticed.
	go func() {
		ticker := time.NewTicker(audioPingPeriod)
		defer func() {
			ticker.Stop()
			conn.Close()
		}()

		for {
			select {
			case frame := <-outbound:
				_ = conn.SetWriteDeadline(time.Now().Add(audioWriteWait))
				if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
					return
				}
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(audioWriteWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// Reader: microphone audio in. Runs on this goroutine so the handler only
	// returns once the client is gone, keeping the bridge alive meanwhile.
	conn.SetReadLimit(audioMaxReadSize)
	_ = conn.SetReadDeadline(time.Now().Add(audioPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(audioPongWait))
	})

	for {
		msgType, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(audioPongWait))

		if msgType != websocket.BinaryMessage || len(payload) == 0 {
			continue
		}
		bridge.Write(media.PCMInt16LEToFloat32(payload))
	}

	close(done)
	bridge.Close()
	conn.Close()
}
