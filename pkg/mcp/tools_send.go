package mcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	call_service "github.com/EvolutionAPI/evolution-go/pkg/call/service"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	message_service "github.com/EvolutionAPI/evolution-go/pkg/message/service"
	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
)

// This file holds the MCP tools that send messages. The search/registry tools
// live in tools.go.
//
// Every schema here mirrors the real /send/* request body field-for-field, so
// what the model produces is exactly what the HTTP API accepts.

func numberProp() map[string]any {
	return str("Recipient: phone number with country code (digits only, e.g. 5511999999999), " +
		"or a group JID ending in @g.us. A person's name is not accepted here — " +
		"resolve it with find_contact first and pass the number it returns.")
}

func delayProp() map[string]any {
	return integer("Optional typing delay in milliseconds before sending.", 0)
}

func quotedProp() map[string]any {
	return str("Optional id of a message to quote/reply to (use a messageId from search_messages).")
}

// sendToolDefinitions returns every message-sending tool.
func (s *Server) sendToolDefinitions() []tool {
	return []tool{
		{
			Name: "send_text_message",
			Description: "Send a plain WhatsApp text message. Verify the recipient with the " +
				"user before sending.",
			InputSchema: schema(obj(map[string]any{
				"instance":            str(instanceArgDesc),
				"number":              numberProp(),
				"text":                str("Message body to send."),
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "number", "text"),
		},
		{
			Name: "send_media_message",
			Description: "Send an image, video, audio or document, either from a public URL " +
				"(the server downloads it) or from base64 bytes you already have. " +
				"Give exactly one of 'url' or 'base64'. Prefer 'url' whenever the file " +
				"is reachable online — base64 is far more expensive to pass around.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"number":   numberProp(),
				"url":      str("Public http(s) URL of the file to send. Omit when using 'base64'."),
				"base64": str("The file itself, base64-encoded, for when it is not reachable by URL. " +
					"A data URI (data:image/png;base64,...) is accepted and its prefix is stripped. " +
					"Omit when using 'url'."),
				"type": map[string]any{
					"type":        "string",
					"description": "Media kind. Determines how WhatsApp renders it.",
					"enum":        []string{"image", "video", "audio", "document"},
				},
				"caption":             str("Caption shown with the media. Not used for audio."),
				"filename":            str("File name shown to the recipient (documents). Recommended with 'base64', which carries no name of its own."),
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "number", "type"),
		},
		{
			Name:        "send_link_message",
			Description: "Send a text message with a rich link preview (title, description and thumbnail).",
			InputSchema: schema(obj(map[string]any{
				"instance":            str(instanceArgDesc),
				"number":              numberProp(),
				"text":                str("Message body. Should normally contain the URL as well."),
				"url":                 str("URL to preview."),
				"title":               str("Preview title."),
				"description":         str("Preview description."),
				"image_url":           str("Preview thumbnail URL."),
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "number", "text", "url"),
		},
		{
			Name:        "send_location_message",
			Description: "Send a geographic location pin.",
			InputSchema: schema(obj(map[string]any{
				"instance":  str(instanceArgDesc),
				"number":    numberProp(),
				"latitude":  map[string]any{"type": "number", "description": "Latitude in decimal degrees."},
				"longitude": map[string]any{"type": "number", "description": "Longitude in decimal degrees."},
				"name":      str("Place name shown on the pin."),
				"address":   str("Street address shown under the name."),
				"delay_ms":  delayProp(),
			}), "number", "latitude", "longitude"),
		},
		{
			Name:        "send_contact_message",
			Description: "Send a contact card (vCard).",
			InputSchema: schema(obj(map[string]any{
				"instance":     str(instanceArgDesc),
				"number":       numberProp(),
				"full_name":    str("Contact's display name."),
				"phone":        str("Contact's phone number with country code."),
				"organization": str("Contact's company (optional)."),
				"delay_ms":     delayProp(),
			}), "number", "full_name", "phone"),
		},
		{
			Name:        "send_sticker_message",
			Description: "Send a sticker from a public image URL (WebP renders best).",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"number":   numberProp(),
				"url":      str("Public URL of the sticker image."),
				"delay_ms": delayProp(),
			}), "number", "url"),
		},
		{
			Name:        "send_poll_message",
			Description: "Send a poll with 2 or more options.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"number":   numberProp(),
				"question": str("The poll question."),
				"options": map[string]any{
					"type":        "array",
					"description": "Poll options (at least 2).",
					"items":       map[string]any{"type": "string"},
				},
				"max_answers": integer("How many options a voter may pick. 1 = single choice.", 1),
				"delay_ms":    delayProp(),
			}), "number", "question", "options"),
		},
		{
			Name: "send_buttons_message",
			Description: "Send an interactive message with tappable buttons.\n\n" +
				"Button types and the fields each one uses:\n" +
				"  reply → display_text + id (id is the callback payload)\n" +
				"  url   → display_text + url\n" +
				"  call  → display_text + phone_number (E.164, e.g. +5511999999999)\n" +
				"  copy  → display_text + copy_code (text placed on the clipboard)\n" +
				"  pix   → currency + name + key_type + key (no display_text)\n\n" +
				"Rules enforced by the server: at most 3 reply buttons; reply cannot be " +
				"mixed with other types; a pix button must be the only button.\n" +
				"Rendering caveat (not enforced): mixing reply with url/call/copy makes the " +
				"message invisible on WhatsApp Web — send only-reply, or only CTAs.",
			InputSchema: schema(obj(map[string]any{
				"instance":    str(instanceArgDesc),
				"number":      numberProp(),
				"title":       str("Header title."),
				"description": str("Body text."),
				"footer":      str("Footer text."),
				"buttons": map[string]any{
					"type":        "array",
					"description": "Buttons to render. See the tool description for per-type fields.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"type": map[string]any{
								"type":        "string",
								"description": "Button kind.",
								"enum":        []string{"reply", "url", "call", "copy", "pix"},
							},
							"display_text": str("Label rendered inside the button (all types except pix)."),
							"id":           str("Callback payload, for type=reply."),
							"url":          str("Target URL, for type=url."),
							"phone_number": str("Destination number in E.164, for type=call."),
							"copy_code":    str("Text copied to the clipboard, for type=copy."),
							"currency":     str("ISO currency code for type=pix (e.g. BRL)."),
							"name":         str("Merchant name shown on the Pix sheet, for type=pix."),
							"key_type": map[string]any{
								"type":        "string",
								"description": "Pix key kind, for type=pix.",
								"enum":        []string{"phone", "email", "cpf", "cnpj", "random"},
							},
							"key": str("Pix key value matching key_type, for type=pix."),
						},
						"required": []string{"type"},
					},
				},
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "number", "buttons"),
		},
		{
			Name: "send_list_message",
			Description: "Send a single-select menu. The recipient taps a button that opens a " +
				"list of sections, each holding selectable rows.",
			InputSchema: schema(obj(map[string]any{
				"instance":    str(instanceArgDesc),
				"number":      numberProp(),
				"title":       str("Header title."),
				"description": str("Body text."),
				"button_text": str("Label of the button that opens the list. Defaults to 'Ver Menu'."),
				"footer":      str("Footer text."),
				"sections": map[string]any{
					"type":        "array",
					"description": "Sections of the menu. At least one section with one row.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title": str("Section heading."),
							"rows": map[string]any{
								"type":        "array",
								"description": "Selectable rows in this section.",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"title":       str("Row label."),
										"description": str("Optional second line."),
										"row_id":      str("Callback payload returned when tapped."),
									},
									"required": []string{"title"},
								},
							},
						},
						"required": []string{"rows"},
					},
				},
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "number", "sections"),
		},
		{
			Name: "send_carousel_message",
			Description: "Send a swipeable carousel of cards, each with media, text and buttons.\n\n" +
				"IMPORTANT — carousel buttons differ from send_buttons_message: there are no " +
				"url/phone_number fields. For URL and CALL buttons the value goes in `id`:\n" +
				"  REPLY (default) → display_text + id (callback payload)\n" +
				"  URL   → display_text + id (put the URL in id)\n" +
				"  CALL  → display_text + id (put the phone number in id)\n" +
				"  COPY  → display_text + copy_code\n" +
				"Pix buttons are not supported in carousels — use send_buttons_message.\n" +
				"Avoid mixing REPLY with CTA buttons in the same card: mixed sets do not " +
				"render on WhatsApp Web.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"number":   numberProp(),
				"body":     str("Optional text shown above the cards."),
				"footer":   str("Optional text shown below the cards."),
				"cards": map[string]any{
					"type":        "array",
					"description": "Cards of the carousel. At least one.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":     str("Card title, above the media."),
							"subtitle":  str("Card subtitle."),
							"image_url": str("Public image URL for the card media."),
							"video_url": str("Public video URL, used only when image_url is empty."),
							"text":      str("Card body text (required)."),
							"footer":    str("Card footer."),
							"buttons": map[string]any{
								"type":        "array",
								"description": "Card buttons. See the tool description for per-type fields.",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"type": map[string]any{
											"type":        "string",
											"description": "Button kind.",
											"enum":        []string{"REPLY", "URL", "CALL", "COPY"},
										},
										"display_text": str("Label rendered inside the button."),
										"id":           str("REPLY payload, or the URL (type=URL), or the phone number (type=CALL)."),
										"copy_code":    str("Text copied to the clipboard, for type=COPY."),
									},
									"required": []string{"type", "display_text"},
								},
							},
						},
						"required": []string{"text"},
					},
				},
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "number", "cards"),
		},
		{
			Name: "send_status",
			Description: "Post to the instance's WhatsApp status (stories). Provide either text, " +
				"or a media url plus its type.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"text":     str("Text status content."),
				"url":      str("Public media URL for a media status."),
				"type": map[string]any{
					"type":        "string",
					"description": "Media kind, required when url is set.",
					"enum":        []string{"image", "video", "audio"},
				},
				"caption": str("Caption for a media status."),
			})),
		},
		{
			Name: "place_call",
			Description: "Place a WhatsApp voice call — the recipient's phone rings. The call " +
				"carries no audio (no microphone source), so use it to alert someone, then " +
				"hang up with end_call. Ask the user before calling anyone.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"number":   numberProp(),
			}), "number"),
		},
		{
			Name:        "end_call",
			Description: "Hang up a call placed with place_call.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"call_id":  str("The callId returned by place_call."),
			}), "call_id"),
		},
		{
			Name: "send_message_to_contact",
			Description: "Send a text message to someone identified by name instead of by number — " +
				"\"send hi to Cibele Falcão\". The contact is resolved on the server and the " +
				"phone number is never returned, which also keeps the number out of the " +
				"conversation. Prefer this over find_contact + send_text_message when the " +
				"user names a person. If the name matches more than one contact nothing is " +
				"sent: the error lists the options with masked numbers so you can ask which " +
				"one was meant.",
			InputSchema: schema(obj(map[string]any{
				"instance": str(instanceArgDesc),
				"name":     str("The contact or group name, as the user said it. Accents and case are ignored."),
				"text":     str("Message body to send."),
				"number_hint": str("Only needed to break a tie between contacts with the same name: " +
					"the last digits of the intended number."),
				"reply_to_message_id": quotedProp(),
				"delay_ms":            delayProp(),
			}), "name", "text"),
		},
		{
			Name: "delete_message",
			Description: "Delete a message for everyone (WhatsApp \"revoke\"), so it disappears " +
				"for the recipient too, not just locally. Only messages this instance " +
				"sent can be revoked, and WhatsApp refuses messages that are already too " +
				"old. This cannot be undone — confirm with the user before calling it.",
			InputSchema: schema(obj(map[string]any{
				"instance":   str(instanceArgDesc),
				"number":     numberProp(),
				"message_id": str("Id of the message to delete, as returned by send tools or search_messages."),
			}), "number", "message_id"),
		},
	}
}

