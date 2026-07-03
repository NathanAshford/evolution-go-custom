// Package handler exposes the PUBLIC passkey-ceremony HTTP endpoints that the
// Evolution Passkey Helper browser extension calls from the web.whatsapp.com
// origin. These routes are intentionally unauthenticated (no apikey): access is
// gated only by an opaque, short-lived ceremony token minted per pairing.
//
// Contract (tools/passkey-helper/content.js):
//
//	GET  /passkey-ceremony/:token            -> { stage, publicKey?, code?, skipHandoffUX, error? }
//	POST /passkey-ceremony/:token/response   -> body = WebAuthn assertion
//	POST /passkey-ceremony/:token/confirm    -> finish pairing
package handler

import (
	"net/http"

	whatsmeow_service "github.com/evolution-foundation/evolution-go/pkg/whatsmeow/service"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow/types"
)

// PasskeyHandler wires the public ceremony endpoints to the whatsmeow service.
type PasskeyHandler struct {
	whatsmeowService whatsmeow_service.WhatsmeowService
}

// NewPasskeyHandler builds the handler.
func NewPasskeyHandler(svc whatsmeow_service.WhatsmeowService) *PasskeyHandler {
	return &PasskeyHandler{whatsmeowService: svc}
}

// GetCeremony returns the current ceremony state for a token. The extension
// polls this and drives its UI off the `stage` field.
// @Summary Get passkey ceremony state
// @Description Returns the current WebAuthn passkey-pairing ceremony state for a token. PUBLIC endpoint (no apikey) — access is gated by the opaque short-lived ceremony token. Polled by the Evolution Passkey Helper browser extension.
// @Tags Passkey
// @Produce json
// @Param token path string true "Ceremony token"
// @Success 200 {object} gin.H "Ceremony state ({stage, skipHandoffUX, publicKey?, code?, error?})"
// @Failure 400 {object} gin.H "token is required"
// @Failure 404 {object} gin.H "ceremony not found or expired"
// @Failure 503 {object} gin.H "passkey ceremony unavailable"
// @Router /passkey-ceremony/{token} [get]
func (h *PasskeyHandler) GetCeremony(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	store := h.whatsmeowService.PasskeyCeremonyStore()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey ceremony unavailable"})
		return
	}

	_, state, ok := store.Lookup(token)
	if !ok {
		// Unknown/expired token. The extension treats an empty/cleared state
		// after having started as "pairing concluded"; before that it keeps
		// waiting. Return 404 so a never-valid token is distinguishable.
		c.JSON(http.StatusNotFound, gin.H{"error": "ceremony not found or expired"})
		return
	}

	// gin.H keeps keys verbatim (publicKey, skipHandoffUX) — no struct-tag risk.
	resp := gin.H{
		"stage":         state.Stage,
		"skipHandoffUX": state.SkipHandoffUX,
	}
	if len(state.PublicKey) > 0 {
		resp["publicKey"] = state.PublicKey // json.RawMessage — emitted as-is
	}
	if state.Code != "" {
		resp["code"] = state.Code
	}
	if state.Error != "" {
		resp["error"] = state.Error
	}
	c.JSON(http.StatusOK, resp)
}

// SubmitResponse receives the WebAuthn assertion produced by the extension and
// forwards it to WhatsApp via SendPasskeyResponse.
// @Summary Submit passkey WebAuthn response
// @Description Receives the WebAuthn assertion produced by the browser extension and forwards it to WhatsApp. PUBLIC endpoint (no apikey) — gated by the ceremony token. Body is the WebAuthnResponse shape (id, rawId, type, response{clientDataJSON, authenticatorData, signature, userHandle?}), base64url-unpadded.
// @Tags Passkey
// @Accept json
// @Produce json
// @Param token path string true "Ceremony token"
// @Param response body types.WebAuthnResponse true "WebAuthn assertion"
// @Success 200 {object} gin.H "ok"
// @Failure 400 {object} gin.H "token is required / invalid body"
// @Failure 404 {object} gin.H "ceremony not found or expired"
// @Failure 500 {object} gin.H "Internal server error"
// @Failure 503 {object} gin.H "passkey ceremony unavailable"
// @Router /passkey-ceremony/{token}/response [post]
func (h *PasskeyHandler) SubmitResponse(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	store := h.whatsmeowService.PasskeyCeremonyStore()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey ceremony unavailable"})
		return
	}

	instanceID, ok := store.InstanceForToken(token)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "ceremony not found or expired"})
		return
	}

	// The extension posts exactly the WebAuthnResponse shape (id, rawId, type,
	// response{clientDataJSON, authenticatorData, signature, userHandle?}),
	// base64url-unpadded, which matches types.WebAuthnResponse's json tags.
	var resp types.WebAuthnResponse
	if err := c.ShouldBindJSON(&resp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.whatsmeowService.SubmitPasskeyResponse(instanceID, &resp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Confirm finishes the pairing after the user verified the confirmation code.
// @Summary Confirm passkey pairing
// @Description Finishes the passkey pairing after the user verified the confirmation code. PUBLIC endpoint (no apikey) — gated by the ceremony token.
// @Tags Passkey
// @Produce json
// @Param token path string true "Ceremony token"
// @Success 200 {object} gin.H "ok"
// @Failure 400 {object} gin.H "token is required"
// @Failure 404 {object} gin.H "ceremony not found or expired"
// @Failure 500 {object} gin.H "Internal server error"
// @Failure 503 {object} gin.H "passkey ceremony unavailable"
// @Router /passkey-ceremony/{token}/confirm [post]
func (h *PasskeyHandler) Confirm(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token is required"})
		return
	}

	store := h.whatsmeowService.PasskeyCeremonyStore()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "passkey ceremony unavailable"})
		return
	}

	instanceID, ok := store.InstanceForToken(token)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "ceremony not found or expired"})
		return
	}

	if err := h.whatsmeowService.ConfirmPasskey(instanceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RegisterRoutes wires the 3 PUBLIC ceremony endpoints directly on the engine,
// with NO auth group (mirrors core.LicenseRoutes). Call from main.go right
// after the license routes.
func RegisterRoutes(eng *gin.Engine, svc whatsmeow_service.WhatsmeowService) {
	h := NewPasskeyHandler(svc)
	grp := eng.Group("/passkey-ceremony")
	{
		grp.GET("/:token", h.GetCeremony)
		grp.POST("/:token/response", h.SubmitResponse)
		grp.POST("/:token/confirm", h.Confirm)
	}
}
