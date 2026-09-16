package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
)

// obj/props are small helpers so the JSON Schemas below stay readable.
func obj(m map[string]any) map[string]any { return m }

func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func boolean(description string) map[string]any {
	return map[string]any{"type": "boolean", "description": description}
}

func integer(description string, def int) map[string]any {
	return map[string]any{"type": "integer", "description": description, "default": def}
}

func schema(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

const instanceArgDesc = "Instance id or name to act on. Optional when the API key " +
	"is an instance token (that instance is used). Required with an admin key — " +
	"call list_instances to discover the options."

const dateArgDesc = "Date/time in RFC3339 (2026-01-31T00:00:00Z) or YYYY-MM-DD. " +
	"YYYY-MM-DD is interpreted in the server's local timezone"

// toolDefinitions returns every tool: the read/search side defined here plus the
// message-sending side from tools_send.go.
func (s *Server) toolDefinitions() []tool {
	return append(s.readToolDefinitions(), s.sendToolDefinitions()...)
}

func (s *Server) readToolDefinitions() []tool {
	searchProps := map[string]any{
		"instance": str(instanceArgDesc),
		"query":    str("Text to look for inside message content (case-insensitive substring match). Omit to match any content."),
		"chat":     str("Filter by chat: a phone number (5511999999999), a full JID, or part of one."),
		"sender":   str("Filter by the sender's number or JID."),
		"from_me": boolean("true returns only messages you sent, false only messages received. " +
			"Omit for both directions."),
		"is_group":     boolean("true restricts to group chats, false to direct chats."),
		"message_type": str("Filter by type: text, image, video, audio, document, sticker, contact, location, reaction, poll, buttons, list, interactive."),
		"start_date":   str("Only messages at or after this moment. " + dateArgDesc),
		"end_date":     str("Only messages at or before this moment. " + dateArgDesc),
		"limit":        integer("Maximum messages to return (1-500).", 50),
		"offset":       integer("Number of messages to skip, for paging through results.", 0),
	}

	return []tool{
		{
			Name: "list_instances",
			Description: "List the WhatsApp instances available to this API key, with their " +
				"connection status. Call this first when you need an instance id.",
			InputSchema: schema(map[string]any{}),
		},
		{
			Name: "search_messages",
			Description: "Search archived WhatsApp messages by content, date range, chat, " +
				"sender, direction or type. Returns the newest matches first. " +
				"Only messages exchanged after the archive was enabled are searchable.",
			InputSchema: schema(searchProps),
		},
		{
			Name: "get_chat_history",
			Description: "Return the most recent messages of one chat, newest first. " +
				"Use this to read a conversation once search_messages has identified the chat.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"chat":     str("Chat to read: phone number with country code, or a group JID."),
				"limit":    integer("Maximum messages to return (1-500).", 50),
				"before":   str("Only messages at or before this moment. " + dateArgDesc),
			}), "chat"),
		},
		{
			Name: "find_contact",
			Description: "Find a WhatsApp contact or group by name and return the number to " +
				"send to. Use this whenever the user names a person instead of giving a " +
				"number — \"send hi to Cibele Falcão\" means calling find_contact with " +
				"query \"Cibele Falcão\" first, then passing the returned number to a send " +
				"tool. Matching ignores case and accents, so \"falcao\" finds \"Falcão\". " +
				"If the result says ambiguous, ask the user which contact they meant " +
				"instead of guessing.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"query": str("Name, or part of a name, to look for — \"Cibele\", \"Cibele Falcão\", " +
					"\"Falcao\". A number searches by number instead. Omit to list every contact."),
				"include_groups": boolean("Include groups this instance belongs to. Defaults to true."),
				"limit":          integer("Maximum matches to return (1-200).", 20),
			})),
		},
		{
			Name: "list_chats",
			Description: "List the most recently active chats for an instance, each with its " +
				"last message and the contact's saved name. Useful to get an overview " +
				"before searching. To look someone up by name, prefer find_contact.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"limit":    integer("Maximum chats to return (1-500).", 30),
			})),
		},
		{
			Name: "fetch_message",
			Description: "Fetch one archived message by its WhatsApp message id.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"id":       str("The WhatsApp message id, as returned by search_messages."),
			}), "id"),
		},
		// ChatGPT's deep research mode requires tools named exactly "search" and
		// "fetch". They are thin aliases over the descriptive tools above.
		{
			Name: "search",
			Description: "Search archived WhatsApp messages. Returns matching messages with " +
				"their ids, which can be passed to fetch.",
			InputSchema: schema(obj(map[string]any{
				"query":    str("Text to look for inside message content."),
				"instance": str(instanceArgDesc),
				"limit":    integer("Maximum messages to return (1-500).", 50),
			}), "query"),
		},
		{
			Name:        "fetch",
			Description: "Fetch the full content of one archived WhatsApp message by id.",
			InputSchema: schema(obj(map[string]any{
				"id":       str("The WhatsApp message id returned by search."),
				"instance": str(instanceArgDesc),
			}), "id"),
		},
	}
}