// --- argument decoding helpers ---

func argFloat(args map[string]any, key string) (float64, bool) {
	v, ok := args[key]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		var parsed float64
		if _, err := fmt.Sscanf(n, "%g", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func argStringSlice(args map[string]any, key string) []string {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func argObjects(args map[string]any, key string) []map[string]any {
	raw, ok := args[key].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// sendPrelude resolves the target instance.
//
// It deliberately does NOT check connectivity: each tool validates its arguments
// first and only then calls requireConnected, so a malformed payload reports the
// actual mistake (which the model can fix) instead of an unrelated
// "not connected" error.
func (s *Server) sendPrelude(sess *session, args map[string]any) (*instance_model.Instance, *callToolResult) {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return nil, errorResult(err.Error())
	}
	return instance, nil
}

// requireConnected gates the actual send.
func requireConnected(instance *instance_model.Instance) *callToolResult {
	if !instance.Connected {
		return errorResult(fmt.Sprintf(
			"instance %q is not connected to WhatsApp; connect it before sending", instance.Name,
		))
	}
	return nil
}

// sendResult renders the common success payload.
func sendResult(instance *instance_model.Instance, sent *send_service.MessageSendStruct, kind string) *callToolResult {
	return jsonResult(map[string]any{
		"status":    "sent",
		"type":      kind,
		"messageId": sent.Info.ID,
		"chat":      sent.Info.Chat.String(),
		"timestamp": sent.Info.Timestamp.Format(time.RFC3339),
		"instance":  instance.Name,
	})
}

func quotedFrom(args map[string]any) send_service.QuotedStruct {
	if id := argString(args, "reply_to_message_id"); id != "" {
		return send_service.QuotedStruct{MessageID: id}
	}
	return send_service.QuotedStruct{}
}

// --- tool implementations ---

func (s *Server) toolSendMedia(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	number := argString(args, "number")
	url := argString(args, "url")
	inline := argString(args, "base64")
	mediaType := strings.ToLower(argString(args, "type"))

	if number == "" {
		return errorResult("the 'number' argument is required")
	}
	switch mediaType {
	case "image", "video", "audio", "document":
	default:
		return errorResult("'type' must be one of: image, video, audio, document")
	}

	// Parity with POST /send/media, which carries base64 in the same "url" field
	// whenever it is not an http(s) address. Callers used to the REST shape keep
	// working, and 'base64' is simply the explicit spelling of the same thing.
	if inline == "" && url != "" && !isHTTPURL(url) {
		inline, url = url, ""
	}

	switch {
	case url == "" && inline == "":
		return errorResult("provide either 'url' (a public http(s) link) or 'base64' (the file bytes)")
	case url != "" && inline != "":
		return errorResult("provide only one of 'url' or 'base64', not both")
	}

	// Decode before the connectivity check so a malformed payload reports the
	// real mistake rather than blaming the connection.
	var fileBytes []byte
	if inline != "" {
		decoded, err := decodeBase64Media(inline)
		if err != nil {
			return errorResult(err.Error())
		}
		fileBytes = decoded
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	data := &send_service.MediaStruct{
		Number:   number,
		Url:      url,
		Type:     mediaType,
		Caption:  argString(args, "caption"),
		Filename: argString(args, "filename"),
		Delay:    int32(argInt(args, "delay_ms", 0)),
		Quoted:   quotedFrom(args),
	}

	var (
		sent *send_service.MessageSendStruct
		err  error
	)
	if fileBytes != nil {
		sent, err = s.sendService.SendMediaFile(data, fileBytes, instance)
	} else {
		sent, err = s.sendService.SendMediaUrl(data, instance)
	}
	if err != nil {
		return errorResult("failed to send media: " + err.Error())
	}

	return sendResult(instance, sent, mediaType)
}

func isHTTPURL(value string) bool {
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// decodeBase64Media accepts what callers actually send: a bare base64 string, a
// data: URI, either alphabet (standard or URL-safe) and either padding style,
// with the line breaks that wrapped payloads carry stripped out.
func decodeBase64Media(value string) ([]byte, error) {
	payload := value

	if strings.HasPrefix(strings.ToLower(payload), "data:") {
		comma := strings.Index(payload, ",")
		if comma < 0 {
			return nil, fmt.Errorf("malformed data URI: expected a comma before the base64 payload")
		}
		payload = payload[comma+1:]
	}

	payload = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		}
		return r
	}, payload)

	if payload == "" {
		return nil, fmt.Errorf("the 'base64' payload is empty")
	}

	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if decoded, err := encoding.DecodeString(payload); err == nil {
			if len(decoded) == 0 {
				return nil, fmt.Errorf("the 'base64' payload decoded to zero bytes")
			}
			return decoded, nil
		}
	}

	return nil, fmt.Errorf("invalid base64 in 'base64': could not decode the payload")
}

func (s *Server) toolSendLink(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendLink(&send_service.LinkStruct{
		Number:      argString(args, "number"),
		Text:        argString(args, "text"),
		Url:         argString(args, "url"),
		Title:       argString(args, "title"),
		Description: argString(args, "description"),
		ImgUrl:      argString(args, "image_url"),
		Delay:       int32(argInt(args, "delay_ms", 0)),
		Quoted:      quotedFrom(args),
	}, instance)
	if err != nil {
		return errorResult("failed to send link: " + err.Error())
	}

	return sendResult(instance, sent, "link")
}

func (s *Server) toolSendLocation(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	lat, okLat := argFloat(args, "latitude")
	lon, okLon := argFloat(args, "longitude")
	if !okLat || !okLon {
		return errorResult("'latitude' and 'longitude' are required and must be numbers")
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendLocation(&send_service.LocationStruct{
		Number:    argString(args, "number"),
		Name:      argString(args, "name"),
		Address:   argString(args, "address"),
		Latitude:  lat,
		Longitude: lon,
		Delay:     int32(argInt(args, "delay_ms", 0)),
	}, instance)
	if err != nil {
		return errorResult("failed to send location: " + err.Error())
	}

	return sendResult(instance, sent, "location")
}

func (s *Server) toolSendContact(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	fullName := argString(args, "full_name")
	phone := argString(args, "phone")
	if fullName == "" || phone == "" {
		return errorResult("'full_name' and 'phone' are required")
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendContact(&send_service.ContactStruct{
		Number: argString(args, "number"),
		Vcard: utils.VCardStruct{
			FullName:     fullName,
			Phone:        phone,
			Organization: argString(args, "organization"),
		},
		Delay: int32(argInt(args, "delay_ms", 0)),
	}, instance)
	if err != nil {
		return errorResult("failed to send contact: " + err.Error())
	}

	return sendResult(instance, sent, "contact")
}

func (s *Server) toolSendSticker(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	url := argString(args, "url")
	if url == "" {
		return errorResult("the 'url' argument is required")
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendSticker(&send_service.StickerStruct{
		Number:  argString(args, "number"),
		Sticker: url,
		Delay:   int32(argInt(args, "delay_ms", 0)),
	}, instance)
	if err != nil {
		return errorResult("failed to send sticker: " + err.Error())
	}

	return sendResult(instance, sent, "sticker")
}

func (s *Server) toolSendPoll(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	question := argString(args, "question")
	options := argStringSlice(args, "options")
	if question == "" {
		return errorResult("the 'question' argument is required")
	}
	if len(options) < 2 {
		return errorResult("a poll needs at least 2 options")
	}

	maxAnswers := argInt(args, "max_answers", 1)
	if maxAnswers < 1 || maxAnswers > len(options) {
		maxAnswers = 1
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendPoll(&send_service.PollStruct{
		Number:    argString(args, "number"),
		Question:  question,
		Options:   options,
		MaxAnswer: maxAnswers,
		Delay:     int32(argInt(args, "delay_ms", 0)),
	}, instance)
	if err != nil {
		return errorResult("failed to send poll: " + err.Error())
	}

	return sendResult(instance, sent, "poll")
}

// buildButtons maps the tool's snake_case button objects onto the API struct and
// enforces the same combination rules the HTTP layer applies, so the model gets
// an actionable message instead of a generic server error.
func buildButtons(raw []map[string]any) ([]send_service.Button, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("at least one button is required")
	}

	buttons := make([]send_service.Button, 0, len(raw))
	replyCount, otherCount, pixCount := 0, 0, 0

	for i, item := range raw {
		kind := strings.ToLower(argString(item, "type"))
		button := send_service.Button{
			Type:        kind,
			DisplayText: argString(item, "display_text"),
		}

		switch kind {
		case "reply":
			replyCount++
			button.Id = argString(item, "id")
			if button.Id == "" {
				return nil, fmt.Errorf("button %d (reply): 'id' is required as the callback payload", i+1)
			}
		case "url":
			otherCount++
			button.URL = argString(item, "url")
			if button.URL == "" {
				return nil, fmt.Errorf("button %d (url): 'url' is required", i+1)
			}
		case "call":
			otherCount++
			button.PhoneNumber = argString(item, "phone_number")
			if button.PhoneNumber == "" {
				return nil, fmt.Errorf("button %d (call): 'phone_number' is required (E.164, e.g. +5511999999999)", i+1)
			}
		case "copy":
			otherCount++
			button.CopyCode = argString(item, "copy_code")
			button.Id = argString(item, "id")
			if button.CopyCode == "" {
				return nil, fmt.Errorf("button %d (copy): 'copy_code' is required", i+1)
			}
		case "pix":
			pixCount++
			button.Currency = argString(item, "currency")
			button.Name = argString(item, "name")
			button.KeyType = strings.ToLower(argString(item, "key_type"))
			button.Key = argString(item, "key")
			if button.Currency == "" {
				button.Currency = "BRL"
			}
			if button.Key == "" || button.KeyType == "" || button.Name == "" {
				return nil, fmt.Errorf("button %d (pix): 'name', 'key' and 'key_type' are required", i+1)
			}
			switch button.KeyType {
			case "phone", "email", "cpf", "cnpj", "random":
			default:
				return nil, fmt.Errorf("button %d (pix): 'key_type' must be phone, email, cpf, cnpj or random", i+1)
			}
		default:
			return nil, fmt.Errorf("button %d: 'type' must be reply, url, call, copy or pix", i+1)
		}

		if kind != "pix" && button.DisplayText == "" {
			return nil, fmt.Errorf("button %d (%s): 'display_text' is required", i+1, kind)
		}

		buttons = append(buttons, button)
	}

	// Mirror the server-side rules up front, with the reason spelled out.
	if replyCount > 3 {
		return nil, fmt.Errorf("at most 3 reply buttons are allowed (got %d)", replyCount)
	}
	if replyCount > 0 && otherCount > 0 {
		return nil, fmt.Errorf("reply buttons cannot be mixed with url/call/copy buttons — " +
			"send only reply buttons, or only CTA buttons")
	}
	if pixCount > 0 && len(raw) > 1 {
		return nil, fmt.Errorf("a pix button must be the only button in the message")
	}

	return buttons, nil
}

func (s *Server) toolSendButtons(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	buttons, err := buildButtons(argObjects(args, "buttons"))
	if err != nil {
		return errorResult(err.Error())
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendButton(&send_service.ButtonStruct{
		Number:      argString(args, "number"),
		Title:       argString(args, "title"),
		Description: argString(args, "description"),
		Footer:      argString(args, "footer"),
		Buttons:     buttons,
		Delay:       int32(argInt(args, "delay_ms", 0)),
		Quoted:      quotedFrom(args),
	}, instance)
	if err != nil {
		return errorResult("failed to send buttons: " + err.Error())
	}

	return sendResult(instance, sent, "buttons")
}

func (s *Server) toolSendList(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	rawSections := argObjects(args, "sections")
	if len(rawSections) == 0 {
		return errorResult("at least one section is required")
	}

	sections := make([]send_service.Section, 0, len(rawSections))
	totalRows := 0
	for _, rawSection := range rawSections {
		rawRows := argObjects(rawSection, "rows")
		rows := make([]send_service.Row, 0, len(rawRows))
		for _, rawRow := range rawRows {
			title := argString(rawRow, "title")
			if title == "" {
				return errorResult("every list row needs a 'title'")
			}
			rows = append(rows, send_service.Row{
				Title:       title,
				Description: argString(rawRow, "description"),
				RowId:       argString(rawRow, "row_id"),
			})
		}
		totalRows += len(rows)
		sections = append(sections, send_service.Section{
			Title: argString(rawSection, "title"),
			Rows:  rows,
		})
	}

	if totalRows == 0 {
		return errorResult("the list needs at least one row")
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendList(&send_service.ListStruct{
		Number:      argString(args, "number"),
		Title:       argString(args, "title"),
		Description: argString(args, "description"),
		ButtonText:  argString(args, "button_text"),
		FooterText:  argString(args, "footer"),
		Sections:    sections,
		Delay:       int32(argInt(args, "delay_ms", 0)),
		Quoted:      quotedFrom(args),
	}, instance)
	if err != nil {
		return errorResult("failed to send list: " + err.Error())
	}

	return sendResult(instance, sent, "list")
}

func (s *Server) toolSendCarousel(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	rawCards := argObjects(args, "cards")
	if len(rawCards) == 0 {
		return errorResult("at least one card is required")
	}

	cards := make([]send_service.CarouselCardStruct, 0, len(rawCards))
	for i, rawCard := range rawCards {
		text := argString(rawCard, "text")
		if text == "" {
			return errorResult(fmt.Sprintf("card %d: 'text' is required", i+1))
		}

		rawButtons := argObjects(rawCard, "buttons")
		buttons := make([]send_service.CarouselButtonStruct, 0, len(rawButtons))
		for j, rawButton := range rawButtons {
			kind := strings.ToUpper(argString(rawButton, "type"))
			if kind == "" {
				kind = "REPLY"
			}

			button := send_service.CarouselButtonStruct{
				Type:        kind,
				DisplayText: argString(rawButton, "display_text"),
				Id:          argString(rawButton, "id"),
				CopyCode:    argString(rawButton, "copy_code"),
			}

			switch kind {
			case "REPLY", "URL", "CALL":
				// Carousel buttons carry the URL / phone number in `id` — there are
				// no dedicated fields, unlike send_buttons_message.
				if button.Id == "" {
					return errorResult(fmt.Sprintf(
						"card %d button %d (%s): 'id' is required (payload for REPLY, the URL for URL, the phone number for CALL)",
						i+1, j+1, kind,
					))
				}
			case "COPY":
				if button.CopyCode == "" {
					return errorResult(fmt.Sprintf("card %d button %d (COPY): 'copy_code' is required", i+1, j+1))
				}
			case "PIX":
				return errorResult("pix buttons are not supported inside carousels — use send_buttons_message")
			default:
				return errorResult(fmt.Sprintf("card %d button %d: 'type' must be REPLY, URL, CALL or COPY", i+1, j+1))
			}

			if button.DisplayText == "" {
				return errorResult(fmt.Sprintf("card %d button %d: 'display_text' is required", i+1, j+1))
			}

			buttons = append(buttons, button)
		}

		cards = append(cards, send_service.CarouselCardStruct{
			Header: send_service.CarouselCardHeaderStruct{
				Title:    argString(rawCard, "title"),
				Subtitle: argString(rawCard, "subtitle"),
				ImageUrl: argString(rawCard, "image_url"),
				VideoUrl: argString(rawCard, "video_url"),
			},
			Body:    send_service.CarouselCardBodyStruct{Text: text},
			Footer:  argString(rawCard, "footer"),
			Buttons: buttons,
		})
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendCarousel(&send_service.CarouselStruct{
		Number: argString(args, "number"),
		Body:   argString(args, "body"),
		Footer: argString(args, "footer"),
		Cards:  cards,
		Delay:  int32(argInt(args, "delay_ms", 0)),
		Quoted: quotedFrom(args),
	}, instance)
	if err != nil {
		return errorResult("failed to send carousel: " + err.Error())
	}

	return sendResult(instance, sent, "carousel")
}

func (s *Server) toolSendStatus(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	url := argString(args, "url")
	text := argString(args, "text")

	if url != "" {
		mediaType := strings.ToLower(argString(args, "type"))
		switch mediaType {
		case "image", "video", "audio":
		default:
			return errorResult("'type' must be image, video or audio when posting a media status")
		}

		if failure := requireConnected(instance); failure != nil {
			return failure
		}

		sent, err := s.sendService.SendStatusMediaUrl(&send_service.StatusMediaStruct{
			Type:    mediaType,
			Url:     url,
			Caption: argString(args, "caption"),
		}, instance)
		if err != nil {
			return errorResult("failed to post media status: " + err.Error())
		}
		return sendResult(instance, sent, "status_"+mediaType)
	}

	if text == "" {
		return errorResult("provide 'text' for a text status, or 'url' plus 'type' for a media status")
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	sent, err := s.sendService.SendStatusText(&send_service.StatusTextStruct{Text: text}, instance)
	if err != nil {
		return errorResult("failed to post status: " + err.Error())
	}

	return sendResult(instance, sent, "status_text")
}

func (s *Server) toolPlaceCall(sess *session, args map[string]any) *callToolResult {
	instance, failure := s.sendPrelude(sess, args)
	if failure != nil {
		return failure
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	snapshot, err := s.callService.OfferCall(&call_service.OfferCallStruct{
		Number: argString(args, "number"),
	}, instance)
	if err != nil {
		return errorResult("failed to place call: " + err.Error())
	}

	data, _ := json.MarshalIndent(snapshot, "", "  ")
	return textResult(string(data))
}

func (s *Server) toolEndCall(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	callID := argString(args, "call_id")
	if callID == "" {
		return errorResult("the 'call_id' argument is required")
	}

	if err := s.callService.TerminateCall(&call_service.CallActionStruct{
		CallID: callID,
		Reason: "user_ended",
	}, instance); err != nil {
		return errorResult("failed to end call: " + err.Error())
	}

	return jsonResult(map[string]any{"status": "ended", "callId": callID, "instance": instance.Name})
}

// toolDeleteMessage revokes a message for everyone. Arguments are validated
// before the connectivity check so a malformed call reports the real mistake
// rather than blaming the connection.
func (s *Server) toolDeleteMessage(sess *session, args map[string]any) *callToolResult {
	instance, fail := s.sendPrelude(sess, args)
	if fail != nil {
		return fail
	}

	number := argString(args, "number")
	messageID := argString(args, "message_id")

	if number == "" {
		return errorResult("the 'number' argument is required")
	}
	if messageID == "" {
		return errorResult("the 'message_id' argument is required")
	}
	if fail := requireConnected(instance); fail != nil {
		return fail
	}

	// The revoke is itself a message, so it comes back with an id of its own —
	// distinct from the id of the message that was taken down.
	revokeID, _, err := s.messageService.DeleteMessageEveryone(&message_service.MessageStruct{
		Chat:      number,
		MessageID: messageID,
	}, instance)
	if err != nil {
		return errorResult("failed to delete the message: " + err.Error())
	}

	return jsonResult(map[string]any{
		"status":           "deleted",
		"deletedMessageId": messageID,
		"revokeMessageId":  revokeID,
		"chat":             number,
		"instance":         instance.Name,
	})
}
