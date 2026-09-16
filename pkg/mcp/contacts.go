package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"go.mau.fi/whatsmeow/types"
	"golang.org/x/text/unicode/norm"

	send_service "github.com/EvolutionAPI/evolution-go/pkg/sendMessage/service"
)

// groupFetchTimeout bounds the only network call in this file. Saved contacts
// live in the local store and are always available, but the joined-group list
// needs a round trip to WhatsApp, and a slow one must not hang the tool.
const groupFetchTimeout = 8 * time.Second

// contactMatch is one addressable destination — a saved contact or a group the
// instance belongs to — plus enough context for the model to tell near-
// homonyms apart before it sends anything.
type contactMatch struct {
	Name         string `json:"name"`
	Number       string `json:"number,omitempty"`
	JID          string `json:"jid"`
	Kind         string `json:"kind"`
	MatchedOn    string `json:"matchedOn,omitempty"`
	Score        int    `json:"score"`
	SavedName    string `json:"savedName,omitempty"`
	PushName     string `json:"pushName,omitempty"`
	BusinessName string `json:"businessName,omitempty"`
}

// candidate is a match target before scoring. One JID carries several names —
// the one saved in the phone's address book, the one its owner chose, the
// verified business name — and a person may be looked up by any of them.
type candidate struct {
	jid          types.JID
	kind         string
	savedName    string
	firstName    string
	pushName     string
	businessName string
}

// displayName picks the name a human would recognise, preferring the one the
// operator saved over the one the other party chose for themselves.
func (c candidate) displayName() string {
	for _, name := range []string{c.savedName, c.businessName, c.pushName, c.firstName} {
		if strings.TrimSpace(name) != "" {
			return name
		}
	}
	return c.jid.User
}

// sendableNumber returns the bare phone number when there is one. LID contacts
// (@lid) hide the real number, so only their JID can be used as a destination.
func (c candidate) sendableNumber() string {
	if c.jid.Server == types.DefaultUserServer {
		return c.jid.User
	}
	return ""
}

// fold normalises a name for comparison: case-insensitive and accent-blind, so
// "Falcao" finds "Falcão" and vice versa.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(strings.TrimSpace(s))) {
		if unicode.Is(unicode.Mn, r) { // Mn = combining accent left over by NFD
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// scoreName rates how well one name answers the query. The tiers matter more
// than the exact numbers: an exact match must outrank a prefix and a prefix
// must outrank a loose token match, so that a genuine tie stays a tie and the
// caller can report ambiguity instead of picking a recipient by luck.
func scoreName(name, query string) int {
	n, q := fold(name), fold(query)
	if n == "" || q == "" {
		return 0
	}

	if n == q {
		return 100
	}
	if strings.HasPrefix(n, q) {
		return 85
	}

	// "falcao" should still find "Cibele Falcão": match the start of any word.
	for _, word := range strings.Fields(n) {
		if strings.HasPrefix(word, q) {
			return 75
		}
	}

	if strings.Contains(n, q) {
		return 65
	}

	// Every token present in any order — "cibele silva" finds "Cibele X Silva".
	tokens := strings.Fields(q)
	if len(tokens) < 2 {
		return 0
	}
	for _, t := range tokens {
		if !strings.Contains(n, t) {
			return 0
		}
	}
	return 55
}

// collectCandidates gathers everything the instance can address by name.
func (s *Server) collectCandidates(instanceID string, includeGroups bool) ([]candidate, error) {
	client := s.whatsmeowService.GetClient(instanceID)
	if client == nil || client.Store == nil || client.Store.Contacts == nil {
		return nil, fmt.Errorf("this instance has no WhatsApp session yet, so its address book was never synced; connect it once and try again")
	}

	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to read the address book: %w", err)
	}

	out := make([]candidate, 0, len(contacts)+16)
	for jid, info := range contacts {
		out = append(out, candidate{
			jid:          jid,
			kind:         "contact",
			savedName:    info.FullName,
			firstName:    info.FirstName,
			pushName:     info.PushName,
			businessName: info.BusinessName,
		})
	}

	// Groups are a bonus: if the fetch fails or times out, contacts alone still
	// answer most queries, so a failure here is deliberately not fatal.
	if includeGroups && client.IsLoggedIn() {
		ctx, cancel := context.WithTimeout(context.Background(), groupFetchTimeout)
		defer cancel()

		if groups, err := client.GetJoinedGroups(ctx); err == nil {
			for _, g := range groups {
				out = append(out, candidate{jid: g.JID, kind: "group", savedName: g.Name})
			}
		}
	}

	return out, nil
}

