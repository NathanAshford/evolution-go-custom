package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	call_service "github.com/EvolutionAPI/evolution-go/pkg/call/service"
	"github.com/EvolutionAPI/evolution-go/pkg/config"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	instance_service "github.com/EvolutionAPI/evolution-go/pkg/instance/service"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	message_service "github.com/EvolutionAPI/evolution-go/pkg/message/service"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
	whatsmeow_service "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/service"
)

// Server exposes Evolution GO over the Model Context Protocol.
type Server struct {
	instanceService  instance_service.InstanceService
	sendService      send_service.SendService
	messageService   message_service.MessageService
	callService      call_service.CallService
	whatsmeowService whatsmeow_service.WhatsmeowService
	config           *config.Config
	loggerWrapper    *logger_wrapper.LoggerManager
}

func NewServer(
	instanceService instance_service.InstanceService,
	sendService send_service.SendService,
	messageService message_service.MessageService,
	callService call_service.CallService,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	cfg *config.Config,
	loggerWrapper *logger_wrapper.LoggerManager,
) *Server {
	return &Server{
		instanceService:  instanceService,
		sendService:      sendService,
		messageService:   messageService,
		callService:      callService,
		whatsmeowService: whatsmeowService,
		config:           cfg,
		loggerWrapper:    loggerWrapper,
	}
}

// session is the authenticated context of one MCP request.
//
// An instance token scopes every tool to that single instance; the admin key
// leaves the scope open, and tools then take an explicit instance argument.
type session struct {
	isAdmin  bool
	instance *instance_model.Instance
}

// scopedInstanceID returns the instance an instance-token session is pinned to.
func (s *session) scopedInstanceID() string {
	if s.instance != nil {
		return s.instance.Id
	}
	return ""
}

// RegisterRoutes mounts the MCP endpoint.
//
// Both /mcp and /mcp/:token are served: header auth suits clients that can set
// one (Claude Code, ChatGPT connectors), while the path form covers clients
// that only accept a bare URL.
func RegisterRoutes(eng *gin.Engine, srv *Server) {
	eng.POST("/mcp", srv.handleRPC)
	eng.POST("/mcp/:token", srv.handleRPC)

	// The Streamable HTTP spec lets a server decline the optional SSE stream.
	// Saying so explicitly stops clients from retrying the upgrade.
	eng.GET("/mcp", srv.handleUnsupportedStream)
	eng.GET("/mcp/:token", srv.handleUnsupportedStream)

	// DELETE terminates a session; this server is stateless, so acknowledge it.
	eng.DELETE("/mcp", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	eng.DELETE("/mcp/:token", func(c *gin.Context) { c.Status(http.StatusNoContent) })
}

func (s *Server) handleUnsupportedStream(c *gin.Context) {
	c.JSON(http.StatusMethodNotAllowed, gin.H{
		"error": "SSE streaming is not supported; POST JSON-RPC requests to this endpoint instead",
	})
}

// authenticate resolves the caller's key into a session. The key may arrive as
// an `apikey` header, a Bearer token, or the :token path segment.
func (s *Server) authenticate(c *gin.Context) (*session, bool) {
	key := c.GetHeader("apikey")

	if key == "" {
		if auth := c.GetHeader("Authorization"); auth != "" {
			key = strings.TrimSpace(strings.TrimPrefix(auth, "Bearer"))
			key = strings.TrimSpace(strings.TrimPrefix(key, "bearer"))
		}
	}
	if key == "" {
		key = c.Param("token")
	}
	if key == "" {
		key = c.Query("apikey")
	}
	if key == "" {
		return nil, false
	}

	if key == s.config.GlobalApiKey {
		return &session{isAdmin: true}, true
	}

	instance, err := s.instanceService.GetInstanceByToken(key)
	if err != nil || instance == nil {
		return nil, false
	}

	return &session{instance: instance}, true
}

func (s *Server) handleRPC(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, newError(nil, codeParseError, "failed to read request body"))
		return
	}

	// A batch is a JSON array; a single call is an object.
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "[") {
		s.handleBatch(c, body)
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, newError(nil, codeParseError, "invalid JSON: "+err.Error()))
		return
	}

	resp := s.dispatch(c, req)
	if resp == nil {
		// Notification — the spec requires an empty 202, not a JSON-RPC reply.
		c.Status(http.StatusAccepted)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) handleBatch(c *gin.Context, body []byte) {
	var reqs []rpcRequest
	if err := json.Unmarshal(body, &reqs); err != nil {
		c.JSON(http.StatusBadRequest, newError(nil, codeParseError, "invalid JSON batch: "+err.Error()))
		return
	}

	responses := make([]*rpcResponse, 0, len(reqs))
	for _, req := range reqs {
		if resp := s.dispatch(c, req); resp != nil {
			responses = append(responses, resp)
		}
	}

	// An all-notification batch gets no body.
	if len(responses) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	c.JSON(http.StatusOK, responses)
}