// callTool executes a tool and always returns a result — tool failures are
// reported as isError content so the model can react, not as protocol errors.
func (s *Server) callTool(sess *session, params callToolParams) *callToolResult {
	args := map[string]any{}
	if len(params.Arguments) > 0 {
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return errorResult("invalid arguments: " + err.Error())
		}
	}

	switch params.Name {
	case "list_instances":
		return s.toolListInstances(sess)
	case "send_text_message":
		return s.toolSendText(sess, args)
	case "send_media_message":
		return s.toolSendMedia(sess, args)
	case "send_link_message":
		return s.toolSendLink(sess, args)
	case "send_location_message":
		return s.toolSendLocation(sess, args)
	case "send_contact_message":
		return s.toolSendContact(sess, args)
	case "send_sticker_message":
		return s.toolSendSticker(sess, args)
	case "send_poll_message":
		return s.toolSendPoll(sess, args)
	case "send_buttons_message":
		return s.toolSendButtons(sess, args)
	case "send_list_message":
		return s.toolSendList(sess, args)
	case "send_carousel_message":
		return s.toolSendCarousel(sess, args)
	case "send_status":
		return s.toolSendStatus(sess, args)
	case "place_call":
		return s.toolPlaceCall(sess, args)
	case "end_call":
		return s.toolEndCall(sess, args)
	case "send_message_to_contact":
		return s.toolSendToContact(sess, args)
	case "delete_message":
		return s.toolDeleteMessage(sess, args)
	case "search_messages":
		return s.toolSearchMessages(sess, args)
	case "get_chat_history":
		return s.toolChatHistory(sess, args)
	case "find_contact":
		return s.toolFindContact(sess, args)
	case "list_chats":
		return s.toolListChats(sess, args)
	case "fetch_message", "fetch":
		return s.toolFetchMessage(sess, args)
	case "search":
		return s.toolSearchMessages(sess, args)
	}

	return errorResult("unknown tool: " + params.Name)
}

// --- argument helpers ---

func argString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func argInt(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64: // JSON numbers decode as float64
		return int(n)
	case int:
		return n
	case string:
		var parsed int
		if _, err := fmt.Sscanf(n, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return def
}

func argBoolPtr(args map[string]any, key string) *bool {
	v, ok := args[key]
	if !ok || v == nil {
		return nil
	}
	switch b := v.(type) {
	case bool:
		return &b
	case string:
		switch strings.ToLower(b) {
		case "true":
			t := true
			return &t
		case "false":
			f := false
			return &f
		}
	}
	return nil
}

// parseDate accepts RFC3339 or a bare YYYY-MM-DD. endOfDay pushes a bare date
// to 23:59:59 so "end_date: 2026-01-31" includes that whole day.
func parseDate(value string, endOfDay bool) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t, nil
	}

	if t, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Second)
		}
		return &t, nil
	}

	return nil, fmt.Errorf("invalid date %q: use RFC3339 (2026-01-31T00:00:00Z) or YYYY-MM-DD", value)
}

