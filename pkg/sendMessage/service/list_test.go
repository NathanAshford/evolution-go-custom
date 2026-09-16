package send_service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListBodyText(t *testing.T) {
	tests := []struct {
		name, title, description, want string
	}{
		{"ambos", "Planos", "Escolha um", "*Planos*\n\nEscolha um"},
		{"so titulo", "Planos", "", "*Planos*"},
		{"so descricao", "", "Escolha um", "Escolha um"},
		{"nenhum", "", "", ""},
	}

	for _, tt := range tests {
		if got := listBodyText(tt.title, tt.description); got != tt.want {
			t.Errorf("%s: listBodyText(%q,%q) = %q, want %q",
				tt.name, tt.title, tt.description, got, tt.want)
		}
	}
}

// The single_select native flow carries its sections as JSON in
// ButtonParamsJSON, so the shape of that payload is the contract with the
// WhatsApp client — not an internal detail.
func TestSectionsToStringShape(t *testing.T) {
	data := &ListStruct{
		ButtonText: "Abrir menu",
		Sections: []Section{{
			Title: "Planos",
			Rows: []Row{
				{Title: "Basico", Description: "R$ 49", RowId: "p1"},
				{Title: "Pro", Description: "R$ 99", RowId: "p2"},
			},
		}},
	}

	raw, err := sectionsToString(data)
	if err != nil {
		t.Fatalf("sectionsToString: %v", err)
	}

	var parsed struct {
		Title    string `json:"title"`
		Sections []struct {
			Title string `json:"title"`
			Rows  []struct {
				Header      string `json:"header"`
				Title       string `json:"title"`
				Description string `json:"description"`
				ID          string `json:"id"`
			} `json:"rows"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("payload is not valid JSON: %v\n%s", err, raw)
	}

	if parsed.Title != "Abrir menu" {
		t.Errorf("title = %q, want the button text", parsed.Title)
	}
	if len(parsed.Sections) != 1 || len(parsed.Sections[0].Rows) != 2 {
		t.Fatalf("got %d sections", len(parsed.Sections))
	}

	row := parsed.Sections[0].Rows[0]
	if row.ID != "p1" || row.Title != "Basico" || row.Description != "R$ 49" {
		t.Errorf("row not carried through: %+v", row)
	}
	// header mirrors title — the client uses it for the row's leading label.
	if row.Header != row.Title {
		t.Errorf("header %q should mirror title %q", row.Header, row.Title)
	}
}

// Empty fields must not travel as empty strings: the client drops rows whose
// labels are blank, so the builder substitutes a space.
func TestSectionsToStringFillsBlanks(t *testing.T) {
	data := &ListStruct{
		Sections: []Section{{Rows: []Row{{}}}},
	}

	raw, err := sectionsToString(data)
	if err != nil {
		t.Fatalf("sectionsToString: %v", err)
	}

	if strings.Contains(raw, `"title":""`) || strings.Contains(raw, `"description":""`) {
		t.Errorf("blank labels leaked into the payload: %s", raw)
	}
	if !strings.Contains(raw, `"id":"row_0"`) {
		t.Errorf("missing generated row id: %s", raw)
	}
	if !strings.Contains(raw, `"title":"Ver Menu"`) {
		t.Errorf("missing default button text: %s", raw)
	}
}

func TestSectionsGeneratedIDsAvoidCollisions(t *testing.T) {
	data := &ListStruct{Sections: []Section{
		{Rows: []Row{{}, {RowId: "custom"}}},
		{Rows: []Row{{RowId: "row_0"}, {}, {RowId: "row_2"}, {}}},
	}}
	raw, err := sectionsToString(data)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Sections []struct {
			Rows []struct {
				ID string `json:"id"`
			} `json:"rows"`
		} `json:"sections"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, section := range parsed.Sections {
		for _, row := range section.Rows {
			if row.ID == "" || seen[row.ID] {
				t.Fatalf("duplicate/empty row ID: %q", row.ID)
			}
			seen[row.ID] = true
		}
	}
	for _, id := range []string{"custom", "row_0", "row_2"} {
		if !seen[id] {
			t.Fatalf("explicit ID lost: %s", id)
		}
	}
}

// populatedSections builds n sections that each carry one real row, so an
// over-length fixture trips the COUNT guard rather than the "section without
// rows" guard that a make([]Section, n) fixture would hit first.
func populatedSections(n int) []Section {
	sections := make([]Section, n)
	for i := range sections {
		sections[i] = Section{Title: "s", Rows: []Row{{Title: "r"}}}
	}
	return sections
}

func TestListLimits(t *testing.T) {
	// Each case names the guard it must trip. Asserting on the message keeps a
	// deleted count check from being masked by a different error.
	for _, tt := range []struct {
		name string
		data *ListStruct
		want string
	}{
		{"no sections", &ListStruct{}, "pelo menos uma secao"},
		{"empty section", &ListStruct{Sections: []Section{{}}}, "sem opcoes"},
		{"too many sections", &ListStruct{Sections: populatedSections(maxListSections + 1)}, "no maximo 10 secoes"},
		{"too many rows", &ListStruct{Sections: []Section{{Rows: make([]Row, maxListRows+1)}}}, "no maximo 10 opcoes"},
		{"rows split across sections exceed the total", &ListStruct{Sections: populatedSections(maxListRows + 1)}, "no maximo"},
		{"second section empty", &ListStruct{Sections: []Section{{Rows: []Row{{}}}, {}}}, "sem opcoes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildListMessage(tt.data)
			if err == nil {
				t.Fatalf("invalid list accepted: %+v", tt.data)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("wrong guard tripped: got %q, want it to mention %q", err, tt.want)
			}
		})
	}
	// Exactly at the limit must still be accepted, in both shapes.
	if _, err := buildListMessage(&ListStruct{Sections: []Section{{Rows: make([]Row, maxListRows)}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := buildListMessage(&ListStruct{Sections: populatedSections(maxListSections)}); err != nil {
		t.Fatal(err)
	}
}

// TestListIsSingleSelectNativeFlow locks the format the diff switched to: the
// list must be a single_select native flow, not the legacy ListMessage.
func TestListIsSingleSelectNativeFlow(t *testing.T) {
	msg, err := buildListMessage(&ListStruct{
		Title: "T", Description: "D", FooterText: "F", ButtonText: "Abrir",
		Sections: []Section{{Title: "Sec", Rows: []Row{{Title: "Linha", RowId: "r1"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg.GetListMessage() != nil {
		t.Fatal("legacy ListMessage reintroduced")
	}
	flow := msg.GetInteractiveMessage().GetNativeFlowMessage()
	if len(flow.GetButtons()) != 1 {
		t.Fatalf("expected exactly one native flow button, got %d", len(flow.GetButtons()))
	}
	if name := flow.GetButtons()[0].GetName(); name != "single_select" {
		t.Fatalf("native flow name = %q, want single_select", name)
	}
	if v := flow.GetMessageVersion(); v != 1 {
		t.Fatalf("MessageVersion = %d, want 1", v)
	}
	params := flow.GetButtons()[0].GetButtonParamsJSON()
	for _, want := range []string{`"title":"Abrir"`, `"Sec"`, `"Linha"`, `"r1"`} {
		if !strings.Contains(params, want) {
			t.Fatalf("button params missing %s: %s", want, params)
		}
	}
}
