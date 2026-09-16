package call_handler

import (
	"net/http"

	call_service "github.com/EvolutionAPI/evolution-go/pkg/call/service"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	"github.com/gin-gonic/gin"
)

// instanceFromContext pulls the authenticated instance placed by the auth
// middleware.
func instanceFromContext(ctx *gin.Context) (*instance_model.Instance, bool) {
	value, exists := ctx.Get("instance")
	if !exists {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return nil, false
	}

	instance, ok := value.(*instance_model.Instance)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "instance not found"})
		return nil, false
	}

	return instance, true
}

// Offer call
// @Summary Place a WhatsApp call
// @Description Places an outgoing WhatsApp voice call. The peer's device rings and the
// @Description returned callId can be used to hang up. Audio is silence keep-alive —
// @Description the HTTP API rings and holds the call but does not stream microphone audio.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.OfferCallStruct true "Call data"
// @Success 200 {object} gin.H "Call placed"
// @Failure 400 {object} gin.H "Invalid number"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/offer [post]
func (g *callHandler) OfferCall(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	var data *call_service.OfferCallStruct
	if err := ctx.ShouldBindBodyWithJSON(&data); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	snapshot, err := g.callService.OfferCall(data, instance)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": snapshot})
}

// Accept call
// @Summary Accept an incoming call
// @Description Answers a call that is currently ringing on this instance.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.CallActionStruct true "Call id"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "callId is required"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/accept [post]
func (g *callHandler) AcceptCall(ctx *gin.Context) {
	g.callAction(ctx, func(data *call_service.CallActionStruct, instance *instance_model.Instance) error {
		return g.callService.AcceptCall(data, instance)
	})
}

// Terminate call
// @Summary Hang up a call
// @Description Ends a call that is ringing or already active.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.CallActionStruct true "Call id and optional reason"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "callId is required"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/terminate [post]
func (g *callHandler) TerminateCall(ctx *gin.Context) {
	g.callAction(ctx, func(data *call_service.CallActionStruct, instance *instance_model.Instance) error {
		return g.callService.TerminateCall(data, instance)
	})
}

// Hangup call
// @Summary Reject a ringing call by id
// @Description Declines an inbound call tracked by the VoIP stack. Use /call/reject to
// @Description decline using the raw callCreator/callId pair from the webhook instead.
// @Tags Call
// @Accept json
// @Produce json
// @Param message body call_service.CallActionStruct true "Call id and optional reason"
// @Success 200 {object} gin.H "success"
// @Failure 400 {object} gin.H "callId is required"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/hangup [post]
func (g *callHandler) HangupCall(ctx *gin.Context) {
	g.callAction(ctx, func(data *call_service.CallActionStruct, instance *instance_model.Instance) error {
		return g.callService.RejectCallByID(data, instance)
	})
}

// callAction factors out the shared bind/execute/respond flow of the by-id
// endpoints.
func (g *callHandler) callAction(
	ctx *gin.Context,
	action func(*call_service.CallActionStruct, *instance_model.Instance) error,
) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	var data *call_service.CallActionStruct
	if err := ctx.ShouldBindBodyWithJSON(&data); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := action(data, instance); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success"})
}

// List calls
// @Summary List active calls
// @Description Returns the calls currently ringing or active on this instance.
// @Tags Call
// @Produce json
// @Success 200 {object} gin.H "Active calls"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/list [get]
func (g *callHandler) ListCalls(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": g.callService.ListCalls(instance)})
}

// Call history
// @Summary List recent finished calls
// @Description Returns the most recent calls that already ended (in-memory, capped at 50).
// @Tags Call
// @Produce json
// @Success 200 {object} gin.H "Call history"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/history [get]
func (g *callHandler) CallHistory(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": g.callService.CallHistory(instance)})
}

// Get call
// @Summary Get one call by id
// @Description Returns a single call, whether it is live or already finished.
// @Tags Call
// @Produce json
// @Param callId path string true "Call ID"
// @Success 200 {object} gin.H "Call"
// @Failure 404 {object} gin.H "Call not found"
// @Failure 500 {object} gin.H "Internal server error"
// @Security ApiKeyAuth
// @Router /call/status/{callId} [get]
func (g *callHandler) GetCall(ctx *gin.Context) {
	instance, ok := instanceFromContext(ctx)
	if !ok {
		return
	}

	callID := ctx.Param("callId")
	if callID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "callId is required"})
		return
	}

	snapshot, err := g.callService.GetCall(callID, instance)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "success", "data": snapshot})
}