// resolveInstance maps the "instance" argument to a concrete instance,
// honouring the session's scope.
func (s *Server) resolveInstance(sess *session, args map[string]any) (*instance_model.Instance, error) {
	// An instance token is pinned to its own instance and may not address others.
	if sess.instance != nil {
		return sess.instance, nil
	}

	ref := argString(args, "instance")
	if ref == "" {
		return nil, fmt.Errorf("the 'instance' argument is required with an admin API key; call list_instances to see the options")
	}

	all, err := s.instanceService.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	for _, inst := range all {
		if inst.Id == ref || strings.EqualFold(inst.Name, ref) {
			return inst, nil
		}
	}

	return nil, fmt.Errorf("instance %q not found", ref)
}

// --- tools ---

type instanceSummary struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Number    string `json:"number,omitempty"`
}

func (s *Server) toolListInstances(sess *session) *callToolResult {
	if sess.instance != nil {
		return jsonResult([]instanceSummary{{
			Id:        sess.instance.Id,
			Name:      sess.instance.Name,
			Connected: sess.instance.Connected,
			Number:    jidNumber(sess.instance.Jid),
		}})
	}

	all, err := s.instanceService.GetAll()
	if err != nil {
		return errorResult("failed to list instances: " + err.Error())
	}

	summaries := make([]instanceSummary, 0, len(all))
	for _, inst := range all {
		summaries = append(summaries, instanceSummary{
			Id:        inst.Id,
			Name:      inst.Name,
			Connected: inst.Connected,
			Number:    jidNumber(inst.Jid),
		})
	}

	return jsonResult(summaries)
}

func jidNumber(jid string) string {
	if jid == "" {
		return ""
	}
	if idx := strings.IndexAny(jid, ":@"); idx > 0 {
		return jid[:idx]
	}
	return jid
}

func (s *Server) toolSendText(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	number := argString(args, "number")
	text := argString(args, "text")

	if number == "" {
		return errorResult("the 'number' argument is required")
	}
	if text == "" {
		return errorResult("the 'text' argument is required")
	}
	if !instance.Connected {
		return errorResult(fmt.Sprintf(
			"instance %q is not connected to WhatsApp; connect it before sending", instance.Name,
		))
	}

	data := &send_service.TextStruct{
		Number: number,
		Text:   text,
		Delay:  int32(argInt(args, "delay_ms", 0)),
	}
	if replyTo := argString(args, "reply_to_message_id"); replyTo != "" {
		data.Quoted = send_service.QuotedStruct{MessageID: replyTo}
	}

	sent, err := s.sendService.SendText(data, instance)
	if err != nil {
		return errorResult("failed to send message: " + err.Error())
	}

	return jsonResult(map[string]any{
		"status":    "sent",
		"messageId": sent.Info.ID,
		"chat":      sent.Info.Chat.String(),
		"timestamp": sent.Info.Timestamp.Format(time.RFC3339),
		"instance":  instance.Name,
	})
}

// searchHit is the shape returned to the model: compact, and with the id it
// needs to fetch the full message.
type searchHit struct {
	Id          string `json:"id"`
	Chat        string `json:"chat"`
	Sender      string `json:"sender,omitempty"`
	PushName    string `json:"pushName,omitempty"`
	FromMe      bool   `json:"fromMe"`
	IsGroup     bool   `json:"isGroup"`
	MessageType string `json:"type"`
	Content     string `json:"content"`
	Timestamp   string `json:"timestamp"`
}

func toHits(messages []message_model.ArchivedMessage) []searchHit {
	hits := make([]searchHit, 0, len(messages))
	for _, m := range messages {
		hits = append(hits, searchHit{
			Id:          m.MessageID,
			Chat:        m.ChatJID,
			Sender:      m.SenderJID,
			PushName:    m.PushName,
			FromMe:      m.FromMe,
			IsGroup:     m.IsGroup,
			MessageType: m.MessageType,
			Content:     m.Content,
			Timestamp:   m.Timestamp.Format(time.RFC3339),
		})
	}
	return hits
}