// mergeIdentities collapses one person appearing twice: once as a phone contact
// and once under the @lid alias WhatsApp uses for the same user. Real address
// books are full of these, and without merging them a single contact looks like
// two people and trips the ambiguity guard.
//
// Two entries only merge when they share a display name AND exactly one side
// has a reachable number — two different people who happen to share a name both
// have numbers, and must stay ambiguous.
func mergeIdentities(candidates []candidate) []candidate {
	groups := make(map[string][]candidate, len(candidates))
	order := make([]string, 0, len(candidates))
	unnamed := make([]candidate, 0)

	for _, c := range candidates {
		key := c.kind + "\x00" + fold(c.displayName())
		if fold(c.displayName()) == "" {
			unnamed = append(unnamed, c)
			continue
		}
		if _, seen := groups[key]; !seen {
			order = append(order, key)
		}
		groups[key] = append(groups[key], c)
	}

	out := make([]candidate, 0, len(candidates))
	for _, key := range order {
		group := groups[key]
		if len(group) == 1 {
			out = append(out, group[0])
			continue
		}

		numbered := make([]candidate, 0, len(group))
		for _, c := range group {
			if c.sendableNumber() != "" {
				numbered = append(numbered, c)
			}
		}

		// Nobody reachable by number (all @lid), or several distinct people:
		// either way there is nothing safe to collapse.
		if len(numbered) == 0 {
			out = append(out, group...)
			continue
		}

		// Fold the alias-only entries' names into the reachable ones, so a name
		// known only from the @lid side is still searchable.
		for i := range numbered {
			for _, c := range group {
				if c.sendableNumber() != "" {
					continue
				}
				if numbered[i].pushName == "" {
					numbered[i].pushName = c.pushName
				}
				if numbered[i].businessName == "" {
					numbered[i].businessName = c.businessName
				}
			}
		}
		out = append(out, numbered...)
	}

	return append(out, unnamed...)
}

