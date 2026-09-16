package mcp

import (
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

func user(number string) types.JID {
	return types.JID{User: number, Server: types.DefaultUserServer}
}

func TestFoldIgnoresCaseAndAccents(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Cibele Falcão", "cibele falcao"},
		{"FALCÃO", "falcao"},
		{"  José  ", "jose"},
		{"Ana Júlia Conceição", "ana julia conceicao"},
		{"", ""},
	}

	for _, tt := range tests {
		if got := fold(tt.in); got != tt.want {
			t.Errorf("fold(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// The tiers are the contract: an exact match must always beat a prefix, and a
// prefix must always beat a loose token match. Ambiguity detection depends on
// equal scores meaning "genuinely equally good", so the ordering must hold.
func TestScoreNameTiersAreOrdered(t *testing.T) {
	exact := scoreName("Cibele Falcão", "cibele falcao")
	prefix := scoreName("Cibele Falcão", "cibele")
	word := scoreName("Cibele Falcão", "falcao")
	substring := scoreName("Maria Cibelezinha", "ibele")
	tokens := scoreName("Cibele Maria Falcão", "cibele falcao")

	if exact != 100 {
		t.Errorf("exact match = %d, want 100", exact)
	}
	if !(exact > prefix && prefix > word && word > substring) {
		t.Errorf("tiers out of order: exact=%d prefix=%d word=%d substring=%d",
			exact, prefix, word, substring)
	}
	if tokens == 0 {
		t.Error("out-of-order tokens should still match")
	}
	if got := scoreName("Cibele Falcão", "roberto"); got != 0 {
		t.Errorf("unrelated query scored %d, want 0", got)
	}
	if got := scoreName("", "cibele"); got != 0 {
		t.Errorf("empty name scored %d, want 0", got)
	}
}

// The motivating case: "envie oi para Cibele Falcão" must resolve to exactly
// one number, even though another contact shares the first name.
func TestRankCandidatesResolvesFullName(t *testing.T) {
	candidates := []candidate{
		{jid: user("5511111111111"), kind: "contact", savedName: "Cibele Falcão"},
		{jid: user("5522222222222"), kind: "contact", savedName: "Cibele Rodrigues"},
		{jid: user("5533333333333"), kind: "contact", savedName: "Roberto Silva"},
	}

	matches := rankCandidates(candidates, "Cibele Falcão", 20)
	if len(matches) == 0 {
		t.Fatal("expected at least one match")
	}
	if matches[0].Name != "Cibele Falcão" {
		t.Errorf("top match = %q, want %q", matches[0].Name, "Cibele Falcão")
	}
	if matches[0].Number != "5511111111111" {
		t.Errorf("top number = %q, want 5511111111111", matches[0].Number)
	}
	if len(matches) > 1 && matches[0].Score == matches[1].Score {
		t.Error("full name should be an unambiguous winner")
	}

	// Accent-blind: the unaccented spelling must find the accented contact.
	if got := rankCandidates(candidates, "falcao", 20); len(got) == 0 || got[0].Name != "Cibele Falcão" {
		t.Errorf("unaccented query failed to find the accented contact: %+v", got)
	}
}

// A bare first name shared by two people must come back as a tie, which is what
// makes toolFindContact report ambiguity instead of picking a recipient.
func TestRankCandidatesReportsTieForSharedFirstName(t *testing.T) {
	candidates := []candidate{
		{jid: user("5511111111111"), kind: "contact", savedName: "Cibele Falcão"},
		{jid: user("5522222222222"), kind: "contact", savedName: "Cibele Rodrigues"},
	}

	matches := rankCandidates(candidates, "Cibele", 20)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if matches[0].Score != matches[1].Score {
		t.Errorf("shared first name should tie: %d vs %d", matches[0].Score, matches[1].Score)
	}
}

func TestRankCandidatesMatchesNumberAndFallsBackToPushName(t *testing.T) {
	candidates := []candidate{
		{jid: user("5511999998888"), kind: "contact", pushName: "Zé da Padaria"},
		{jid: user("5511777776666"), kind: "contact", savedName: "Outro"},
	}

	byNumber := rankCandidates(candidates, "5511999998888", 20)
	if len(byNumber) == 0 || byNumber[0].JID != "5511999998888@s.whatsapp.net" {
		t.Fatalf("number lookup failed: %+v", byNumber)
	}
	if byNumber[0].MatchedOn != "number" {
		t.Errorf("matchedOn = %q, want number", byNumber[0].MatchedOn)
	}
	// With no saved name, the push name is what the operator sees.
	if byNumber[0].Name != "Zé da Padaria" {
		t.Errorf("display name = %q, want the push name", byNumber[0].Name)
	}

	if got := rankCandidates(candidates, "padaria", 20); len(got) == 0 || got[0].MatchedOn != "pushName" {
		t.Errorf("push name search failed: %+v", got)
	}
}

// LID contacts hide the phone number; the JID must still come back so the model
// has something addressable rather than an empty destination.
func TestRankCandidatesLidContactHasJIDButNoNumber(t *testing.T) {
	candidates := []candidate{
		{jid: types.JID{User: "123456789", Server: types.HiddenUserServer}, kind: "contact", savedName: "Cibele Falcão"},
	}

	matches := rankCandidates(candidates, "Cibele", 20)
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1", len(matches))
	}
	if matches[0].Number != "" {
		t.Errorf("LID contact returned a number %q, want empty", matches[0].Number)
	}
	if matches[0].JID == "" {
		t.Error("LID contact must still expose its JID")
	}
}

func TestRankCandidatesEmptyQueryListsEverythingUpToLimit(t *testing.T) {
	candidates := []candidate{
		{jid: user("551111"), savedName: "A"},
		{jid: user("552222"), savedName: "B"},
		{jid: user("553333"), savedName: "C"},
	}

	if got := rankCandidates(candidates, "", 20); len(got) != 3 {
		t.Errorf("empty query returned %d, want 3", len(got))
	}
	if got := rankCandidates(candidates, "", 2); len(got) != 2 {
		t.Errorf("limit not applied: got %d, want 2", len(got))
	}
}

// A tool that is declared but never dispatched answers "unknown tool" at
// runtime, which only shows up once a model tries to call it. callTool reaches
// resolveInstance before touching any service, so an empty session surfaces the
// routing without needing the whole dependency graph.
func TestNewToolsAreDeclaredAndDispatched(t *testing.T) {
	s := &Server{}

	declared := map[string]bool{}
	for _, tool := range s.toolDefinitions() {
		if declared[tool.Name] {
			t.Errorf("tool %q is declared twice", tool.Name)
		}
		declared[tool.Name] = true
	}

	for _, name := range []string{"find_contact", "delete_message"} {
		if !declared[name] {
			t.Errorf("tool %q is missing from toolDefinitions", name)
		}

		res := s.callTool(&session{}, callToolParams{Name: name})
		if res == nil {
			t.Fatalf("callTool(%q) returned nil", name)
		}
		if len(res.Content) > 0 && strings.Contains(res.Content[0].Text, "unknown tool") {
			t.Errorf("tool %q is declared but not wired into callTool", name)
		}
	}
}

// Found against a real address book: one person shows up twice, as a phone
// contact and as the @lid alias for the same user. That is one contact, not
// two, and must not trip the ambiguity guard.
func TestMergeIdentitiesCollapsesLidAliasOfSamePerson(t *testing.T) {
	candidates := []candidate{
		{jid: user("5511999990031"), kind: "contact", savedName: "Cibele Falcão"},
		{jid: types.JID{User: "77776666", Server: types.HiddenUserServer}, kind: "contact", pushName: "Cibele Falcão"},
	}

	matches := rankCandidates(candidates, "Cibele", 20)
	if len(matches) != 1 {
		t.Fatalf("got %d matches, want 1 (the @lid alias should merge)", len(matches))
	}
	if matches[0].Number != "5511999990031" {
		t.Errorf("kept the unreachable alias: number = %q", matches[0].Number)
	}
	// The alias' push name should survive on the merged entry.
	if matches[0].PushName != "Cibele Falcão" {
		t.Errorf("push name lost in the merge: %q", matches[0].PushName)
	}
}

// Two genuinely different people who share a name both have numbers, so there
// is nothing to collapse — this must stay ambiguous.
func TestMergeIdentitiesKeepsDistinctPeopleWithSameName(t *testing.T) {
	candidates := []candidate{
		{jid: user("5511111111111"), kind: "contact", savedName: "João Silva"},
		{jid: user("5522222222222"), kind: "contact", savedName: "João Silva"},
	}

	matches := rankCandidates(candidates, "João Silva", 20)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2 — distinct people must not merge", len(matches))
	}
	if matches[0].Score != matches[1].Score {
		t.Error("two distinct contacts with the same name should tie")
	}
}

// Nothing reachable to merge into: keep every alias rather than dropping the
// contact entirely.
func TestMergeIdentitiesKeepsLidOnlyContacts(t *testing.T) {
	candidates := []candidate{
		{jid: types.JID{User: "111", Server: types.HiddenUserServer}, kind: "contact", pushName: "Alguém"},
		{jid: types.JID{User: "222", Server: types.HiddenUserServer}, kind: "contact", pushName: "Alguém"},
	}

	if got := rankCandidates(candidates, "Alguém", 20); len(got) != 2 {
		t.Errorf("got %d matches, want 2", len(got))
	}
}

// A group must never absorb a person who happens to share its name.
func TestMergeIdentitiesDoesNotMixGroupsWithContacts(t *testing.T) {
	candidates := []candidate{
		{jid: user("5511111111111"), kind: "contact", savedName: "Família"},
		{jid: types.JID{User: "120363", Server: types.GroupServer}, kind: "group", savedName: "Família"},
	}

	matches := rankCandidates(candidates, "Família", 20)
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2 (contact and group are different things)", len(matches))
	}
}

func TestMaskNumberHidesAllButLastFour(t *testing.T) {
	cases := map[string]string{
		"5511999990031": "••••0031",
		"1234":          "••••",
		"12":            "••••",
		"":              "",
	}
	for in, want := range cases {
		if got := maskNumber(in); got != want {
			t.Errorf("maskNumber(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRankCandidatesFindsGroupByName(t *testing.T) {
	candidates := []candidate{
		{jid: types.JID{User: "120363", Server: types.GroupServer}, kind: "group", savedName: "Família Falcão"},
		{jid: user("5511111111111"), kind: "contact", savedName: "Roberto"},
	}

	matches := rankCandidates(candidates, "familia falcao", 20)
	if len(matches) == 0 {
		t.Fatal("group not found")
	}
	if matches[0].Kind != "group" {
		t.Errorf("kind = %q, want group", matches[0].Kind)
	}
	if matches[0].Number != "" {
		t.Errorf("group returned a number %q, want empty (send to the JID)", matches[0].Number)
	}
}