func (s *Server) toolSearchMessages(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	startDate, err := parseDate(argString(args, "start_date"), false)
	if err != nil {
		return errorResult(err.Error())
	}
	endDate, err := parseDate(argString(args, "end_date"), true)
	if err != nil {
		return errorResult(err.Error())
	}

	filter := message_model.MessageSearchFilter{
		InstanceID:  instance.Id,
		Query:       argString(args, "query"),
		ChatJID:     normalizeChatRef(argString(args, "chat")),
		SenderJID:   normalizeChatRef(argString(args, "sender")),
		MessageType: argString(args, "message_type"),
		FromMe:      argBoolPtr(args, "from_me"),
		IsGroup:     argBoolPtr(args, "is_group"),
		StartDate:   startDate,
		EndDate:     endDate,
		Limit:       argInt(args, "limit", 50),
		Offset:      argInt(args, "offset", 0),
	}

	messages, total, err := s.whatsmeowService.SearchArchivedMessages(filter)
	if err != nil {
		return errorResult("search failed: " + err.Error())
	}

	return jsonResult(map[string]any{
		"instance":     instance.Name,
		"totalMatches": total,
		"returned":     len(messages),
		"results":      toHits(messages),
	})
}

func (s *Server) toolChatHistory(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	chat := argString(args, "chat")
	if chat == "" {
		return errorResult("the 'chat' argument is required")
	}

	before, err := parseDate(argString(args, "before"), true)
	if err != nil {
		return errorResult(err.Error())
	}

	messages, total, err := s.whatsmeowService.SearchArchivedMessages(message_model.MessageSearchFilter{
		InstanceID: instance.Id,
		ChatJID:    normalizeChatRef(chat),
		EndDate:    before,
		Limit:      argInt(args, "limit", 50),
	})
	if err != nil {
		return errorResult("failed to read chat history: " + err.Error())
	}

	return jsonResult(map[string]any{
		"instance":      instance.Name,
		"chat":          chat,
		"totalMessages": total,
		"returned":      len(messages),
		"messages":      toHits(messages),
	})
}

func (s *Server) toolListChats(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	chats, err := s.whatsmeowService.ListArchivedChats(instance.Id, argInt(args, "limit", 30))
	if err != nil {
		return errorResult("failed to list chats: " + err.Error())
	}

	type chatOut struct {
		Chat        string `json:"chat"`
		Name        string `json:"name,omitempty"`
		SavedName   string `json:"savedName,omitempty"`
		PushName    string `json:"pushName,omitempty"`
		IsGroup     bool   `json:"isGroup"`
		LastMessage string `json:"lastMessage"`
		LastFromMe  bool   `json:"lastFromMe"`
		Timestamp   string `json:"timestamp"`
		Total       int64  `json:"totalMessages"`
	}

	// The archive only ever sees push names — the name the other party chose for
	// themselves. The address book is what the operator actually recognises, so
	// prefer it and keep both. A nil index just means we fall back to push names.
	names := s.contactNameIndex(instance.Id)

	out := make([]chatOut, 0, len(chats))
	for _, c := range chats {
		saved := names[jidNumber(c.ChatJID)]

		display := saved
		if display == "" {
			display = c.PushName
		}

		out = append(out, chatOut{
			Chat:        c.ChatJID,
			Name:        display,
			SavedName:   saved,
			PushName:    c.PushName,
			IsGroup:     c.IsGroup,
			LastMessage: c.LastMessage,
			LastFromMe:  c.LastFromMe,
			Timestamp:   c.Timestamp.Format(time.RFC3339),
			Total:       c.Total,
		})
	}

	return jsonResult(map[string]any{
		"instance": instance.Name,
		"chats":    out,
	})
}

func (s *Server) toolFetchMessage(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	id := argString(args, "id")
	if id == "" {
		return errorResult("the 'id' argument is required")
	}

	msg, err := s.whatsmeowService.GetArchivedMessage(instance.Id, id)
	if err != nil {
		return errorResult("failed to fetch message: " + err.Error())
	}
	if msg == nil {
		return errorResult(fmt.Sprintf("message %q not found in the archive", id))
	}

	hits := toHits([]message_model.ArchivedMessage{*msg})
	return jsonResult(hits[0])
}

// normalizeChatRef turns a user-supplied chat reference into something the
// partial JID match can use: a bare number keeps only its digits, while an
// explicit JID is passed through untouched.
func normalizeChatRef(ref string) string {
	if ref == "" || strings.Contains(ref, "@") {
		return ref
	}

	var digits strings.Builder
	for _, r := range ref {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}

	if digits.Len() == 0 {
		return ref
	}
	return digits.String()
}