// rankCandidates scores, sorts and truncates. An empty query lists everything.
func rankCandidates(candidates []candidate, query string, limit int) []contactMatch {
	digits := digitsOnly(query)
	candidates = mergeIdentities(candidates)
	matches := make([]contactMatch, 0, limit)

	for _, c := range candidates {
		best, field := 0, ""

		for _, f := range []struct{ label, value string }{
			{"savedName", c.savedName},
			{"businessName", c.businessName},
			{"pushName", c.pushName},
			{"firstName", c.firstName},
		} {
			if sc := scoreName(f.value, query); sc > best {
				best, field = sc, f.label
			}
		}

		// A numeric query addresses the number itself, not a name.
		if len(digits) >= 4 && strings.Contains(c.jid.User, digits) {
			sc := 70
			if c.jid.User == digits {
				sc = 100
			}
			if sc > best {
				best, field = sc, "number"
			}
		}

		if query == "" {
			best, field = 1, ""
		}
		if best == 0 {
			continue
		}

		matches = append(matches, contactMatch{
			Name:         c.displayName(),
			Number:       c.sendableNumber(),
			JID:          c.jid.String(),
			Kind:         c.kind,
			MatchedOn:    field,
			Score:        best,
			SavedName:    c.savedName,
			PushName:     c.pushName,
			BusinessName: c.businessName,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return fold(matches[i].Name) < fold(matches[j].Name)
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}
	return matches
}

func (s *Server) toolFindContact(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	limit := argInt(args, "limit", 20)
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}

	includeGroups := true
	if p := argBoolPtr(args, "include_groups"); p != nil {
		includeGroups = *p
	}

	candidates, err := s.collectCandidates(instance.Id, includeGroups)
	if err != nil {
		return errorResult(err.Error())
	}

	query := argString(args, "query")
	matches := rankCandidates(candidates, query, limit)

	if len(matches) == 0 {
		return jsonResult(map[string]any{
			"instance":      instance.Name,
			"query":         query,
			"matches":       []contactMatch{},
			"searchedNames": len(candidates),
			"noMatchAdvice": "No saved contact or group matched. Try a shorter query (a first name only), or ask the user for the phone number.",
		})
	}

	// Two equally good hits mean the name alone does not identify a person.
	// Say so, so the model asks instead of messaging the wrong Cibele.
	ambiguous := len(matches) > 1 && matches[0].Score == matches[1].Score

	out := map[string]any{
		"instance":      instance.Name,
		"query":         query,
		"matches":       matches,
		"searchedNames": len(candidates),
		"ambiguous":     ambiguous,
	}
	if ambiguous {
		out["ambiguityAdvice"] = "Several contacts match this name equally well. Ask the user which one before sending anything."
	}

	return jsonResult(out)
}

// maskNumber keeps only the last four digits, so a result can say who was
// messaged without handing a phone number back to the model.
func maskNumber(number string) string {
	if number == "" {
		return ""
	}
	if len(number) <= 4 {
		return "••••"
	}
	return "••••" + number[len(number)-4:]
}

// toolSendToContact resolves a name and sends in one step, so the phone number
// never leaves the server. Some MCP clients refuse to run a tool that returns
// personal data like a phone number; this keeps the whole lookup server-side.
//
// It refuses to guess. A tie between two contacts comes back as an error
// listing masked options, because picking one silently would message a real
// person by luck.
func (s *Server) toolSendToContact(sess *session, args map[string]any) *callToolResult {
	instance, err := s.resolveInstance(sess, args)
	if err != nil {
		return errorResult(err.Error())
	}

	name := argString(args, "name")
	text := argString(args, "text")
	hint := digitsOnly(argString(args, "number_hint"))

	if name == "" {
		return errorResult("the 'name' argument is required")
	}
	if text == "" {
		return errorResult("the 'text' argument is required")
	}

	candidates, err := s.collectCandidates(instance.Id, true)
	if err != nil {
		return errorResult(err.Error())
	}

	matches := rankCandidates(candidates, name, 10)

	if hint != "" {
		filtered := make([]contactMatch, 0, len(matches))
		for _, m := range matches {
			if strings.HasSuffix(m.Number, hint) {
				filtered = append(filtered, m)
			}
		}
		matches = filtered
	}

	if len(matches) == 0 {
		return errorResult(fmt.Sprintf(
			"no saved contact or group matches %q on instance %q. Try a shorter name, or ask the user for the phone number and use send_text_message.",
			name, instance.Name,
		))
	}

	if len(matches) > 1 && matches[0].Score == matches[1].Score {
		var b strings.Builder
		fmt.Fprintf(&b, "%q matches more than one contact, so nothing was sent. Ask the user which one they meant, then call again with the full name (or number_hint with the last digits):\n", name)
		for _, m := range matches {
			if m.Score != matches[0].Score {
				break
			}
			fmt.Fprintf(&b, "  - %s", m.Name)
			if masked := maskNumber(m.Number); masked != "" {
				fmt.Fprintf(&b, " (%s)", masked)
			}
			if m.Kind == "group" {
				b.WriteString(" [group]")
			}
			b.WriteString("\n")
		}
		return errorResult(b.String())
	}

	winner := matches[0]

	// Groups and @lid contacts have no phone number; their JID is the address.
	destination := winner.Number
	if destination == "" {
		destination = winner.JID
	}
	if destination == "" {
		return errorResult(fmt.Sprintf("%q has no address that can be messaged", winner.Name))
	}

	if failure := requireConnected(instance); failure != nil {
		return failure
	}

	data := &send_service.TextStruct{
		Number: destination,
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

	// Deliberately no phone number in the response.
	return jsonResult(map[string]any{
		"status":     "sent",
		"to":         winner.Name,
		"toMasked":   maskNumber(winner.Number),
		"kind":       winner.Kind,
		"matchedOn":  winner.MatchedOn,
		"confidence": winner.Score,
		"messageId":  sent.Info.ID,
		"timestamp":  sent.Info.Timestamp.Format(time.RFC3339),
		"instance":   instance.Name,
	})
}

// contactNameIndex maps a JID user part to the best known display name. It is
// best-effort by design: list_chats must keep working for an instance whose
// session is momentarily gone.
func (s *Server) contactNameIndex(instanceID string) map[string]string {
	client := s.whatsmeowService.GetClient(instanceID)
	if client == nil || client.Store == nil || client.Store.Contacts == nil {
		return nil
	}

	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		return nil
	}

	index := make(map[string]string, len(contacts))
	for jid, info := range contacts {
		c := candidate{
			jid:          jid,
			savedName:    info.FullName,
			firstName:    info.FirstName,
			pushName:     info.PushName,
			businessName: info.BusinessName,
		}
		if name := c.displayName(); name != "" && name != jid.User {
			index[jid.User] = name
		}
	}
	return index
}
