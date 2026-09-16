// Package walimits queries WhatsApp's account-level messaging limits.
//
// These MEX queries are not part of upstream whatsmeow, so they live here
// instead of in a patched copy of the library. They reach the wire through
// Client.DangerousInternals().SendMexIQ, which keeps the fork-free dependency
// on upstream whatsmeow intact — the library can now be bumped with `go get`.
//
// Both limits are what produce WhatsApp error 463 when messaging a cold
// contact: an active reachout timelock, or an exhausted new-chat quota.
package walimits

import (
	"context"
	"encoding/json"
	"fmt"

	"go.mau.fi/util/jsontime"
	"go.mau.fi/whatsmeow"
)

const (
	queryNewChatMessageCappingInfo = "24503548349331633"
	queryAccountReachoutTimelock   = "23983697327930364"
)

type respGetNewChatMessageCappingInfo struct {
	MessageCappingInfo *NewChatMessageCappingInfo `json:"xwa2_message_capping_info"`
}

type respGetAccountReachoutTimelock struct {
	ReachoutTimelock *AccountReachoutTimelock `json:"xwa2_fetch_account_reachout_timelock"`
}

// GetNewChatMessageCappingInfo returns the account's quota for starting brand-new
// chats in the current cycle. Once it is exhausted, messages to new contacts fail
// with error 463.
func GetNewChatMessageCappingInfo(ctx context.Context, cli *whatsmeow.Client) (*NewChatMessageCappingInfo, error) {
	if cli == nil {
		return nil, fmt.Errorf("nil whatsmeow client")
	}

	data, err := cli.DangerousInternals().SendMexIQ(ctx, queryNewChatMessageCappingInfo, map[string]any{
		"input": map[string]any{
			"type": "INDIVIDUAL_NEW_CHAT_MSG",
		},
	})

	var respData respGetNewChatMessageCappingInfo
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		} else if err == nil && respData.MessageCappingInfo == nil {
			err = fmt.Errorf("mex unexpected null response for new chat message capping info")
		}
	}

	return respData.MessageCappingInfo, err
}

// GetAccountReachoutTimelock reports whether the account is currently barred from
// reaching out to new contacts, and until when.
func GetAccountReachoutTimelock(ctx context.Context, cli *whatsmeow.Client) (*AccountReachoutTimelock, error) {
	if cli == nil {
		return nil, fmt.Errorf("nil whatsmeow client")
	}

	data, err := cli.DangerousInternals().SendMexIQ(ctx, queryAccountReachoutTimelock, map[string]any{})

	var respData respGetAccountReachoutTimelock
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		} else if err == nil && respData.ReachoutTimelock == nil {
			err = fmt.Errorf("mex unexpected null response for fetching reachout timelock")
		}
	}

	return respData.ReachoutTimelock, err
}

// --- types (moved out of the vendored library) ---

type NewChatMessageCappingOTEStatus string

const (
	NewChatMessageCappingOTEStatusNotEligible          NewChatMessageCappingOTEStatus = "NOT_ELIGIBLE"
	NewChatMessageCappingOTEStatusEligible             NewChatMessageCappingOTEStatus = "ELIGIBLE"
	NewChatMessageCappingOTEStatusActiveInCurrentCycle NewChatMessageCappingOTEStatus = "ACTIVE_IN_CURRENT_CYCLE"
	NewChatMessageCappingOTEStatusExhausted            NewChatMessageCappingOTEStatus = "EXHAUSTED"
)

type NewChatMessageCappingMVStatus string

const (
	NewChatMessageCappingMVStatusNotEligible            NewChatMessageCappingMVStatus = "NOT_ELIGIBLE"
	NewChatMessageCappingMVStatusNotActive              NewChatMessageCappingMVStatus = "NOT_ACTIVE"
	NewChatMessageCappingMVStatusActive                 NewChatMessageCappingMVStatus = "ACTIVE"
	NewChatMessageCappingMVStatusActiveUpgradeAvailable NewChatMessageCappingMVStatus = "ACTIVE_UPGRADE_AVAILABLE"
)

type NewChatMessageCappingStatus string

const (
	NewChatMessageCappingStatusNone          NewChatMessageCappingStatus = "NONE"
	NewChatMessageCappingStatusFirstWarning  NewChatMessageCappingStatus = "FIRST_WARNING"
	NewChatMessageCappingStatusSecondWarning NewChatMessageCappingStatus = "SECOND_WARNING"
	NewChatMessageCappingStatusCapped        NewChatMessageCappingStatus = "CAPPED"
)

type ReachoutTimelockEnforcementType string

const (
	ReachoutTimelockEnforcementTypeDefault                                     ReachoutTimelockEnforcementType = "DEFAULT"
	ReachoutTimelockEnforcementTypeBizQuality                                  ReachoutTimelockEnforcementType = "BIZ_QUALITY"
	ReachoutTimelockEnforcementTypeBizCommerceViolationAdult                   ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ADULT"
	ReachoutTimelockEnforcementTypeBizCommerceViolationAlcohol                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ALCOHOL"
	ReachoutTimelockEnforcementTypeBizCommerceViolationAnimals                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_ANIMALS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationBodyPartsFluids         ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_BODY_PARTS_FLUIDS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDating                  ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DATING"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDigitalServicesProducts ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DIGITAL_SERVICES_PRODUCTS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDrugs                   ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DRUGS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationDrugsOnlyOTC            ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_DRUGS_ONLY_OTC"
	ReachoutTimelockEnforcementTypeBizCommerceViolationGambling                ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_GAMBLING"
	ReachoutTimelockEnforcementTypeBizCommerceViolationHealthcare              ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_HEALTHCARE"
	ReachoutTimelockEnforcementTypeBizCommerceViolationRealFakeCurrency        ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_REAL_FAKE_CURRENCY"
	ReachoutTimelockEnforcementTypeBizCommerceViolationSupplements             ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_SUPPLEMENTS"
	ReachoutTimelockEnforcementTypeBizCommerceViolationTobacco                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_TOBACCO"
	ReachoutTimelockEnforcementTypeBizCommerceViolationViolentContent          ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_VIOLENT_CONTENT"
	ReachoutTimelockEnforcementTypeBizCommerceViolationWeapons                 ReachoutTimelockEnforcementType = "BIZ_COMMERCE_VIOLATION_WEAPONS"
	ReachoutTimelockEnforcementTypeWebCompanionOnly                            ReachoutTimelockEnforcementType = "WEB_COMPANION_ONLY"
)

type NewChatMessageCappingInfo struct {
	TotalQuota          int                            `json:"total_quota"`
	UsedQuota           int                            `json:"used_quota"`
	CycleStartTimestamp jsontime.UnixString            `json:"cycle_start_timestamp"`
	CycleEndTimestamp   jsontime.UnixString            `json:"cycle_end_timestamp"`
	ServerSentTimestamp jsontime.UnixString            `json:"server_sent_timestamp"`
	OTEStatus           NewChatMessageCappingOTEStatus `json:"ote_status"`
	MVStatus            NewChatMessageCappingMVStatus  `json:"mv_status"`
	CappingStatus       NewChatMessageCappingStatus    `json:"capping_status"`
}

type AccountReachoutTimelock struct {
	IsActive            bool                            `json:"is_active"`
	TimeEnforcementEnds jsontime.UnixString             `json:"time_enforcement_ends"`
	EnforcementType     ReachoutTimelockEnforcementType `json:"enforcement_type"`
}
