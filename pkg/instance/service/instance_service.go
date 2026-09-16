package instance_service

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/EvolutionAPI/evolution-go/pkg/config"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
	event_types "github.com/EvolutionAPI/evolution-go/pkg/internal/event_types"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
	"github.com/EvolutionAPI/evolution-go/pkg/walimits"
	whatsmeow_registry "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/registry"
	whatsmeow_service "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/service"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

type InstanceService interface {
	Create(data *CreateStruct) (*instance_model.Instance, error)
	Connect(data *ConnectStruct, instance *instance_model.Instance) (*instance_model.Instance, string, string, error)
	Reconnect(instance *instance_model.Instance) error
	Disconnect(instance *instance_model.Instance) (*instance_model.Instance, error)
	Logout(instance *instance_model.Instance) (*instance_model.Instance, error)
	Status(instance *instance_model.Instance) (*StatusStruct, error)
	GetLimits(instanceId string) (*LimitsStruct, error)
	GetQr(instance *instance_model.Instance) (*QrcodeStruct, error)
	Pair(data *PairStruct, instance *instance_model.Instance) (*PairReturnStruct, error)
	GetAll() ([]*instance_model.Instance, error)
	Info(instanceId string) (*instance_model.Instance, error)
	Delete(id string) error
	SetProxy(id string, proxyConfig *ProxyConfig) error
	SetProxyFromStruct(id string, data *SetProxyStruct) error
	GetProxy(id string) (*ProxyConfig, error)
	TestProxy(cfg *ProxyConfig) (*ProxyTestResult, error)
	ReconnectProxy(id string) error
	RemoveProxy(id string) error
	ForceReconnect(instanceId string, number string) error
	GetInstanceByToken(token string) (*instance_model.Instance, error)
	GetLogs(instanceId string, startDate, endDate time.Time, level string, limit int) ([]logger_wrapper.LogEntry, error)
	GetAdvancedSettings(instanceId string) (*instance_model.AdvancedSettings, error)
	UpdateAdvancedSettings(instanceId string, settings *instance_model.AdvancedSettings) error
	AddWebhook(instanceId string, url string) (*instance_model.Instance, error)
	RemoveWebhook(instanceId string, url string) (*instance_model.Instance, error)
	ListWebhooks(instanceId string) ([]string, error)
}

type instances struct {
	instanceRepository instance_repository.InstanceRepository
	config             *config.Config
	clients            *whatsmeow_registry.Clients
	whatsmeowService   whatsmeow_service.WhatsmeowService
	loggerWrapper      *logger_wrapper.LoggerManager
}

type ProxyConfig struct {
	Protocol string `json:"protocol,omitempty"`
	Port     string `json:"port"`
	Password string `json:"password"`
	Username string `json:"username"`
	Host     string `json:"host"`
}

type CreateStruct struct {
	InstanceId       string                           `json:"instanceId"`
	Name             string                           `json:"name"`
	Token            string                           `json:"token"`
	Proxy            *ProxyConfig                     `json:"proxy"`
	AdvancedSettings *instance_model.AdvancedSettings `json:"advancedSettings"`
}

type ConnectStruct struct {
	WebhookUrl      string   `json:"webhookUrl"`
	Subscribe       []string `json:"subscribe"`
	Immediate       bool     `json:"immediate"`
	Phone           string   `json:"phone"`
	RabbitmqEnable  string   `json:"rabbitmqEnable"`
	WebSocketEnable string   `json:"websocketEnable"`
	NatsEnable      string   `json:"natsEnable"`
}

type StatusStruct struct {
	Connected bool
	LoggedIn  bool
	myJid     *types.JID
	Name      string
}

// ReachoutTimelockStruct describes WhatsApp's reachout timelock for the account
// (when active, companion devices/the API cannot start chats with NEW contacts → error 463).
type ReachoutTimelockStruct struct {
	IsActive            bool   `json:"isActive"`
	TimeEnforcementEnds int64  `json:"timeEnforcementEnds"` // unix seconds; 0 when not active
	EnforcementType     string `json:"enforcementType"`
}

// NewChatCappingStruct describes the account's new-chat messaging quota for the current cycle.
type NewChatCappingStruct struct {
	CappingStatus string `json:"cappingStatus"`
	TotalQuota    int    `json:"totalQuota"`
	UsedQuota     int    `json:"usedQuota"`
	CycleEnds     int64  `json:"cycleEnds"` // unix seconds
}

// LimitsStruct aggregates WhatsApp's account-level messaging limits for an instance.
type LimitsStruct struct {
	ReachoutTimelock *ReachoutTimelockStruct `json:"reachoutTimelock"`
	NewChatCapping   *NewChatCappingStruct   `json:"newChatCapping"`
}

