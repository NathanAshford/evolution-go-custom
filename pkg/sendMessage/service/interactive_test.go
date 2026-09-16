package send_service

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/EvolutionAPI/evolution-go/pkg/utils"
	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func interactiveFixtures(t *testing.T) map[string]*waE2E.Message {
	t.Helper()
	messages := make(map[string]*waE2E.Message)
	for _, kind := range []string{"reply", "copy", "url", "call", "pix"} {
		msg, err := buildButtonMessage(&ButtonStruct{Title: "Titulo", Description: "Corpo", Footer: "Rodape", Buttons: []Button{{Type: kind, DisplayText: "Escolher", Id: "id", URL: "https://example.com", CopyCode: "code", PhoneNumber: "+5511999999999", Currency: "BRL", Name: "Loja", Key: "pix-key", KeyType: "random"}}})
		if err != nil {
			t.Fatal(err)
		}
		messages[kind] = msg
	}
	list, err := buildListMessage(&ListStruct{Title: "Lista", Description: "Corpo", FooterText: "Rodape", Sections: []Section{{Title: "Secao", Rows: []Row{{Title: "Opcao", RowId: "row"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	messages["list"] = list
	carousel, err := buildCarouselMessage(&CarouselStruct{Body: "Corpo", Footer: "Rodape", Cards: []CarouselCardStruct{{Header: CarouselCardHeaderStruct{Title: "Titulo", Subtitle: "Subtitulo"}, Body: CarouselCardBodyStruct{Text: "Card"}, Buttons: []CarouselButtonStruct{{Type: "REPLY", DisplayText: "Escolher", Id: "id"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	messages["carousel"] = carousel
	return messages
}

func TestInteractivePayloadsTravelUnwrapped(t *testing.T) {
	secrets := make(map[string]bool)
	for name, msg := range interactiveFixtures(t) {
		t.Run(name, func(t *testing.T) {
			raw, err := proto.Marshal(msg)
			if err != nil {
				t.Fatal(err)
			}
			decoded := &waE2E.Message{}
			if err := proto.Unmarshal(raw, decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.InteractiveMessage == nil {
				t.Fatal("missing top-level interactive message")
			}
			if decoded.DocumentWithCaptionMessage != nil || decoded.ViewOnceMessage != nil || decoded.ViewOnceMessageV2 != nil || decoded.ViewOnceMessageV2Extension != nil || decoded.EphemeralMessage != nil {
				t.Fatal("interactive message wrapped")
			}
			if decoded.ButtonsMessage != nil || decoded.ListMessage != nil {
				t.Fatal("legacy message emitted")
			}
			if decoded.InteractiveMessage.ContextInfo == nil {
				t.Fatal("missing context")
			}
			ctx := decoded.GetMessageContextInfo()
			if len(ctx.GetMessageSecret()) != 32 || bytes.Equal(ctx.GetMessageSecret(), make([]byte, 32)) {
				t.Fatal("missing random 32-byte secret")
			}
			if secrets[string(ctx.GetMessageSecret())] {
				t.Fatal("secret reused")
			}
			secrets[string(ctx.GetMessageSecret())] = true
			if ctx.DeviceListMetadata == nil || ctx.GetDeviceListMetadataVersion() != 2 {
				t.Fatal("metadata differs from reference")
			}
			if utils.GetMessageType(decoded) != "interactive" {
				t.Fatal("outgoing message misclassified")
			}
		})
	}
}

func TestButtonNativeFlowPayloads(t *testing.T) {
	label := "Escolha \"A\"\\B\nlinha — ação"
	// Every field carries a DIFFERENT value on purpose: if a builder reads the
	// wrong struct field (Id where CopyCode was meant, and so on) the assertion
	// below must fail. Feeding one shared string into all four fields would
	// make any such field-crossing bug invisible.
	const (
		idValue    = "id\"\\\nvalue"
		urlValue   = "https://example.com/?q=\"a\""
		phoneValue = "+5511999999999"
		copyValue  = "copy\"\\\ncode"
	)
	for _, tt := range []struct{ kind, name, key, value string }{
		{"reply", "quick_reply", "id", idValue}, {"url", "cta_url", "url", urlValue},
		{"call", "cta_call", "phone_number", phoneValue}, {"copy", "cta_copy", "copy_code", copyValue},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			msg, err := buildButtonMessage(&ButtonStruct{Title: "Titulo", Description: "Corpo", Footer: "Rodape", Buttons: []Button{{Type: tt.kind, DisplayText: label, Id: idValue, URL: urlValue, PhoneNumber: phoneValue, CopyCode: copyValue}}})
			if err != nil {
				t.Fatal(err)
			}
			flow := msg.InteractiveMessage.GetNativeFlowMessage()
			if flow.GetMessageVersion() != 1 || flow.MessageParamsJSON != nil || len(flow.Buttons) != 1 {
				t.Fatalf("unexpected flow: %v", flow)
			}
			btn := flow.Buttons[0]
			var params map[string]string
			if err := json.Unmarshal([]byte(btn.GetButtonParamsJSON()), &params); err != nil {
				t.Fatal(err)
			}
			if btn.GetName() != tt.name || params["display_text"] != label || params[tt.key] != tt.value {
				t.Fatalf("incorrect params: %v", params)
			}
			if tt.kind == "url" && params["merchant_url"] != tt.value {
				t.Fatal("missing merchant_url")
			}
			if msg.InteractiveMessage.GetFooter().GetText() != "Rodape" {
				t.Fatal("footer lost")
			}
			if tt.kind == "reply" && (msg.InteractiveMessage.GetHeader().GetTitle() != "Titulo" || msg.InteractiveMessage.GetBody().GetText() != "Corpo") {
				t.Fatal("reply content lost")
			}
		})
	}
	pix := interactiveFixtures(t)["pix"].InteractiveMessage.GetNativeFlowMessage()
	if pix.GetMessageParamsJSON() != `{"native_flow_name":"order_details","version":1}` || pix.Buttons[0].GetName() != "payment_info" {
		t.Fatal("pix-specific params changed")
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(pix.Buttons[0].GetButtonParamsJSON()), &params); err != nil {
		t.Fatal(err)
	}
	// Compare against the concrete type: a missing key yields an untyped nil
	// interface, and nil != "" is always true, so `params[k] == ""` would never
	// catch an absent reference_id.
	if ref, ok := params["reference_id"].(string); !ok || ref == "" {
		t.Fatalf("pix reference_id missing or not a string: %#v", params["reference_id"])
	}
	if params["currency"] != "BRL" {
		t.Fatal("pix payload lost")
	}
	settings := params["payment_settings"].([]any)[0].(map[string]any)["pix_static_code"].(map[string]any)
	if settings["key"] != "pix-key" || settings["key_type"] != "EVP" {
		t.Fatal("pix key lost")
	}
}

func TestButtonValidation(t *testing.T) {
	for _, buttons := range [][]Button{nil, {{Type: "unsupported"}}, {{Type: "reply"}, {Type: "url"}}, {{Type: "pix"}, {Type: "reply"}}, {{Type: "reply"}, {Type: "reply"}, {Type: "reply"}, {Type: "reply"}}} {
		if _, err := buildButtonMessage(&ButtonStruct{Buttons: buttons}); err == nil {
			t.Errorf("accepted invalid buttons: %+v", buttons)
		}
	}
	for _, buttons := range [][]Button{{{Type: "reply"}, {Type: "reply"}, {Type: "reply"}}, {{Type: "copy"}, {Type: "url"}, {Type: "call"}}} {
		if _, err := buildButtonMessage(&ButtonStruct{Buttons: buttons}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCarouselEscapesButtonParams(t *testing.T) {
	for _, tt := range []struct{ kind, name, key string }{{"URL", "cta_url", "url"}, {"CALL", "cta_call", "phone_number"}, {"COPY", "cta_copy", "copy_code"}, {"REPLY", "quick_reply", "id"}, {"", "quick_reply", "id"}, {"url", "cta_url", "url"}} {
		t.Run(tt.kind, func(t *testing.T) {
			// Distinct per field for the same reason as TestButtonNativeFlowPayloads:
			// a shared value hides a builder that reads the wrong field.
			value := "texto \"aspas\"\\barra\nlinha"
			idValue, copyValue := value+"#id", value+"#copy"
			if tt.key == "copy_code" {
				value = copyValue
			} else {
				value = idValue
			}
			msg, err := buildCarouselMessage(&CarouselStruct{Body: "Acima", Footer: "Abaixo", Cards: []CarouselCardStruct{{Header: CarouselCardHeaderStruct{Title: "Titulo", Subtitle: "Subtitulo"}, Body: CarouselCardBodyStruct{Text: "Card"}, Footer: "Rodape", Buttons: []CarouselButtonStruct{{Type: tt.kind, DisplayText: value, Id: idValue, CopyCode: copyValue}}}}})
			if err != nil {
				t.Fatal(err)
			}
			card := msg.InteractiveMessage.GetCarouselMessage().Cards[0]
			flow := card.GetNativeFlowMessage()
			if flow.MessageVersion != nil || flow.MessageParamsJSON != nil {
				t.Fatal("card includes unsupported flow metadata")
			}
			var params map[string]string
			if err := json.Unmarshal([]byte(flow.Buttons[0].GetButtonParamsJSON()), &params); err != nil {
				t.Fatal(err)
			}
			if flow.Buttons[0].GetName() != tt.name || params["display_text"] != value || params[tt.key] != value {
				t.Fatalf("incorrect params: %v", params)
			}
			if card.GetHeader().GetSubtitle() != "Subtitulo" || card.GetFooter().GetText() != "Rodape" || msg.InteractiveMessage.GetFooter().GetText() != "Abaixo" {
				t.Fatal("carousel content lost")
			}
		})
	}
}

func TestInteractiveQuotePreservesContext(t *testing.T) {
	for name, msg := range interactiveFixtures(t) {
		t.Run(name, func(t *testing.T) {
			ctx := msg.InteractiveMessage.ContextInfo
			ctx.MentionedJID = []string{"5511999999999@s.whatsapp.net"}
			setInteractiveQuote(msg.InteractiveMessage, QuotedStruct{MessageID: "quoted-id", Participant: "5511999999999@s.whatsapp.net"})
			if ctx != msg.InteractiveMessage.ContextInfo || ctx.GetStanzaID() != "quoted-id" || ctx.GetParticipant() != "5511999999999@s.whatsapp.net" || ctx.QuotedMessage == nil || len(ctx.MentionedJID) != 1 {
				t.Fatal("quote/context lost")
			}
		})
	}
}

func TestInteractiveHeaderShape(t *testing.T) {
	if buildInteractiveHeader(" ", nil) != nil {
		t.Fatal("empty header emitted")
	}
	for _, media := range []*waE2E.Message{{ImageMessage: &waE2E.ImageMessage{}}, {VideoMessage: &waE2E.VideoMessage{}}, {DocumentMessage: &waE2E.DocumentMessage{}}} {
		h := buildInteractiveHeader("Titulo", media)
		if h == nil || !h.GetHasMediaAttachment() || h.Media == nil || h.GetTitle() != "Titulo" {
			t.Fatalf("invalid header: %v", h)
		}
	}
	h := buildInteractiveHeader("Titulo", nil)
	if h.GetHasMediaAttachment() || h.Media != nil {
		t.Fatal("text header claims media")
	}
}

func TestInteractiveWireNodes(t *testing.T) {
	const want = `<biz><interactive type="native_flow" v="1"><native_flow name="mixed" v="9"/></interactive></biz>`
	for _, tt := range []struct {
		number string
		count  int
	}{{"5511999999999", 2}, {"5511999999999@s.whatsapp.net", 2}, {"12345@lid", 2}, {"120363000000000000@g.us", 1}, {"120363000000000000", 1}, {"123456789012-123456789012", 1}} {
		t.Run(tt.number, func(t *testing.T) {
			recipient, err := validateMessageFields(tt.number, nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			nodes := interactiveBizNodes(recipient)
			if len(nodes) != tt.count {
				t.Fatalf("got %d nodes, want %d", len(nodes), tt.count)
			}
			if got := renderInteractiveNode(nodes[0]); got != want {
				t.Fatalf("biz node: %s", got)
			}
			if len(nodes) == 2 && renderInteractiveNode(nodes[1]) != `<bot biz_bot="1"/>` {
				t.Fatal("incorrect bot node")
			}
		})
	}
}

func renderInteractiveNode(n waBinary.Node) string {
	keys := make([]string, 0, len(n.Attrs))
	for k := range n.Attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteString("<" + n.Tag)
	for _, k := range keys {
		fmt.Fprintf(&out, " %s=%q", k, fmt.Sprint(n.Attrs[k]))
	}
	kids, _ := n.Content.([]waBinary.Node)
	if len(kids) == 0 {
		out.WriteString("/>")
		return out.String()
	}
	out.WriteString(">")
	for _, kid := range kids {
		out.WriteString(renderInteractiveNode(kid))
	}
	out.WriteString("</" + n.Tag + ">")
	return out.String()
}

// Check the actual dependency source, not a stale copy of its detection logic.
// A dependency change needs review before we keep injecting our own <biz> node.
func TestPinnedWhatsmeowButtonDetection(t *testing.T) {
	out, err := exec.Command("go", "list", "-mod=readonly", "-f", "{{.Dir}}", "go.mau.fi/whatsmeow").Output()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), "send.go"))
	if err != nil {
		t.Fatal(err)
	}
	start := bytes.Index(src, []byte("func getButtonTypeFromMessage("))
	if start < 0 {
		t.Fatal("upstream detection moved; review biz-node injection")
	}
	end := bytes.Index(src[start:], []byte("\n}\n"))
	if end < 0 {
		t.Fatal("upstream function changed")
	}
	got := fmt.Sprintf("%x", sha256.Sum256(src[start:start+end+2]))
	if got != "44853478b6aae222dcab7b8cd607983e93a913c6364c468a2f7881046ac1101e" {
		t.Fatal("whatsmeow button detection changed; check for duplicate biz nodes before updating guard")
	}
}

func TestHeaderMediaReadLimits(t *testing.T) {
	for _, tt := range []struct {
		body  string
		limit int64
		fail  bool
	}{{"abc", 3, false}, {"abcd", 3, true}, {"", 3, true}} {
		data, err := readHeaderMedia(strings.NewReader(tt.body), tt.limit)
		if (err != nil) != tt.fail {
			t.Fatalf("body %q: %v", tt.body, err)
		}
		if !tt.fail && string(data) != tt.body {
			t.Fatal("body truncated")
		}
	}
}

// largeBodyBytes is comfortably over the limit but small enough to serve
// quickly; fetchHeaderMedia must reject it on the declared Content-Length
// without buffering it.
const largeBodyBytes = maxHeaderMediaBytes + 1

// zeroReader is an endless source of zero bytes, used to stream an oversize
// chunked body without allocating it.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { return len(p), nil }

func TestFetchHeaderMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			io.WriteString(w, "image-data")
		case "/empty":
			w.WriteHeader(http.StatusNoContent)
		case "/large":
			// Declares an oversize body and sends nothing. Only the
			// Content-Length pre-check can produce a "size limit" error here:
			// without it the read fails with "unexpected EOF" instead, which
			// is what the message assertion below distinguishes.
			w.Header().Set("Content-Length", fmt.Sprint(largeBodyBytes))
			w.WriteHeader(http.StatusOK)
		case "/large-chunked":
			// No Content-Length, so the pre-check cannot fire and
			// readHeaderMedia's own limit is the only thing standing between
			// an attacker and unbounded memory. Defense in depth, tested.
			w.WriteHeader(http.StatusOK)
			io.CopyN(w, zeroReader{}, largeBodyBytes)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	for _, path := range []string{"/ok", "/empty", "/large", "/large-chunked", "/missing"} {
		data, err := fetchHeaderMedia(server.URL + path)
		if path == "/ok" {
			if err != nil || string(data) != "image-data" {
				t.Fatalf("%q: %v", data, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("accepted %s", path)
		}
		if strings.HasPrefix(path, "/large") && !strings.Contains(err.Error(), "size limit") {
			t.Fatalf("oversize body rejected for the wrong reason: %v", err)
		}
	}
	if _, err := fetchHeaderMedia(":bad url"); err == nil {
		t.Fatal("accepted invalid URL")
	}
}

func TestHeaderThumbnailBounds(t *testing.T) {
	for _, size := range []image.Point{{200, 100}, {1, 5000}, {5000, 1}} {
		var input bytes.Buffer
		if err := png.Encode(&input, image.NewRGBA(image.Rect(0, 0, size.X, size.Y))); err != nil {
			t.Fatal(err)
		}
		thumb := jpegThumbnailOf(input.Bytes())
		img, _, err := image.Decode(bytes.NewReader(thumb))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() > 72 || img.Bounds().Dy() > 72 {
			t.Fatal("unbounded thumbnail")
		}
	}
	if jpegThumbnailOf([]byte("not an image")) != nil {
		t.Fatal("invalid image accepted")
	}

	// Cross the pixel guard with an image that genuinely decodes. This is the
	// decompression-bomb shape: 6000x6000 = 36Mpx of uniform grey compresses to
	// about 60KB, so a tiny download expands to a 36MB allocation (much worse
	// for a colour image). The guard must reject it from the header alone,
	// before image.Decode is ever called.
	var bomb bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&bomb, image.NewGray(image.Rect(0, 0, 6000, 6000))); err != nil {
		t.Fatal(err)
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(bomb.Bytes())); err != nil || int64(cfg.Width)*int64(cfg.Height) <= 32<<20 {
		t.Fatalf("fixture does not exceed the pixel bound: %dx%d err=%v", cfg.Width, cfg.Height, err)
	}
	if thumb := jpegThumbnailOf(bomb.Bytes()); thumb != nil {
		t.Fatal("a 36-megapixel image was decoded: the pixel bound did not fire")
	}
}

func TestCarouselLimits(t *testing.T) {
	validCard := CarouselCardStruct{Body: CarouselCardBodyStruct{Text: "Card"}}
	cases := []*CarouselStruct{
		{}, {Cards: make([]CarouselCardStruct, maxCarouselCards+1)},
		{Cards: []CarouselCardStruct{{Body: CarouselCardBodyStruct{Text: " \n"}}}},
		{Cards: []CarouselCardStruct{{Body: validCard.Body, Buttons: make([]CarouselButtonStruct, maxCardButtons+1)}}},
	}
	for _, data := range cases {
		if _, err := buildCarouselMessage(data); err == nil {
			t.Errorf("invalid carousel accepted: %+v", data)
		}
	}
	cards := make([]CarouselCardStruct, maxCarouselCards)
	for i := range cards {
		cards[i] = validCard
		cards[i].Buttons = make([]CarouselButtonStruct, maxCardButtons)
	}
	if _, err := buildCarouselMessage(&CarouselStruct{Cards: cards}); err != nil {
		t.Fatal(err)
	}
}

// TestInteractiveSendNodesWiring covers the decision SendMessage makes about
// which plaintext stanza nodes accompany a message. The WhatsApp server judges
// only these nodes, so "the constructor returns the right nodes" is not enough:
// something must prove the constructor is actually reached for interactive
// messages and bypassed for everything else.
func TestInteractiveSendNodesWiring(t *testing.T) {
	direct := types.JID{User: "5511999999999", Server: types.DefaultUserServer}
	group := types.JID{User: "120363000000000000", Server: types.GroupServer}
	fallback := &[]waBinary.Node{{Tag: "custom"}}

	for name, msg := range interactiveFixtures(t) {
		t.Run("interactive/"+name, func(t *testing.T) {
			nodes, err := interactiveSendNodes(msg, direct, fallback)
			if err != nil {
				t.Fatalf("interactive send refused: %v", err)
			}
			if nodes == nil {
				t.Fatal("no additional nodes attached: the stanza would carry no <biz>, so the server drops the buttons")
			}
			if nodes == fallback {
				t.Fatal("caller-supplied nodes used instead of the interactive ones")
			}
			got := make([]string, 0, len(*nodes))
			for _, n := range *nodes {
				got = append(got, renderInteractiveNode(n))
			}
			want := []string{
				`<biz><interactive type="native_flow" v="1"><native_flow name="mixed" v="9"/></interactive></biz>`,
				`<bot biz_bot="1"/>`,
			}
			if strings.Join(got, "") != strings.Join(want, "") {
				t.Fatalf("wire nodes changed:\n got %v\nwant %v", got, want)
			}
		})
	}

	t.Run("group omits bot", func(t *testing.T) {
		nodes, err := interactiveSendNodes(interactiveFixtures(t)["reply"], group, nil)
		if err != nil {
			t.Fatal(err)
		}
		if nodes == nil || len(*nodes) != 1 {
			t.Fatalf("group must carry exactly the <biz> node, got %v", nodes)
		}
	})

	t.Run("non-interactive keeps caller nodes", func(t *testing.T) {
		text := &waE2E.Message{Conversation: proto.String("oi")}
		nodes, err := interactiveSendNodes(text, direct, fallback)
		if err != nil {
			t.Fatal(err)
		}
		if nodes != fallback {
			t.Fatal("caller-supplied AdditionalNodes dropped for a plain text message")
		}
		if nodes, err = interactiveSendNodes(text, direct, nil); err != nil || nodes != nil {
			t.Fatalf("plain text must not gain interactive nodes: %v %v", nodes, err)
		}
	})
}

// TestWrappedInteractiveIsRefused is the regression lock for the original bug.
// Every FutureProofMessage wrapper hides the buttons from every client, so a
// wrapped interactive message must never reach the wire — including when the
// wrapping happens downstream of the builders, in SendButton/SendList/
// SendCarousel or in SendMessage itself.
func TestWrappedInteractiveIsRefused(t *testing.T) {
	direct := types.JID{User: "5511999999999", Server: types.DefaultUserServer}
	inner := interactiveFixtures(t)["reply"]

	wrappers := map[string]*waE2E.Message{
		"DocumentWithCaptionMessage": {DocumentWithCaptionMessage: &waE2E.FutureProofMessage{Message: inner}},
		"ViewOnceMessage":            {ViewOnceMessage: &waE2E.FutureProofMessage{Message: inner}},
		"ViewOnceMessageV2":          {ViewOnceMessageV2: &waE2E.FutureProofMessage{Message: inner}},
		"ViewOnceMessageV2Extension": {ViewOnceMessageV2Extension: &waE2E.FutureProofMessage{Message: inner}},
		"nested DocumentWithCaption": {ViewOnceMessage: &waE2E.FutureProofMessage{Message: &waE2E.Message{DocumentWithCaptionMessage: &waE2E.FutureProofMessage{Message: inner}}}},
	}
	for name, msg := range wrappers {
		t.Run(name, func(t *testing.T) {
			if !wrappedInteractiveMessage(msg) {
				t.Fatal("wrapper not detected")
			}
			nodes, err := interactiveSendNodes(msg, direct, nil)
			if err == nil {
				t.Fatal("a wrapped interactive message was allowed onto the wire: buttons would silently not render")
			}
			if !errors.Is(err, interactiveDispatchError) {
				t.Fatalf("unexpected error: %v", err)
			}
			if nodes != nil {
				t.Fatal("nodes returned alongside the refusal")
			}
		})
	}

	// A wrapper around something that is NOT interactive is none of our business.
	doc := &waE2E.Message{DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
		Message: &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{}},
	}}
	if wrappedInteractiveMessage(doc) {
		t.Fatal("a genuine document-with-caption was misflagged")
	}
	if _, err := interactiveSendNodes(doc, direct, nil); err != nil {
		t.Fatalf("genuine document-with-caption refused: %v", err)
	}
	if wrappedInteractiveMessage(nil) {
		t.Fatal("nil message misflagged")
	}
}

// TestNoSourceWrapsInteractive is a static guard over the whole package.
//
// The runtime check in interactiveSendNodes already refuses a wrapped
// interactive message, but it can only fire on the send path a test can reach.
// The original bug was a wrapper applied in the exported Send* methods, which
// need a live WhatsApp client and so cannot be driven from a unit test. This
// walks the package AST instead and fails if ANY non-test source builds a
// FutureProofMessage that transitively contains an InteractiveMessage —
// wherever it is written.
func TestNoSourceWrapsInteractive(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no package parsed: the guard would silently pass")
	}

	// containsInteractive reports whether a composite literal sets the
	// InteractiveMessage field anywhere beneath it.
	var containsInteractive func(ast.Node) bool
	containsInteractive = func(n ast.Node) bool {
		found := false
		ast.Inspect(n, func(inner ast.Node) bool {
			kv, ok := inner.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "InteractiveMessage" {
				found = true
				return false
			}
			return true
		})
		return found
	}

	inspected := 0
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				sel, ok := lit.Type.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "FutureProofMessage" {
					return true
				}
				inspected++
				if containsInteractive(lit) {
					t.Errorf("%s: an InteractiveMessage is wrapped in a FutureProofMessage. "+
						"Clients dispatch on the outer field number and never descend, so the buttons "+
						"would not render on any phone. Send the InteractiveMessage at the top level.",
						fset.Position(lit.Pos()))
				}
				return true
			})
			_ = name
		}
	}
	t.Logf("scanned %d FutureProofMessage literals", inspected)
}

// TestReplyButtonsAlwaysCarryDistinctIDs guards the click round-trip. A
// quick_reply button whose params carry an empty id sends back an empty
// selection, so two such buttons are indistinguishable to the caller and the
// bot cannot tell which one the user tapped. Every reply button must therefore
// leave the builder with a non-empty id, unique within its message.
func TestReplyButtonsAlwaysCarryDistinctIDs(t *testing.T) {
	idsOf := func(t *testing.T, buttons []*waE2E.InteractiveMessage_NativeFlowMessage_NativeFlowButton) []string {
		t.Helper()
		ids := make([]string, 0, len(buttons))
		for _, b := range buttons {
			var params map[string]string
			if err := json.Unmarshal([]byte(b.GetButtonParamsJSON()), &params); err != nil {
				t.Fatal(err)
			}
			if params["id"] == "" {
				t.Fatalf("reply button %q has an empty id: a tap on it is indistinguishable from a tap on any other", params["display_text"])
			}
			ids = append(ids, params["id"])
		}
		return ids
	}
	assertDistinct := func(t *testing.T, ids []string) {
		t.Helper()
		seen := map[string]bool{}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("duplicate reply id %q: the two buttons cannot be told apart", id)
			}
			seen[id] = true
		}
	}

	t.Run("buttons", func(t *testing.T) {
		for _, buttons := range [][]Button{
			{{Type: "reply", DisplayText: "A"}, {Type: "reply", DisplayText: "B"}},
			{{Type: "reply", DisplayText: "A", Id: "chosen"}, {Type: "reply", DisplayText: "B"}},
			{{Type: "reply", DisplayText: "A", Id: "   "}, {Type: "reply", DisplayText: "B", Id: "b"}},
		} {
			msg, err := buildButtonMessage(&ButtonStruct{Description: "d", Buttons: buttons})
			if err != nil {
				t.Fatal(err)
			}
			assertDistinct(t, idsOf(t, msg.GetInteractiveMessage().GetNativeFlowMessage().GetButtons()))
		}
		// An explicitly supplied id must survive untouched.
		msg, err := buildButtonMessage(&ButtonStruct{Description: "d", Buttons: []Button{{Type: "reply", DisplayText: "A", Id: "keep-me"}}})
		if err != nil {
			t.Fatal(err)
		}
		if got := idsOf(t, msg.GetInteractiveMessage().GetNativeFlowMessage().GetButtons())[0]; got != "keep-me" {
			t.Fatalf("explicit id rewritten to %q", got)
		}
	})

	t.Run("carousel", func(t *testing.T) {
		msg, err := buildCarouselMessage(&CarouselStruct{Body: "b", Cards: []CarouselCardStruct{
			{Body: CarouselCardBodyStruct{Text: "c1"}, Buttons: []CarouselButtonStruct{{Type: "reply", DisplayText: "A"}, {Type: "reply", DisplayText: "B"}}},
			{Body: CarouselCardBodyStruct{Text: "c2"}, Buttons: []CarouselButtonStruct{{Type: "reply", DisplayText: "C"}, {Type: "reply", DisplayText: "D", Id: "explicit"}}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		// Ids must be unique across the WHOLE carousel, not just within a card:
		// the click arrives with no indication of which card produced it.
		var all []string
		for _, card := range msg.GetInteractiveMessage().GetCarouselMessage().GetCards() {
			all = append(all, idsOf(t, card.GetNativeFlowMessage().GetButtons())...)
		}
		assertDistinct(t, all)
	})
}
