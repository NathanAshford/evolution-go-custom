package mcp

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
)

func TestIsHTTPURL(t *testing.T) {
	yes := []string{"http://a.com/x.png", "https://a.com/x.png", "HTTPS://A.COM/X.PNG"}
	no := []string{"", "ftp://a.com/x.png", "iVBORw0KGgo=", "data:image/png;base64,iVBORw0KGgo="}

	for _, v := range yes {
		if !isHTTPURL(v) {
			t.Errorf("isHTTPURL(%q) = false, want true", v)
		}
	}
	for _, v := range no {
		if isHTTPURL(v) {
			t.Errorf("isHTTPURL(%q) = true, want false", v)
		}
	}
}

func TestDecodeBase64MediaAcceptsRealWorldShapes(t *testing.T) {
	want := "hello world"
	std := base64.StdEncoding.EncodeToString([]byte(want))

	cases := map[string]string{
		"bare standard":  std,
		"data URI":       "data:image/png;base64," + std,
		"data URI upper": "DATA:image/png;base64," + std,
		"line wrapped":   std[:4] + "\n" + std[4:],
		"with spaces":    std[:4] + " " + std[4:],
		"unpadded":       base64.RawStdEncoding.EncodeToString([]byte(want)),
		"url safe":       base64.URLEncoding.EncodeToString([]byte(want)),
	}

	for name, payload := range cases {
		got, err := decodeBase64Media(payload)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s: decoded %q, want %q", name, got, want)
		}
	}
}

func TestDecodeBase64MediaRejectsGarbage(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"whitespace only":  "   \n\t ",
		"not base64":       "!!! this is not base64 !!!",
		"data URI no body": "data:image/png;base64,",
		"data URI comma":   "data:image/png;base64",
	}

	for name, payload := range cases {
		if _, err := decodeBase64Media(payload); err == nil {
			t.Errorf("%s: expected an error, got none", name)
		}
	}
}

// callMedia drives send_media_message through the real dispatch with a session
// pinned to a disconnected instance, so nothing can actually be sent.
func callMedia(t *testing.T, args string) *callToolResult {
	t.Helper()

	s := &Server{}
	sess := &session{instance: &instance_model.Instance{Id: "i1", Name: "test", Connected: false}}

	res := s.callTool(sess, callToolParams{
		Name:      "send_media_message",
		Arguments: json.RawMessage(args),
	})
	if res == nil {
		t.Fatal("callTool returned nil")
	}
	return res
}

func resultText(res *callToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	return res.Content[0].Text
}

// A malformed payload must be reported as such. If the connectivity check ran
// first, every one of these would come back as "not connected" and the caller
// would never learn what was actually wrong.
func TestSendMediaReportsPayloadErrorsBeforeConnectivity(t *testing.T) {
	tests := []struct {
		name, args, wantSubstring string
	}{
		{
			"neither url nor base64",
			`{"number":"5511999999999","type":"image"}`,
			"either 'url'",
		},
		{
			"both url and base64",
			`{"number":"5511999999999","type":"image","url":"https://a.com/x.png","base64":"aGk="}`,
			"only one of",
		},
		{
			"invalid base64",
			`{"number":"5511999999999","type":"image","base64":"!!!nope!!!"}`,
			"base64",
		},
		{
			"bad media type",
			`{"number":"5511999999999","type":"hologram","url":"https://a.com/x.png"}`,
			"'type' must be one of",
		},
		{
			"missing number",
			`{"type":"image","url":"https://a.com/x.png"}`,
			"'number' argument is required",
		},
	}

	for _, tt := range tests {
		res := callMedia(t, tt.args)
		text := resultText(res)

		if !res.IsError {
			t.Errorf("%s: expected an error result, got success", tt.name)
		}
		if !strings.Contains(text, tt.wantSubstring) {
			t.Errorf("%s: message %q does not contain %q", tt.name, text, tt.wantSubstring)
		}
		if strings.Contains(text, "not connected") {
			t.Errorf("%s: connectivity was checked before the payload: %q", tt.name, text)
		}
	}
}

// A well-formed payload has nothing left to complain about, so it should reach
// the connectivity gate — which is what proves the ordering above is a real
// ordering and not just an accident of these inputs.
func TestSendMediaValidPayloadReachesConnectivityGate(t *testing.T) {
	valid := []string{
		`{"number":"5511999999999","type":"image","url":"https://a.com/x.png"}`,
		`{"number":"5511999999999","type":"image","base64":"aGVsbG8="}`,
		// REST parity: base64 passed in the "url" field, as POST /send/media allows.
		`{"number":"5511999999999","type":"image","url":"aGVsbG8="}`,
	}

	for _, args := range valid {
		text := resultText(callMedia(t, args))
		if !strings.Contains(text, "not connected") {
			t.Errorf("expected the connectivity gate for %s, got %q", args, text)
		}
	}
}