// dispatch routes one JSON-RPC call. It returns nil for notifications.
func (s *Server) dispatch(c *gin.Context, req rpcRequest) *rpcResponse {
	isNotification := req.isNotification()

	respond := func(resp *rpcResponse) *rpcResponse {
		if isNotification {
			return nil
		}
		return resp
	}

	switch req.Method {
	case "initialize":
		// initialize is answered before auth so clients get a clear tools/list
		// failure later rather than an opaque handshake rejection.
		return respond(newResult(req.ID, initializeResult{
			ProtocolVersion: protocolVersion,
			Capabilities:    serverCapabilities{Tools: &toolsCapability{}},
			ServerInfo:      serverInfo{Name: serverName, Version: serverVersion},
			Instructions: "Evolution GO WhatsApp connector.\n\n" +
				"Reading: search_messages (alias: search) finds archived messages by content, " +
				"date range, chat, sender, direction or type; fetch_message (alias: fetch) reads " +
				"one by id; get_chat_history and list_chats give conversation context.\n\n" +
				"Sending: send_text_message, send_media_message (image/video/audio/document), " +
				"send_link_message, send_location_message, send_contact_message, " +
				"send_sticker_message, send_poll_message, send_buttons_message, " +
				"send_list_message, send_carousel_message and send_status. " +
				"place_call rings a phone (no audio); end_call hangs up.\n\n" +
				"Interactive buttons have per-type fields — read the tool description before " +
				"building one. Carousel buttons differ from regular buttons: they carry the URL " +
				"or phone number in `id`.\n\n" +
				"Call list_instances first when you do not know which instance to use. " +
				"Confirm the recipient with the user before sending anything.",
		}))

	case "notifications/initialized", "notifications/cancelled", "initialized":
		return nil

	case "ping":
		return respond(newResult(req.ID, map[string]any{}))

	case "tools/list":
		if _, ok := s.authenticate(c); !ok {
			return respond(newError(req.ID, codeUnauthorized, "invalid or missing API key"))
		}
		return respond(newResult(req.ID, toolsListResult{Tools: s.toolDefinitions()}))

	case "tools/call":
		sess, ok := s.authenticate(c)
		if !ok {
			return respond(newError(req.ID, codeUnauthorized, "invalid or missing API key"))
		}

		var params callToolParams
		if len(req.Params) > 0 {
			if err := json.Unmarshal(req.Params, &params); err != nil {
				return respond(newError(req.ID, codeInvalidParams, "invalid params: "+err.Error()))
			}
		}
		if params.Name == "" {
			return respond(newError(req.ID, codeInvalidParams, "tool name is required"))
		}

		result := s.callTool(sess, params)
		return respond(newResult(req.ID, result))

	// Declared as unsupported so clients stop probing for them.
	case "resources/list":
		return respond(newResult(req.ID, map[string]any{"resources": []any{}}))
	case "prompts/list":
		return respond(newResult(req.ID, map[string]any{"prompts": []any{}}))
	}

	return respond(newError(req.ID, codeMethodNotFound, "unknown method: "+req.Method))
}
