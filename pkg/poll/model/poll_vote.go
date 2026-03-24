package model

import "time"

// PollVote representa um voto em uma enquete do WhatsApp
type PollVote struct {
	ID              string    `json:"id"`
	CompanyID       string    `json:"companyId"`
	InstanceID      string    `json:"instanceId"`
	PollMessageID   string    `json:"pollMessageId"`
	PollChatJid     string    `json:"pollChatJid"`
	VoteMessageID   string    `json:"voteMessageId"`
	VoterJid        string    `json:"voterJid"`
	VoterPhone      string    `json:"voterPhone,omitempty"`
	VoterName       string    `json:"voterName,omitempty"`
	SelectedOptions []string  `json:"selectedOptions"` // SHA-256 hashes
	VotedAt         time.Time `json:"votedAt"`
	ReceivedAt      time.Time `json:"receivedAt"`
}

// PollResults representa os resultados agregados de uma enquete
type PollResults struct {
	PollMessageID string         `json:"pollMessageId"`
	PollChatJid   string         `json:"pollChatJid"`
	TotalVotes    int            `json:"totalVotes"`
	Votes         []PollVote     `json:"votes"`
	OptionCounts  map[string]int `json:"optionCounts"` // hash -> count
	Voters        []VoterInfo    `json:"voters"`
}

// VoterInfo representa informações de um votante
type VoterInfo struct {
	Jid             string    `json:"jid"`
	Phone           string    `json:"phone,omitempty"`
	Name            string    `json:"name,omitempty"`
	SelectedOptions []string  `json:"selectedOptions"`
	VotedAt         time.Time `json:"votedAt"`
}
