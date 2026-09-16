package whatsmeow_service

import (
	"fmt"
	"testing"
)

func TestParseNativeFlowResponseParamsAliases(t *testing.T) {
	idKeys := []string{"id", "selectedRowId", "selected_row_id", "selectedId", "selected_id"}
	labelKeys := []string{"display_text", "title", "selectedDisplayText"}

	for _, idKey := range idKeys {
		for _, labelKey := range labelKeys {
			t.Run(idKey+"/"+labelKey, func(t *testing.T) {
				paramsJSON := fmt.Sprintf(`{%q:"option-123",%q:"Opção escolhida"}`, idKey, labelKey)
				buttonID, buttonText := parseNativeFlowResponseParams(paramsJSON)
				if buttonID != "option-123" || buttonText != "Opção escolhida" {
					t.Fatalf("parseNativeFlowResponseParams(%q) = (%q, %q), want (%q, %q)",
						paramsJSON, buttonID, buttonText, "option-123", "Opção escolhida")
				}
			})
		}
	}
}

func TestParseNativeFlowResponseParamsPrecedenceAndFallback(t *testing.T) {
	tests := []struct {
		name       string
		paramsJSON string
		wantID     string
		wantText   string
	}{
		{
			name:       "legacy keys take precedence",
			paramsJSON: `{"id":"legacy","selectedRowId":"row-camel","selected_row_id":"row-snake","selectedId":"selected-camel","selected_id":"selected-snake","display_text":"Legacy","title":"Title","selectedDisplayText":"Selected"}`,
			wantID:     "legacy",
			wantText:   "Legacy",
		},
		{
			name:       "empty legacy keys fall back to row and title",
			paramsJSON: `{"id":"","selectedRowId":"row-camel","selected_row_id":"row-snake","selectedId":"selected-camel","selected_id":"selected-snake","display_text":"","title":"Title","selectedDisplayText":"Selected"}`,
			wantID:     "row-camel",
			wantText:   "Title",
		},
		{
			name:       "empty row camel falls back to row snake",
			paramsJSON: `{"id":"","selectedRowId":"","selected_row_id":"row-snake","selectedId":"selected-camel","selected_id":"selected-snake","display_text":"","title":"","selectedDisplayText":"Selected"}`,
			wantID:     "row-snake",
			wantText:   "Selected",
		},
		{
			name:       "empty row snake falls back to selected camel",
			paramsJSON: `{"id":"","selectedRowId":"","selected_row_id":"","selectedId":"selected-camel","selected_id":"selected-snake"}`,
			wantID:     "selected-camel",
		},
		{
			name:       "empty selected camel falls back to selected snake",
			paramsJSON: `{"id":"","selectedRowId":"","selected_row_id":"","selectedId":"","selected_id":"selected-snake"}`,
			wantID:     "selected-snake",
		},
		{
			name:       "non-string legacy values fall back",
			paramsJSON: `{"id":123,"selectedRowId":"row-camel","display_text":false,"title":"Title"}`,
			wantID:     "row-camel",
			wantText:   "Title",
		},
		{
			name:       "non-string aliases fall back",
			paramsJSON: `{"id":null,"selectedRowId":{},"selected_row_id":[],"selectedId":true,"selected_id":"selected-snake","display_text":[],"title":{},"selectedDisplayText":"Selected"}`,
			wantID:     "selected-snake",
			wantText:   "Selected",
		},
		{
			name:       "label without id",
			paramsJSON: `{"selectedDisplayText":"Selected"}`,
			wantText:   "Selected",
		},
		{
			name:       "strings retain whitespace and escapes",
			paramsJSON: `{"id":" option-123 ","display_text":"  Opção \"A\"\n "}`,
			wantID:     " option-123 ",
			wantText:   "  Opção \"A\"\n ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buttonID, buttonText := parseNativeFlowResponseParams(tt.paramsJSON)
			if buttonID != tt.wantID || buttonText != tt.wantText {
				t.Fatalf("parseNativeFlowResponseParams(%q) = (%q, %q), want (%q, %q)",
					tt.paramsJSON, buttonID, buttonText, tt.wantID, tt.wantText)
			}
		})
	}
}

func TestParseNativeFlowResponseParamsIgnoresEmptyAndNonStringValues(t *testing.T) {
	keys := []string{"id", "selectedRowId", "selected_row_id", "selectedId", "selected_id", "display_text", "title", "selectedDisplayText"}
	values := []struct {
		name string
		json string
	}{
		{"empty string", `""`},
		{"null", `null`},
		{"number", `123`},
		{"boolean", `true`},
		{"object", `{"id":"nested","display_text":"Nested"}`},
		{"array", `["option-123"]`},
	}

	for _, key := range keys {
		for _, value := range values {
			t.Run(key+"/"+value.name, func(t *testing.T) {
				paramsJSON := fmt.Sprintf(`{%q:%s}`, key, value.json)
				buttonID, buttonText := parseNativeFlowResponseParams(paramsJSON)
				if buttonID != "" || buttonText != "" {
					t.Fatalf("parseNativeFlowResponseParams(%q) = (%q, %q), want empty strings",
						paramsJSON, buttonID, buttonText)
				}
			})
		}
	}
}

func TestParseNativeFlowResponseParamsMissingOrInvalidJSON(t *testing.T) {
	tests := []struct {
		name       string
		paramsJSON string
	}{
		{"empty input", ""},
		{"whitespace input", " \n\t "},
		{"empty object", `{}`},
		{"empty key", `{"":"ignored"}`},
		{"unrelated keys", `{"name":"quick_reply","description":"ignored"}`},
		{"null", `null`},
		{"array", `[{"id":"option-123","display_text":"Selected"}]`},
		{"string", `"option-123"`},
		{"number", `123`},
		{"boolean", `true`},
		{"malformed JSON", `not JSON`},
		{"truncated object", `{"id":"option-123","display_text":"Selected"`},
		{"trailing comma", `{"id":"option-123","display_text":"Selected",}`},
		{"trailing data", `{"id":"option-123","display_text":"Selected"} garbage`},
		{"multiple objects", `{"id":"option-123"}{"display_text":"Selected"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buttonID, buttonText := parseNativeFlowResponseParams(tt.paramsJSON)
			if buttonID != "" || buttonText != "" {
				t.Fatalf("parseNativeFlowResponseParams(%q) = (%q, %q), want empty strings",
					tt.paramsJSON, buttonID, buttonText)
			}
		})
	}
}