type QrcodeStruct struct {
	Qrcode string
	Code   string
	// Passkey ceremony fields. Populated when the account requires a WebAuthn
	// passkey to finish linking (no QR to scan at that point). The manager uses
	// PasskeyStage to switch its UI and PasskeyOpenURL for the
	// "Abrir WhatsApp Web" button that launches the passkey ceremony.
	PasskeyStage   string `json:"passkeyStage,omitempty"`
	PasskeyOpenURL string `json:"passkeyOpenUrl,omitempty"`
	PasskeyCode    string `json:"passkeyCode,omitempty"`
}

type PairStruct struct {
	Subscribe []string `json:"subscribe"`
	Phone     string   `json:"phone"`
}

type PairReturnStruct struct {
	PairingCode string
}

type SetProxyStruct struct {
	Protocol string `json:"protocol,omitempty"`
	Host     string `json:"host" validate:"required"`
	Port     string `json:"port" validate:"required"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type ForceReconnectStruct struct {
	Number string `json:"number"`
}

// How long ensureClientConnected waits for a freshly started client to finish
// connecting. Polling rather than sleeping a flat interval: the handler returns
// as soon as the client is up, and a slow connect is no longer cut off early.
const clientWaitTimeout = 8 * time.Second

// waitForConnectedClient polls the registry until the instance has a connected
// client, returning it, or the last value seen (possibly nil) on timeout.
func (i *instances) waitForConnectedClient(instanceId string, timeout time.Duration) *whatsmeow.Client {
	deadline := time.Now().Add(timeout)

	for {
		client := i.clients.Get(instanceId)
		if client != nil && client.IsConnected() {
			return client
		}

		if time.Now().After(deadline) {
			return client
		}

		time.Sleep(qrPollInterval)
	}
}

func (i *instances) ensureClientConnected(instanceId string) (*whatsmeow.Client, error) {
	logger := i.loggerWrapper.GetLogger(instanceId)
	client := i.clients.Get(instanceId)
	logger.LogInfo("[%s] Checking client connection status - Client exists: %v", instanceId, client != nil)

	if client == nil {
		logger.LogInfo("[%s] No client found, attempting to start new instance", instanceId)
		err := i.whatsmeowService.StartInstance(instanceId)
		if err != nil {
			logger.LogError("[%s] Failed to start instance: %v", instanceId, err)
			return nil, errors.New("no active session found")
		}

		logger.LogInfo("[%s] Instance started, waiting for the client to connect...", instanceId)
		client = i.waitForConnectedClient(instanceId, clientWaitTimeout)

		logger.LogInfo("[%s] Checking new client - Exists: %v, Connected: %v",
			instanceId,
			client != nil,
			client != nil && client.IsConnected())

		if client == nil || !client.IsConnected() {
			logger.LogError("[%s] New client validation failed - Exists: %v, Connected: %v",
				instanceId,
				client != nil,
				client != nil && client.IsConnected())
			return nil, errors.New("no active session found")
		}
	} else if !client.IsConnected() {
		logger.LogError("[%s] Existing client is disconnected - Connected status: %v",
			instanceId,
			client.IsConnected())
		return nil, errors.New("client disconnected")
	}

	logger.LogInfo("[%s] Client successfully validated - Connected: %v", instanceId, client.IsConnected())
	return client, nil
}

func (i *instances) Create(data *CreateStruct) (*instance_model.Instance, error) {
	if data.Proxy != nil {
		data.Proxy.Protocol = utils.NormalizeProxyProtocol(data.Proxy.Protocol, data.Proxy.Port)
	}

	proxyJson, err := json.Marshal(data.Proxy)
	if err != nil {
		return nil, err
	}

	findInstance, _ := i.instanceRepository.GetInstanceByName(data.Name)

	if findInstance != nil {
		return nil, fmt.Errorf("instance already exists")
	}

	instance := instance_model.Instance{
		Id:         data.InstanceId,
		Name:       data.Name,
		Token:      data.Token,
		OsName:     i.config.OsName,
		Proxy:      string(proxyJson),
		Connected:  false,
		ClientName: i.config.ClientName,
	}

	// Set advanced settings if provided
	if data.AdvancedSettings != nil {
		instance.AlwaysOnline = data.AdvancedSettings.AlwaysOnline
		instance.RejectCall = data.AdvancedSettings.RejectCall
		instance.MsgRejectCall = data.AdvancedSettings.MsgRejectCall
		instance.ReadMessages = data.AdvancedSettings.ReadMessages
		instance.IgnoreGroups = data.AdvancedSettings.IgnoreGroups
		instance.IgnoreStatus = data.AdvancedSettings.IgnoreStatus
	}

	createdInstance, err := i.instanceRepository.Create(instance)
	if err != nil {
		return nil, err
	}

	return createdInstance, nil
}

func (i *instances) Connect(data *ConnectStruct, instance *instance_model.Instance) (*instance_model.Instance, string, string, error) {
	var subscribedEvents []string

	i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Processing subscribe events: %v", instance.Id, data.Subscribe)

	if len(data.Subscribe) == 0 {
		subscribedEvents = append(subscribedEvents, event_types.MESSAGE)
	} else if len(data.Subscribe) > 0 && data.Subscribe[0] == "ALL" {
		for _, event := range event_types.AllEventTypes {
			subscribedEvents = append(subscribedEvents, event)
		}
	} else {
		for _, arg := range data.Subscribe {
			if !event_types.IsEventType(arg) {
				i.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Message type discarded '%s'", instance.Id, arg)
				continue
			}
			subscribedEvents = append(subscribedEvents, arg)
		}
	}

	eventString := strings.Join(subscribedEvents, ",")

	instance.Events = eventString
	instance.Webhook = data.WebhookUrl
	instance.RabbitmqEnable = data.RabbitmqEnable
	instance.NatsEnable = data.NatsEnable
	instance.WebSocketEnable = data.WebSocketEnable

	err := i.instanceRepository.Update(instance)
	if err != nil {
		i.loggerWrapper.GetLogger(instance.Id).LogError("[%s] Error updating instance: %s", instance.Id, err)
		return nil, "", "", err
	}

	// Verifica se a instância já está rodando
	isInstanceRunning := i.clients.Get(instance.Id) != nil

	// Sincroniza as configurações na instância em execução (se já estiver conectada)
	err = i.whatsmeowService.UpdateInstanceSettings(instance.Id)
	if err != nil {
		i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Instance not in runtime yet, will be updated when connected", instance.Id)
		isInstanceRunning = false
	} else {
		i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Instance settings updated successfully in runtime", instance.Id)
		isInstanceRunning = true
	}

	// Se a instância não estiver rodando, inicia uma nova
	if !isInstanceRunning {
		i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Starting new client instance", instance.Id)

		clientData := &whatsmeow_service.ClientData{
			Instance:      instance,
			Subscriptions: subscribedEvents,
			Phone:         data.Phone,
			IsProxy:       false,
		}

		// Only parse when there is something to parse: with a global proxy set and
		// no per-instance one, instance.Proxy is "" and Unmarshal would fail on
		// empty input, aborting a connection that should simply use the global
		// proxy. Legacy rows also store the literal "null".
		if instance.Proxy != "" || i.config.ProxyHost != "" {
			var proxyConfig ProxyConfig
			if instance.Proxy != "" && instance.Proxy != "null" {
				if err := json.Unmarshal([]byte(instance.Proxy), &proxyConfig); err != nil {
					i.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unmarshalling proxy config: %v", instance.Id, err)
					return nil, "", "", err
				}
			}

			if proxyConfig.Host != "" || i.config.ProxyHost != "" {
				clientData.IsProxy = true
			}
		}

		go i.whatsmeowService.StartClient(clientData)
	} else {
		i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Instance already running, settings updated without restarting client", instance.Id)
	}

	// logger.LogInfo("Waiting 1 seconds")
	// time.Sleep(1000 * time.Millisecond)

	// if i.clients.Get(instance.Id) != nil {
	// 	if !i.clients.Get(instance.Id).IsConnected() {
	// 		return instance, "", "", fmt.Errorf("failed to connect")
	// 	}
	// } else {
	// 	return instance, "", "", fmt.Errorf("failed to connect")
	// }

	return instance, instance.Jid, eventString, nil
}

func (i *instances) Reconnect(instance *instance_model.Instance) error {
	// Operator-triggered: refill the automatic retry budget so an instance that
	// already burned its 3 attempts can be recovered from the manager.
	i.whatsmeowService.ResetAutoReconnect(instance.Id)

	// ReconnectClient is called directly instead of going through
	// ensureClientConnected: that helper errors with "client disconnected" when
	// the socket is down, which is exactly when a reconnect is needed. The
	// reconnect path already tears down and restarts the client from scratch.
	return i.whatsmeowService.ReconnectClient(instance.Id)
}

func (i *instances) Disconnect(instance *instance_model.Instance) (*instance_model.Instance, error) {
	client, err := i.ensureClientConnected(instance.Id)
	if err != nil {
		return instance, err
	}

	if client.IsConnected() {
		if client.IsLoggedIn() {
			i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Disconnection successful", instance.Id)
			i.clients.Stop(instance.Id)

			instance.Events = ""

			err := i.instanceRepository.Update(instance)
			if err != nil {
				return instance, err
			}

			return instance, nil
		}
	}

	i.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Ignoring disconnect as it was not connected", instance.Id)
	return instance, nil
}

func (i *instances) Logout(instance *instance_model.Instance) (*instance_model.Instance, error) {
	client, err := i.ensureClientConnected(instance.Id)
	if err != nil {
		return instance, err
	}

	if client.IsLoggedIn() && client.IsConnected() {
		err := client.Logout(context.Background())
		if err != nil {
			return instance, err
		}

		instance.Connected = false
		err = i.instanceRepository.Update(instance)
		if err != nil {
			return instance, err
		}

		i.clients.Remove(instance.Id)

		i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Logout successful", instance.Id)
		return instance, nil
	}

	if client.IsConnected() {
		client.Disconnect()

		i.clients.Remove(instance.Id)

		i.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Disconnection successful", instance.Id)
		return instance, nil
	}

	i.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Ignoring logout as it was not connected", instance.Id)
	return instance, fmt.Errorf("ignoring logout as it was not connected")
}

func (i *instances) Status(instance *instance_model.Instance) (*StatusStruct, error) {
	client, err := i.ensureClientConnected(instance.Id)
	if err != nil {
		return nil, err
	}

	isConnected := client.IsConnected()
	isLoggedIn := client.IsLoggedIn()

	var myJid *types.JID
	var name string
	if isLoggedIn {
		myJid = client.Store.ID
		name = client.Store.PushName
	}

	status := &StatusStruct{
		Connected: isConnected,
		LoggedIn:  isLoggedIn,
		myJid:     myJid,
		Name:      name,
	}

	return status, nil
}

// GetLimits returns WhatsApp's reachout timelock and new-chat messaging quota for an
// instance — the account-level limits behind error 463. It serves the value cached on
// connect (the MEX queries are slow/rate-limited); on a cache miss it does a live query
// with a short timeout so the HTTP request never hangs.
func (i *instances) GetLimits(instanceId string) (*LimitsStruct, error) {
	if e, ok := whatsmeow_service.GetCachedAccountLimits(instanceId); ok {
		result := &LimitsStruct{
			ReachoutTimelock: &ReachoutTimelockStruct{
				IsActive:            e.ReachoutActive,
				TimeEnforcementEnds: e.ReachoutEnds,
				EnforcementType:     e.ReachoutType,
			},
		}
		if e.CappingStatus != "" {
			result.NewChatCapping = &NewChatCappingStruct{
				CappingStatus: e.CappingStatus,
				TotalQuota:    e.TotalQuota,
				UsedQuota:     e.UsedQuota,
				CycleEnds:     e.CycleEnds,
			}
		}
		return result, nil
	}

	client, err := i.ensureClientConnected(instanceId)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	result := &LimitsStruct{}

	if tl, err := walimits.GetAccountReachoutTimelock(ctx, client); err != nil {
		i.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Failed to fetch reachout timelock: %v", instanceId, err)
	} else if tl != nil {
		var ends int64
		if tl.IsActive {
			ends = tl.TimeEnforcementEnds.Unix()
		}
		result.ReachoutTimelock = &ReachoutTimelockStruct{
			IsActive:            tl.IsActive,
			TimeEnforcementEnds: ends,
			EnforcementType:     string(tl.EnforcementType),
		}
	}

	if capping, err := walimits.GetNewChatMessageCappingInfo(ctx, client); err != nil {
		i.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Failed to fetch new-chat capping info: %v", instanceId, err)
	} else if capping != nil {
		result.NewChatCapping = &NewChatCappingStruct{
			CappingStatus: string(capping.CappingStatus),
			TotalQuota:    capping.TotalQuota,
			UsedQuota:     capping.UsedQuota,
			CycleEnds:     capping.CycleEndTimestamp.Unix(),
		}
	}

	return result, nil
}

// How long GetQr waits for a QR code, and the polling step. The step is short
// because the QR is usually written within a few hundred milliseconds of the
// client connecting — polling returns as soon as it appears instead of always
// paying the full timeout.
const (
	qrWaitTimeout  = 5 * time.Second
	qrRetryTimeout = 5 * time.Second
	qrPollInterval = 150 * time.Millisecond
)

// waitForQrcode polls the instance row until a QR code is stored, returning it,
// or "" if the timeout elapses first.
func (i *instances) waitForQrcode(instanceId string, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)

	for {
		stored, err := i.instanceRepository.GetInstanceByID(instanceId)
		if err == nil && stored.Qrcode != "" {
			return stored.Qrcode
		}

		if time.Now().After(deadline) {
			return ""
		}

		time.Sleep(qrPollInterval)
	}
}

func (i *instances) GetQr(instance *instance_model.Instance) (*QrcodeStruct, error) {
	logger := i.loggerWrapper.GetLogger(instance.Id)
	client := i.clients.Get(instance.Id)

	// Se não há cliente ou o cliente está logado, precisamos iniciar um novo cliente
	if client == nil || client.IsLoggedIn() {
		if client != nil && client.IsLoggedIn() {
			logger.LogInfo("[%s] Client is logged in, starting new instance for QR code", instance.Id)
		} else {
			logger.LogInfo("[%s] No client found, starting new instance for QR code", instance.Id)
		}

		// Iniciar nova instância para gerar QR code
		err := i.whatsmeowService.StartInstance(instance.Id)
		if err != nil {
			logger.LogError("[%s] Failed to start instance: %v", instance.Id, err)
			return nil, fmt.Errorf("failed to start instance: %w", err)
		}

		// Wait for the QR to be generated. Sleeping a flat 3s made every first
		// request pay the worst case even when the QR was ready in 200ms, and
		// still gave up too early on a slow start — the reported
		// "no QR code available" after a 5s wait. Poll instead: return as soon
		// as the code lands.
		logger.LogInfo("[%s] Waiting for QR code generation...", instance.Id)
		i.waitForQrcode(instance.Id, qrWaitTimeout)

		// Verificar novamente se há cliente
		client = i.clients.Get(instance.Id)
		if client != nil && client.IsLoggedIn() {
			return nil, fmt.Errorf("session already logged in")
		}
	} else if !client.IsConnected() {
		// Se o cliente existe mas não está conectado, pode estar aguardando QR code
		logger.LogInfo("[%s] Client exists but not connected, checking for existing QR code", instance.Id)
	}

	// Buscar instância atualizada do banco para pegar o QR code mais recente
	instance, err := i.instanceRepository.GetInstanceByID(instance.Id)
	if err != nil {
		return nil, err
	}

	// If a passkey ceremony is in progress, there is no QR to scan — return the
	// passkey stage + the #wapk openUrl so the manager can render the
	// "Abrir WhatsApp Web" button. Checked before the empty-QR branch because
	// during a passkey ceremony instance.Qrcode is empty.
	if store := i.whatsmeowService.PasskeyCeremonyStore(); store != nil {
		if token, state, ok := store.StateByInstance(instance.Id); ok {
			logger.LogInfo("[%s] Passkey ceremony active (stage=%s) — returning passkey info instead of QR", instance.Id, state.Stage)
			return &QrcodeStruct{
				PasskeyStage:   state.Stage,
				PasskeyCode:    state.Code,
				PasskeyOpenURL: buildPasskeyOpenURL(token),
			}, nil
		}
	}

	code := instance.Qrcode
	if code == "" {
		// Give it one more, longer window before giving up.
		logger.LogInfo("[%s] No QR code available yet, waiting a bit more...", instance.Id)
		if code = i.waitForQrcode(instance.Id, qrRetryTimeout); code == "" {
			return nil, fmt.Errorf("no QR code available. Please wait a moment and try again")
		}
	}

	parts := strings.Split(code, "|")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid QR code format")
	}

	qr := &QrcodeStruct{
		Qrcode: parts[0],
		Code:   parts[1],
	}

	return qr, nil
}

// buildPasskeyOpenURL builds the URL the manager opens to start the passkey
// ceremony: https://web.whatsapp.com/#wapk=<base64url({t:token,b:publicBase})>.
// publicBase must be the PUBLICLY reachable API base the browser can hit; set it
// via PASSKEY_PUBLIC_URL. Kept in sync with the passkey-helper extension contract.
func buildPasskeyOpenURL(token string) string {
	publicBase := os.Getenv("PASSKEY_PUBLIC_URL")
	if publicBase == "" {
		publicBase = "<SET_PASSKEY_PUBLIC_URL>"
	}
	payload := fmt.Sprintf(`{"t":%q,"b":%q}`, token, publicBase)
	wapk := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return "https://web.whatsapp.com/#wapk=" + wapk
}

func (i *instances) Pair(data *PairStruct, instance *instance_model.Instance) (*PairReturnStruct, error) {
	logger := i.loggerWrapper.GetLogger(instance.Id)
	client := i.clients.Get(instance.Id)

	if client == nil || !client.IsConnected() {
		if client != nil && client.IsLoggedIn() {
			return nil, fmt.Errorf("instance is already authenticated")
		}
		logger.LogInfo("[%s] No active connection, starting instance for phone pairing", instance.Id)
		if err := i.whatsmeowService.StartInstance(instance.Id); err != nil {
			logger.LogError("[%s] Failed to start instance for pairing: %v", instance.Id, err)
			return nil, fmt.Errorf("failed to start instance: %w", err)
		}
		// Wait for the WA websocket connection and initial QR generation to establish.
		// PairPhone must be called after the QR event is received per whatsmeow docs.
		time.Sleep(3 * time.Second)
		client = i.clients.Get(instance.Id)
		if client == nil {
			return nil, fmt.Errorf("failed to initialize client for pairing")
		}
	}

	if client.IsLoggedIn() {
		return nil, fmt.Errorf("instance is already authenticated")
	}

	code, err := client.PairPhone(context.Background(), data.Phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		logger.LogError("[%s] PairPhone failed: %v", instance.Id, err)
		return nil, fmt.Errorf("pairing failed: %w", err)
	}

	return &PairReturnStruct{PairingCode: code}, nil
}

func (i *instances) GetAll() ([]*instance_model.Instance, error) {
	instances, err := i.instanceRepository.GetAll(i.config.ClientName)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if client := i.clients.Get(instance.Id); client != nil {
			instance.Connected = client.IsLoggedIn()
		} else {
			instance.Connected = false
		}
	}

	// NOTE: the stored proxy (including credentials) is returned as-is. Both this
	// route and /instance/info are admin-only, and the manager needs the saved
	// values to pre-fill the proxy form instead of showing blank fields.

	return instances, nil
}

func (i *instances) Info(instanceId string) (*instance_model.Instance, error) {
	instance, err := i.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return nil, err
	}

	// Atualiza o status connected com base no estado real do cliente
	if client := i.clients.Get(instance.Id); client != nil {
		instance.Connected = client.IsLoggedIn()
	} else {
		instance.Connected = false
	}

	// The stored proxy is kept in the response — see the note in GetAll.

	return instance, nil
}

func (i *instances) Delete(id string) error {
	instance, err := i.instanceRepository.GetInstanceByID(id)
	if err != nil {
		return err
	}

	if client := i.clients.Get(instance.Id); client != nil && client.IsConnected() {
		if client.IsLoggedIn() {
			if err := client.Logout(context.Background()); err != nil {
				i.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Logout failed while deleting: %v", instance.Id, err)
			}
		}
		client.Disconnect()
	}

	// Limpar todos os recursos da instância antes de deletar. Remove signals the
	// instance goroutine to stop and drops the entry; it never closes a channel
	// anyone still sends on.
	i.clients.Remove(instance.Id)

	// Limpar cache via whatsmeow service
	err = i.whatsmeowService.ClearInstanceCache(instance.Id, instance.Token)
	if err != nil {
		i.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Failed to clear instance cache: %v", instance.Id, err)
	}

	err = i.instanceRepository.Delete(id)
	if err != nil {
		return err
	}

	return nil
}

func (i *instances) SetProxy(id string, proxyConfig *ProxyConfig) error {
	instance, err := i.instanceRepository.GetInstanceByID(id)
	if err != nil {
		return err
	}

	// Validate proxy configuration
	if proxyConfig == nil {
		return fmt.Errorf("proxy configuration cannot be nil")
	}

	if proxyConfig.Host == "" {
		return fmt.Errorf("proxy host is required")
	}

	if proxyConfig.Port == "" {
		return fmt.Errorf("proxy port is required")
	}

	proxyConfig.Protocol = utils.NormalizeProxyProtocol(proxyConfig.Protocol, proxyConfig.Port)

	// Convert proxy config to JSON
	proxyJSON, err := json.Marshal(proxyConfig)
	if err != nil {
		i.loggerWrapper.GetLogger(id).LogError("[%s] Failed to marshal proxy config: %v", id, err)
		return fmt.Errorf("failed to marshal proxy configuration: %v", err)
	}

	instance.Proxy = string(proxyJSON)

	// Update instance in database
	err = i.instanceRepository.Update(instance)
	if err != nil {
		i.loggerWrapper.GetLogger(id).LogError("[%s] Failed to update instance with proxy: %v", id, err)
		return err
	}

	i.loggerWrapper.GetLogger(id).LogInfo("[%s] Proxy configuration updated: %s://%s:%s", id, proxyConfig.Protocol, proxyConfig.Host, proxyConfig.Port)

	// Operator changed the proxy config — give the instance a fresh automatic
	// retry budget so an exhausted one can recover with the new proxy.
	i.whatsmeowService.ResetAutoReconnect(id)

	// Reiniciar o cliente para aplicar as mudanças de proxy.
	// Usa ReconnectClient diretamente pois funciona mesmo quando a instância está desconectada.
	go func() {
		if err := i.whatsmeowService.ReconnectClient(id); err != nil {
			i.loggerWrapper.GetLogger(id).LogError("[%s] Failed to reconnect after proxy change: %v", id, err)
		}
	}()

	return nil
}

func (i *instances) SetProxyFromStruct(id string, data *SetProxyStruct) error {
	if data == nil {
		return fmt.Errorf("proxy data cannot be nil")
	}

	proxyConfig := &ProxyConfig{
		Protocol: data.Protocol,
		Host:     data.Host,
		Port:     data.Port,
		Username: data.Username,
		Password: data.Password,
	}

	return i.SetProxy(id, proxyConfig)
}

// GetProxy returns the proxy configuration saved for an instance, or nil when
// none is set. Admin-only route — credentials are included so the manager can
// pre-fill the form.
func (i *instances) GetProxy(id string) (*ProxyConfig, error) {
	instance, err := i.instanceRepository.GetInstanceByID(id)
	if err != nil {
		return nil, err
	}

	if instance.Proxy == "" {
		return nil, nil
	}

	var proxyConfig ProxyConfig
	if err := json.Unmarshal([]byte(instance.Proxy), &proxyConfig); err != nil {
		i.loggerWrapper.GetLogger(id).LogError("[%s] Failed to unmarshal stored proxy config: %v", id, err)
		return nil, fmt.Errorf("stored proxy configuration is invalid: %w", err)
	}

	// Older rows may hold the literal "null" or an object without a host, which
	// unmarshals into a zero struct. Report those as "no proxy configured".
	if proxyConfig.Host == "" {
		return nil, nil
	}

	return &proxyConfig, nil
}

// ReconnectProxy re-establishes the WhatsApp connection using the proxy already
// saved for the instance, without changing the stored configuration. Useful when
// the proxy dropped and the socket needs to be rebuilt through it.
func (i *instances) ReconnectProxy(id string) error {
	proxyConfig, err := i.GetProxy(id)
	if err != nil {
		return err
	}

	if proxyConfig == nil || proxyConfig.Host == "" {
		return fmt.Errorf("no proxy configured for this instance")
	}

	i.loggerWrapper.GetLogger(id).LogInfo(
		"[%s] Reconnecting through proxy %s://%s:%s",
		id, proxyConfig.Protocol, proxyConfig.Host, proxyConfig.Port,
	)

	// Operator-triggered — give the instance a fresh automatic retry budget.
	i.whatsmeowService.ResetAutoReconnect(id)

	return i.whatsmeowService.ReconnectClient(id)
}

func (i *instances) RemoveProxy(id string) error {
	instance, err := i.instanceRepository.GetInstanceByID(id)
	if err != nil {
		return err
	}

	instance.Proxy = ""

	err = i.instanceRepository.Update(instance)
	if err != nil {
		return err
	}

	i.loggerWrapper.GetLogger(id).LogInfo("[%s] Proxy configuration removed", id)

	i.whatsmeowService.ResetAutoReconnect(id)

	go func() {
		if err := i.whatsmeowService.ReconnectClient(id); err != nil {
			i.loggerWrapper.GetLogger(id).LogError("[%s] Failed to reconnect after proxy removal: %v", id, err)
		}
	}()

	return nil
}

func (i *instances) ForceReconnect(instanceId string, number string) error {
	// An instance that was never started has no client at all. Dereferencing the
	// old map straight away panicked exactly in that case — the one where a
	// forced reconnect is most likely to be asked for.
	if client := i.clients.Get(instanceId); client != nil && client.IsConnected() && client.IsLoggedIn() {
		return fmt.Errorf("client already connected")
	}

	err := i.whatsmeowService.ForceUpdateJid(instanceId, number)
	if err != nil {
		return err
	}

	instance, err := i.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return err
	}

	subscribedEvents := strings.Split(instance.Events, ",")

	clientData := &whatsmeow_service.ClientData{
		Instance:      instance,
		Subscriptions: subscribedEvents,
		Phone:         "",
		IsProxy:       false,
	}

	// Same guard as above: an empty instance.Proxy alongside a global proxy must
	// not be fed to Unmarshal, and legacy rows may hold the literal "null".
	if instance.Proxy != "" || i.config.ProxyHost != "" {
		var proxyConfig ProxyConfig
		if instance.Proxy != "" && instance.Proxy != "null" {
			if err := json.Unmarshal([]byte(instance.Proxy), &proxyConfig); err != nil {
				i.loggerWrapper.GetLogger(instance.Id).LogError("[%s] error unmarshalling proxy config: %v", instance.Id, err)
				return err
			}
		}

		if proxyConfig.Host != "" || i.config.ProxyHost != "" {
			clientData.IsProxy = true
		}
	}

	if client := i.clients.Get(instance.Id); client != nil {
		client.Disconnect()

		i.clients.Remove(instance.Id)
	}

	go i.whatsmeowService.StartClient(clientData)

	time.Sleep(2 * time.Second)

	client := i.clients.Get(instance.Id)
	if client == nil {
		return fmt.Errorf("failed to connect")
	}

	if !client.IsConnected() {
		return fmt.Errorf("failed to connect")
	}

	if !client.IsLoggedIn() {
		return fmt.Errorf("failed to login")
	}

	return nil
}

func (i *instances) GetInstanceByToken(token string) (*instance_model.Instance, error) {
	return i.instanceRepository.GetInstanceByToken(token)
}

func (i *instances) GetLogs(instanceId string, startDate, endDate time.Time, level string, limit int) ([]logger_wrapper.LogEntry, error) {
	// Inicializa o slice vazio para garantir que nunca retorne null
	logs := make([]logger_wrapper.LogEntry, 0)

	// Define valores padrão
	if limit <= 0 {
		limit = 100 // Limite padrão de 100 registros
	}

	// Se não foi fornecida data inicial, usa 7 dias atrás
	if startDate.IsZero() {
		startDate = time.Now().AddDate(0, 0, -7)
	}

	// Se não foi fornecida data final, usa data atual
	if endDate.IsZero() {
		endDate = time.Now()
	}

	// Ajusta as datas para início e fim do dia
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, time.UTC)
	endDate = time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, time.UTC)

	// Garante que a data inicial não seja posterior à data final
	if startDate.After(endDate) {
		return logs, fmt.Errorf("data inicial não pode ser posterior à data final")
	}

	// Níveis de log válidos
	validLevels := map[string]bool{
		"INFO":  true,
		"ERROR": true,
		"WARN":  true,
		"DEBUG": true,
	}

	var levelArray []string
	if level == "" {
		// Se nenhum nível foi especificado, usa todos
		levelArray = []string{"INFO", "ERROR", "WARN", "DEBUG"}
	} else {
		// Divide e normaliza os níveis fornecidos
		for _, l := range strings.Split(level, ",") {
			l = strings.TrimSpace(strings.ToUpper(l))
			if !validLevels[l] {
				return logs, fmt.Errorf("nível de log inválido: %s", l)
			}
			levelArray = append(levelArray, l)
		}
	}

	// Lê os logs do arquivo
	logPath := filepath.Join(i.config.LogDirectory, instanceId, "instance.log")
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return logs, nil // Retorna array vazio se arquivo não existir
		}
		return logs, fmt.Errorf("erro ao abrir arquivo de log: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Aumenta o buffer do scanner para lidar com linhas grandes
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		var entry logger_wrapper.LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Ignora linhas inválidas
		}

		// Ajusta o timestamp da entrada para UTC para comparação correta
		entry.Timestamp = entry.Timestamp.UTC()

		// Aplica os filtros
		if entry.Timestamp.Before(startDate) || entry.Timestamp.After(endDate) {
			continue
		}

		if !slices.Contains(levelArray, entry.Level) {
			continue
		}

		logs = append(logs, entry)

		// Verifica o limite
		if len(logs) >= limit {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return logs, fmt.Errorf("erro ao ler arquivo de log: %v", err)
	}

	// Ordena os logs por timestamp em ordem decrescente
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.After(logs[j].Timestamp)
	})

	return logs, nil
}

func (i *instances) GetAdvancedSettings(instanceId string) (*instance_model.AdvancedSettings, error) {
	i.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Getting advanced settings", instanceId)

	settings, err := i.instanceRepository.GetAdvancedSettings(instanceId)
	if err != nil {
		i.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error getting advanced settings: %v", instanceId, err)
		return nil, err
	}

	return settings, nil
}

func (i *instances) UpdateAdvancedSettings(instanceId string, settings *instance_model.AdvancedSettings) error {
	i.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Updating advanced settings", instanceId)

	err := i.instanceRepository.UpdateAdvancedSettings(instanceId, settings)
	if err != nil {
		i.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error updating advanced settings: %v", instanceId, err)
		return err
	}

	// Sincroniza as configurações na instância em execução
	err = i.whatsmeowService.UpdateInstanceAdvancedSettings(instanceId)
	if err != nil {
		i.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Error syncing advanced settings to runtime: %v", instanceId, err)
		// Não falha a operação, apenas loga o warning
	}

	i.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Advanced settings updated successfully", instanceId)
	return nil
}

func (i *instances) AddWebhook(instanceId string, url string) (*instance_model.Instance, error) {
	instance, err := i.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return nil, err
	}
	for _, existing := range instance.Webhooks {
		if existing == url {
			return instance, nil
		}
	}
	instance.Webhooks = append(instance.Webhooks, url)
	if err := i.instanceRepository.Update(instance); err != nil {
		return nil, err
	}
	i.syncWebhooksToRuntime(instance.Id)
	return instance, nil
}

func (i *instances) RemoveWebhook(instanceId string, url string) (*instance_model.Instance, error) {
	instance, err := i.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(instance.Webhooks))
	for _, existing := range instance.Webhooks {
		if existing != url {
			filtered = append(filtered, existing)
		}
	}
	instance.Webhooks = filtered
	if err := i.instanceRepository.Update(instance); err != nil {
		return nil, err
	}
	i.syncWebhooksToRuntime(instance.Id)
	return instance, nil
}

// syncWebhooksToRuntime pushes the stored webhook list into the running client.
//
// The event handlers read the instance snapshot held by the live client, not
// the database, so without this a webhook added to a connected instance was
// only honoured after the next reconnect — it looked like webhooks simply did
// not fire. An instance that is not running has nothing to sync and will pick
// the list up when it connects.
func (i *instances) syncWebhooksToRuntime(instanceId string) {
	if err := i.whatsmeowService.UpdateInstanceSettings(instanceId); err != nil {
		i.loggerWrapper.GetLogger(instanceId).LogInfo(
			"[%s] Webhook list saved; instance is not running, it will apply on connect", instanceId,
		)
		return
	}
	i.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Webhook list applied to the running instance", instanceId)
}

func (i *instances) ListWebhooks(instanceId string) ([]string, error) {
	instance, err := i.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return nil, err
	}
	all := make([]string, 0)
	if instance.Webhook != "" && instance.Webhook != "disabled" {
		all = append(all, instance.Webhook)
	}
	all = append(all, instance.Webhooks...)
	return all, nil
}

func NewInstanceService(
	instanceRepository instance_repository.InstanceRepository,
	clients *whatsmeow_registry.Clients,
	whatsmeowService whatsmeow_service.WhatsmeowService,
	config *config.Config,
	loggerWrapper *logger_wrapper.LoggerManager,
) InstanceService {
	return &instances{
		instanceRepository: instanceRepository,
		clients:            clients,
		whatsmeowService:   whatsmeowService,
		config:             config,
		loggerWrapper:      loggerWrapper,
	}
}
