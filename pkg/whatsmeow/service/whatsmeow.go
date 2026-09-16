package whatsmeow_service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/webp"

	_ "github.com/lib/pq"
	"github.com/patrickmn/go-cache"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/appstate"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"github.com/EvolutionAPI/evolution-go/pkg/config"
	producer_interfaces "github.com/EvolutionAPI/evolution-go/pkg/events/interfaces"
	instance_model "github.com/EvolutionAPI/evolution-go/pkg/instance/model"
	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
	"github.com/EvolutionAPI/evolution-go/pkg/internal/event_types"
	label_model "github.com/EvolutionAPI/evolution-go/pkg/label/model"
	label_repository "github.com/EvolutionAPI/evolution-go/pkg/label/repository"
	logger_wrapper "github.com/EvolutionAPI/evolution-go/pkg/logger"
	message_model "github.com/EvolutionAPI/evolution-go/pkg/message/model"
	message_repository "github.com/EvolutionAPI/evolution-go/pkg/message/repository"
	"github.com/EvolutionAPI/evolution-go/pkg/passkey/ceremony"
	poll_service "github.com/EvolutionAPI/evolution-go/pkg/poll/service"
	storage_interfaces "github.com/EvolutionAPI/evolution-go/pkg/storage/interfaces"
	"github.com/EvolutionAPI/evolution-go/pkg/utils"
	voip_registry "github.com/EvolutionAPI/evolution-go/pkg/voip/registry"
	"github.com/EvolutionAPI/evolution-go/pkg/walimits"
	whatsmeow_registry "github.com/EvolutionAPI/evolution-go/pkg/whatsmeow/registry"
)

type WhatsmeowService interface {
	StartClient(clientData *ClientData)
	ConnectOnStartup(clientName string)
	StartInstance(instanceId string) error
	ReconnectClient(instanceId string) error
	ClearInstanceCache(instanceId string, token string) error
	CallWebhook(instance *instance_model.Instance, queueName string, jsonData []byte)
	SendToGlobalQueues(event string, jsonData []byte, userId string)
	ForceUpdateJid(instanceId string, number string) error
	UpdateInstanceSettings(instanceId string) error
	UpdateInstanceAdvancedSettings(instanceId string) error
	GetPollService() poll_service.PollService // NOVO: Acesso ao serviço de polls

	// Passkey (WebAuthn) pairing bridge — the ceremony store is read by the public
	// /passkey-ceremony endpoints and written by the whatsmeow event goroutine.
	PasskeyCeremonyStore() *ceremony.Store
	SubmitPasskeyResponse(instanceId string, resp *types.WebAuthnResponse) error
	ConfirmPasskey(instanceId string) error

	// Bounded automatic reconnection. ScheduleAutoReconnect is called when an
	// instance drops unexpectedly; ResetAutoReconnect refills the attempt budget
	// (on a successful connect, a logout, or an operator-triggered reconnect).
	ScheduleAutoReconnect(instanceId string)
	ResetAutoReconnect(instanceId string)
	AutoReconnectAttempts(instanceId string) int

	// VoIP calls. The registry owns the per-call CallManagers; the call service
	// reaches them through here, and this service feeds it inbound signaling.
	CallRegistry() *voip_registry.Registry
	GetClient(instanceId string) *whatsmeow.Client

	// Searchable message archive — backs the MCP search/fetch tools.
	ArchiveOutgoingMessage(instanceId, messageID, chatJID string, msg *waE2E.Message, timestamp time.Time)
	SearchArchivedMessages(filter message_model.MessageSearchFilter) ([]message_model.ArchivedMessage, int64, error)
	GetArchivedMessage(instanceId, messageID string) (*message_model.ArchivedMessage, error)
	ListArchivedChats(instanceId string, limit int) ([]message_model.ChatSummary, error)
}

// maxAutoReconnectAttempts bounds how many times an instance is brought back up
// automatically after an unexpected drop or a transport-level connect failure.
// Without a bound, retries would run forever and hammer WhatsApp when the
// account is banned or the proxy is dead. The budget is refilled on every
// successful connect and whenever the operator changes the proxy config.
// maxAutoReconnectAttempts bounds retries for an instance that has never been
// paired. Those fail for reasons retrying cannot fix — no session, bad
// credentials — so looping forever would only burn resources.
//
// An instance that HAS paired is a different case: a drop there is a transient
// network or proxy problem, and the operator expects it to come back on its own.
// Those retry indefinitely, with the delay growing to reconnectBackoffCap.
const maxAutoReconnectAttempts = 3

// reconnectBackoffCap is the ceiling for the growing retry delay. Long enough
// that a night-long outage costs a handful of attempts per hour, short enough
// that recovery is quick once the network returns.
const reconnectBackoffCap = 5 * time.Minute

// reconnectDelay returns the wait before the given 1-based attempt. It ramps up
// quickly for the first few tries, when most drops resolve, then holds.
func reconnectDelay(attempt int) time.Duration {
	switch {
	case attempt <= 1:
		return 5 * time.Second
	case attempt == 2:
		return 15 * time.Second
	case attempt == 3:
		return 30 * time.Second
	case attempt == 4:
		return time.Minute
	case attempt == 5:
		return 2 * time.Minute
	default:
		return reconnectBackoffCap
	}
}

// reconnectTracker counts automatic reconnection attempts per instance.
//
// It is held by pointer on whatsmeowService so callers share one tracker rather
// than a copy of its mutex.
type reconnectTracker struct {
	mu       sync.Mutex
	attempts map[string]int
	inFlight map[string]bool
	// generation is bumped whenever the budget is refilled. A sleeping retry
	// compares the generation it started under, not its attempt number:
	// comparing numbers meant a reset followed by a fresh attempt could land
	// back on the same value, and the stale retry would then think it was still
	// current and reconnect an instance the operator had just taken over.
	generation map[string]uint64
}

func newReconnectTracker() *reconnectTracker {
	return &reconnectTracker{
		attempts:   make(map[string]int),
		inFlight:   make(map[string]bool),
		generation: make(map[string]uint64),
	}
}

// begin claims the next attempt slot for an instance. It returns the attempt
// number (1-based) and false when the budget is exhausted or another attempt is
// already running, so concurrent Disconnected events can't stack up retries.
func (t *reconnectTracker) begin(instanceId string, unlimited bool) (int, uint64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.inFlight[instanceId] {
		return t.attempts[instanceId], t.generation[instanceId], false
	}
	// A paired instance keeps trying: the budget only applies to instances that
	// have never managed to log in.
	if !unlimited && t.attempts[instanceId] >= maxAutoReconnectAttempts {
		return t.attempts[instanceId], t.generation[instanceId], false
	}

	t.attempts[instanceId]++
	t.inFlight[instanceId] = true
	return t.attempts[instanceId], t.generation[instanceId], true
}

func (t *reconnectTracker) done(instanceId string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.inFlight, instanceId)
}

// next claims one more attempt for a retry loop that already holds the
// in-flight slot. It exists so a paired instance can keep retrying inside the
// goroutine that started: re-entering through begin would deadlock against its
// own in-flight flag.
func (t *reconnectTracker) next(instanceId string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attempts[instanceId]++
	return t.attempts[instanceId]
}

func (t *reconnectTracker) reset(instanceId string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, instanceId)
	t.generation[instanceId]++
}

// current reports whether a retry started under gen is still the wanted one.
func (t *reconnectTracker) current(instanceId string, gen uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.generation[instanceId] == gen
}

func (t *reconnectTracker) count(instanceId string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.attempts[instanceId]
}

type clientVersion struct {
	Major int
	Minor int
	Patch int
}

type whatsmeowService struct {
	instanceRepository instance_repository.InstanceRepository
	authDB             *sql.DB
	messageRepository  message_repository.MessageRepository
	labelRepository    label_repository.LabelRepository
	pollService        poll_service.PollService // NOVO: Serviço de enquetes
	config             *config.Config
	userInfoCache      *cache.Cache
	clients            *whatsmeow_registry.Clients
	myClientPointer    *myClientStore
	rabbitmqProducer   producer_interfaces.Producer
	webhookProducer    producer_interfaces.Producer
	websocketProducer  producer_interfaces.Producer
	sqliteDB           *sql.DB
	exPath             string
	mediaStorage       storage_interfaces.MediaStorage
	processedMessages  *cache.Cache
	natsProducer       producer_interfaces.Producer
	loggerWrapper      *logger_wrapper.LoggerManager
	passkeyCeremony    *ceremony.Store
	reconnect          *reconnectTracker
	callRegistry       *voip_registry.Registry
}

type MyClient struct {
	service        WhatsmeowService
	WAClient       *whatsmeow.Client
	eventHandlerID uint32
	userID         string

	// mu guards the fields below, which the HTTP handlers rewrite (via
	// UpdateInstanceSettings) while the whatsmeow event goroutine is reading
	// them. Without it, a settings change raced every incoming message.
	mu                 sync.RWMutex
	instance           *instance_model.Instance
	token              string
	subscriptions      []string
	webhookUrl         string
	rabbitmqEnable     string
	natsEnable         string
	websocketEnable    string
	instanceRepository instance_repository.InstanceRepository
	messageRepository  message_repository.MessageRepository
	labelRepository    label_repository.LabelRepository
	pollService        poll_service.PollService // NOVO: Serviço de enquetes
	clients            *whatsmeow_registry.Clients
	entry              *whatsmeow_registry.Entry
	userInfoCache      *cache.Cache
	config             *config.Config
	historySyncID      int32
	rabbitmqProducer   producer_interfaces.Producer
	webhookProducer    producer_interfaces.Producer
	websocketProducer  producer_interfaces.Producer
	mediaStorage       storage_interfaces.MediaStorage
	processedMessages  *cache.Cache
	natsProducer       producer_interfaces.Producer
	loggerWrapper      *logger_wrapper.LoggerManager
	qrcodeCount        int
	passkeyCeremony    *ceremony.Store
}

type ClientData struct {
	Instance      *instance_model.Instance
	Subscriptions []string
	Phone         string
	IsProxy       bool
}

type Values struct {
	m map[string]string
}

func (v Values) Get(key string) string {
	return v.m[key]
}

type UserCollection struct {
	Users map[types.JID]types.UserInfo
}

type ProxyConfig struct {
	Protocol string `json:"protocol,omitempty"`
	Host     string `json:"host"`
	Password string `json:"password"`
	Port     string `json:"port"`
	Username string `json:"username"`
}

func (w *whatsmeowService) ReconnectClient(instanceId string) error {
	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Starting reconnection process - simulating restart", instanceId)

	// Passo 1: Limpar conexão existente se houver
	if client := w.clients.Get(instanceId); client != nil {
		w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Disconnecting existing client", instanceId)

		// Remover event handler ANTES de desconectar para evitar eventos espúrios
		if mycli, ok := w.myClientPointer.get(instanceId); ok {
			if mycli.eventHandlerID != 0 {
				client.RemoveEventHandler(mycli.eventHandlerID)
				w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Event handler removed", instanceId)
			}
		}

		// Desconectar o cliente WebSocket
		if client.IsConnected() {
			client.Disconnect()
			w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] WebSocket disconnected", instanceId)
		}
	}

	// Passo 2: Limpar todos os recursos da instância
	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Cleaning up resources", instanceId)

	// Stop the StartClient goroutine. Remove closes the entry's done channel,
	// which every goroutine for this instance is selecting on — non-blocking,
	// and safe however many times the teardown paths reach it.
	if w.clients.Remove(instanceId) {
		w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Stop signal sent", instanceId)
	}
	w.myClientPointer.remove(instanceId)

	// Limpar cache de userInfo para esta instância
	if instance, err := w.instanceRepository.GetInstanceByID(instanceId); err == nil {
		w.userInfoCache.Delete(instance.Token)
		w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] UserInfo cache cleared for token: %s", instanceId, instance.Token)
	}

	// Passo 3: Atualizar status no banco
	instance, err := w.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return fmt.Errorf("failed to get instance: %v", err)
	}

	instance.Connected = false
	instance.DisconnectReason = "Reconnecting"
	err = w.instanceRepository.UpdateConnected(instanceId, false, "Reconnecting")
	if err != nil {
		w.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Failed to update disconnect status: %v", instanceId, err)
	}

	// Passo 4: Aguardar um pouco para garantir limpeza completa
	time.Sleep(2 * time.Second)

	// Passo 5: Iniciar nova instância como se fosse a primeira vez
	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Starting fresh instance", instanceId)
	return w.StartInstance(instanceId)
}

// abortForProxyFailure stops a connection attempt for an instance whose proxy
// is unusable, instead of quietly connecting straight to WhatsApp.
//
// Connecting without the configured proxy would leak the server's real IP to
// WhatsApp — exactly what the operator set the proxy up to avoid — and it would
// do so invisibly, since the instance would show up as healthy. Failing loudly
// leaves the reason on the instance so it is visible in the UI and the API.
func (w *whatsmeowService) abortForProxyFailure(instanceId, reason string, cause error) {
	logger := w.loggerWrapper.GetLogger(instanceId)
	logger.LogError(
		"[%s] %s: %v — refusing to connect without it (the real IP would be exposed)",
		instanceId, reason, cause,
	)

	// Drop the half-built client so nothing can accidentally use an unproxied one.
	w.clients.Remove(instanceId)

	message := fmt.Sprintf("Proxy unavailable: %s (%v)", reason, cause)
	if err := w.instanceRepository.UpdateConnected(instanceId, false, message); err != nil {
		logger.LogWarn("[%s] Failed to record the proxy failure: %v", instanceId, err)
	}
}

// ScheduleAutoReconnect brings an instance back up after an unexpected drop or
// a failed dial, up to maxAutoReconnectAttempts times with increasing backoff.
//
// Each Disconnected event or transport-level connect failure consumes one attempt.
// A successful reconnect emits
// events.Connected, which calls ResetAutoReconnect and refills the budget — so
// a flapping connection is retried indefinitely as long as it keeps recovering,
// while a permanently broken one gives up after 3 tries instead of looping.
func (w *whatsmeowService) ScheduleAutoReconnect(instanceId string) {
	logger := w.loggerWrapper.GetLogger(instanceId)

	// An instance that already paired keeps retrying for as long as it takes.
	// Its JID is the durable proof of that: it survives restarts and is only
	// cleared on logout, unlike the in-memory client.
	paired := false
	if instance, err := w.instanceRepository.GetInstanceByID(instanceId); err == nil && instance.Jid != "" {
		paired = true
	}

	attempt, generation, ok := w.reconnect.begin(instanceId, paired)
	if !ok {
		logger.LogWarn(
			"[%s] Auto-reconnect not scheduled (attempts used: %d/%d). Manual reconnect required.",
			instanceId, attempt, maxAutoReconnectAttempts,
		)
		return
	}

	// The retries run as one loop inside a single goroutine rather than each
	// attempt scheduling the next. Re-entering ScheduleAutoReconnect would hit
	// the in-flight flag this goroutine still holds and be refused, which would
	// silently end the chain at the first failure.
	go func() {
		defer w.reconnect.done(instanceId)

		for {
			delay := reconnectDelay(attempt)
			if paired {
				logger.LogInfo("[%s] Auto-reconnect attempt %d scheduled in %s (paired: retrying until it comes back)", instanceId, attempt, delay)
			} else {
				logger.LogInfo("[%s] Auto-reconnect attempt %d/%d scheduled in %s", instanceId, attempt, maxAutoReconnectAttempts, delay)
			}
			time.Sleep(delay)

			// The operator may have reconnected (or removed) the instance while
			// we were waiting. ResetAutoReconnect bumps the generation, so a
			// generation mismatch means this retry is no longer wanted.
			if !w.reconnect.current(instanceId, generation) {
				logger.LogInfo("[%s] Auto-reconnect attempt %d superseded, skipping", instanceId, attempt)
				return
			}

			err := w.ReconnectClient(instanceId)
			if err == nil {
				logger.LogInfo("[%s] Auto-reconnect attempt %d dispatched", instanceId, attempt)
				return
			}

			logger.LogError("[%s] Auto-reconnect attempt %d failed: %v", instanceId, attempt, err)

			if !paired {
				if attempt >= maxAutoReconnectAttempts {
					logger.LogError(
						"[%s] Auto-reconnect exhausted after %d attempts. Manual reconnect required.",
						instanceId, maxAutoReconnectAttempts,
					)
				}
				return
			}

			// A failed dial emits no Disconnected event, so nothing else would
			// schedule the next try — keep the loop going instead.
			attempt = w.reconnect.next(instanceId)
		}
	}()
}

// ResetAutoReconnect refills the automatic reconnection budget for an instance.
func (w *whatsmeowService) ResetAutoReconnect(instanceId string) {
	w.reconnect.reset(instanceId)
}

// AutoReconnectAttempts reports how many automatic attempts have been consumed.
func (w *whatsmeowService) AutoReconnectAttempts(instanceId string) int {
	return w.reconnect.count(instanceId)
}

func (w *whatsmeowService) ForceUpdateJid(instanceId string, number string) error {
	instance, err := w.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error getting instance: %v", instanceId, err)
		return err
	}

	if instance.Jid == "" && number != "" {
		sqlDeviceSearch := fmt.Sprintf("SELECT jid FROM whatsmeow_device WHERE jid LIKE '%%%s%%'", number)
		rows, err := w.authDB.Query(sqlDeviceSearch)
		if err != nil {
			w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error getting device: %v", instanceId, err)
			return err
		}

		defer rows.Close()

		var latestJid string
		var latestSession int

		for rows.Next() {
			type deviceStruct struct {
				Jid string `json:"jid"`
			}
			var device deviceStruct
			err := rows.Scan(&device.Jid)
			if err != nil {
				w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error getting device: %v", instanceId, err)
				return err
			}

			// Extrair o número da sessão do JID
			parts := strings.Split(device.Jid, ":")
			if len(parts) == 2 {
				sessionPart := strings.Split(parts[1], "@")[0]
				session, err := strconv.Atoi(sessionPart)
				if err != nil {
					w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error parsing session number: %v", instanceId, err)
					return err
				}

				// Atualizar se for a sessão mais recente
				if session > latestSession {
					latestSession = session
					latestJid = device.Jid
				}
			}
		}

		// Atualizar a instância com o JID mais recente
		if latestJid != "" {
			instance.Jid = latestJid
			err = w.instanceRepository.UpdateJid(instanceId, latestJid)
			if err != nil {
				w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error updating instance: %v", instanceId, err)
			}
			w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Updated instance with latest JID: %s (session: %d)", instanceId, latestJid, latestSession)
		}
	}

	return nil
}

// applyWAVersion sets the client version used in the websocket handshake.
//
// store.DeviceProps.Version only describes the companion device in the pairing
// payload — the handshake advertises store's package-level WAVersion, which
// ships with a hardcoded value that goes stale as WhatsApp moves forward. Left
// unset, WhatsApp answers 405 "client outdated" and the QR channel yields
// err-client-outdated instead of a QR code, so pairing silently stops working.
func applyWAVersion(v clientVersion) {
	if v.Major == 0 && v.Minor == 0 && v.Patch == 0 {
		return
	}
	store.SetWAVersion(store.WAVersionContainer{uint32(v.Major), uint32(v.Minor), uint32(v.Patch)})
}

func (w *whatsmeowService) StartClient(cd *ClientData) {

	w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("Starting websocket connection to Whatsapp for user '%s'", cd.Instance.Id)

	var deviceStore *store.Device

	if client := w.clients.Get(cd.Instance.Id); client != nil && client.IsConnected() {
		return
	}

	// Claim this instance before doing any work. Begin stops whatever was
	// running for it and hands back the handle this goroutine watches, so two
	// concurrent QR requests can no longer leave two live clients fighting over
	// one instance — the older one is told to stop here.
	entry := w.clients.Begin(cd.Instance.Id)

	// One shared device store for the whole process. Opening a new one here per
	// connection attempt leaked a connection pool every time, which is what
	// exhausted Postgres and made every start and login fail with
	// "sorry, too many clients already" — see store_container.go.
	container, err := w.deviceStore()
	if err != nil {
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to open the device store: %v", cd.Instance.Id, err)
		return
	}

	if cd.Instance.Jid != "" {
		jid, _ := utils.ParseJID(cd.Instance.Jid)
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Jid found. Getting device store for jid: %s", cd.Instance.Id, jid)
		deviceStore, err = container.GetDevice(context.Background(), jid)
		if err != nil {
			w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Erro ao obter device store: %v", cd.Instance.Id, err)
			return
		}
	} else {
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] No jid found. Creating new device", cd.Instance.Id)
		deviceStore = container.NewDevice()
	}

	if deviceStore == nil {
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] No store found. Creating new one", cd.Instance.Id)
		deviceStore = container.NewDevice()

		cd.Instance.Connected = false
		err := w.instanceRepository.UpdateConnected(cd.Instance.Id, cd.Instance.Connected, cd.Instance.DisconnectReason)
		if err != nil {
			w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Error updating instance: %s", cd.Instance.Id, err)
		}
	}

	// Resolve the version to advertise BEFORE touching the shared device props:
	// fetching it from the web is a network call, and doing that while holding
	// the props lock would serialise every instance's startup behind it.
	var version clientVersion

	if cd.Instance.OsName == "" {
		cd.Instance.OsName = utils.WhatsAppGetUserOS()
	}

	if w.config.WhatsappVersionMajor != 0 && w.config.WhatsappVersionMinor != 0 && w.config.WhatsappVersionPatch != 0 {
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Setting whatsapp version to %d.%d.%d", cd.Instance.Id, w.config.WhatsappVersionMajor, w.config.WhatsappVersionMinor, w.config.WhatsappVersionPatch)
		version = clientVersion{
			Major: w.config.WhatsappVersionMajor,
			Minor: w.config.WhatsappVersionMinor,
			Patch: w.config.WhatsappVersionPatch,
		}
	} else {
		// Try to fetch version from WhatsApp Web
		webVersion, err := fetchWhatsAppWebVersion()
		if err != nil {
			w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to fetch WhatsApp Web version: %v", cd.Instance.Id, err)
		} else {
			w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Setting whatsapp version from web to %d.%d.%d", cd.Instance.Id, webVersion.Major, webVersion.Minor, webVersion.Patch)
			version = *webVersion
		}
	}

	// The props themselves are applied under devicePropsMu, together with the
	// Connect that reads them — see device_props.go.
	osName := cd.Instance.OsName

	// 🔒 FIX: Sempre criar logger, mesmo que WaDebug esteja vazio
	// Usar "INFO" como nível mínimo para garantir que logs importantes apareçam
	minLevel := w.config.WaDebug
	if minLevel == "" {
		minLevel = "INFO" // Nível mínimo para garantir que logs INFO apareçam
	}
	clientLog := waLog.Stdout("Client", minLevel, true)
	client := whatsmeow.NewClient(deviceStore, clientLog)

	// Publish the client only if this goroutine still owns the instance. A
	// concurrent start may have superseded us while the device store and the
	// version were being fetched; in that case the newer start owns it and this
	// one must not overwrite its client.
	if !w.clients.Attach(cd.Instance.Id, entry, client) {
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] Start superseded by a newer one, abandoning this attempt", cd.Instance.Id)
		return
	}

	if cd.IsProxy {
		var proxyConfig ProxyConfig
		// IsProxy is also set when only the global proxy is configured, and then
		// cd.Instance.Proxy is empty. Unmarshalling "" fails, and this used to
		// abort the whole connection — so the instance never came up at all.
		// The empty struct then correctly falls through to the global settings.
		if cd.Instance.Proxy != "" && cd.Instance.Proxy != "null" {
			if err := json.Unmarshal([]byte(cd.Instance.Proxy), &proxyConfig); err != nil {
				w.abortForProxyFailure(cd.Instance.Id, "unreadable proxy configuration", err)
				return
			}
		}

		proxyProtocol := proxyConfig.Protocol
		proxyHost := proxyConfig.Host
		proxyPort := proxyConfig.Port
		proxyUsername := proxyConfig.Username
		proxyPassword := proxyConfig.Password

		if proxyConfig.Host == "" {
			proxyHost = w.config.ProxyHost
		}

		if proxyConfig.Port == "" {
			proxyPort = w.config.ProxyPort
		}

		if proxyConfig.Protocol == "" {
			proxyProtocol = w.config.ProxyProtocol
		}

		if proxyConfig.Username == "" {
			proxyUsername = w.config.ProxyUsername
		}

		if proxyConfig.Password == "" {
			proxyPassword = w.config.ProxyPassword
		}

		// An instance configured with a proxy must never reach WhatsApp without
		// it: falling back to a direct connection would silently expose the
		// server's real IP, which is the one thing the proxy exists to hide.
		// So a proxy that cannot be built or applied aborts the connection.
		proxyAddress, err := utils.BuildProxyAddress(proxyProtocol, proxyHost, proxyPort, proxyUsername, proxyPassword)
		if err != nil {
			w.abortForProxyFailure(cd.Instance.Id, "invalid proxy configuration", err)
			return
		}

		if err = client.SetProxyAddress(proxyAddress); err != nil {
			w.abortForProxyFailure(cd.Instance.Id, "proxy could not be applied", err)
			return
		}

		w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Proxy enabled (%s)", cd.Instance.Id, utils.NormalizeProxyProtocol(proxyProtocol, proxyPort))
	} else {
		// No proxy for this instance: clear whatsmeow's default, which otherwise
		// picks up HTTP_PROXY/HTTPS_PROXY from the environment behind our back.
		client.SetProxy(nil)
	}

	client.EnableAutoReconnect = false
	client.AutoTrustIdentity = true

	mycli := &MyClient{
		service:            w,
		instance:           cd.Instance,
		WAClient:           client,
		eventHandlerID:     1,
		userID:             cd.Instance.Id,
		token:              cd.Instance.Token,
		subscriptions:      cd.Subscriptions,
		webhookUrl:         cd.Instance.Webhook,
		rabbitmqEnable:     cd.Instance.RabbitmqEnable,
		natsEnable:         cd.Instance.NatsEnable,
		websocketEnable:    cd.Instance.WebSocketEnable,
		instanceRepository: w.instanceRepository,
		messageRepository:  w.messageRepository,
		labelRepository:    w.labelRepository,
		pollService:        w.pollService, // NOVO: Serviço de enquetes
		userInfoCache:      w.userInfoCache,
		clients:            w.clients,
		entry:              entry,
		config:             w.config,
		historySyncID:      0,
		rabbitmqProducer:   w.rabbitmqProducer,
		webhookProducer:    w.webhookProducer,
		websocketProducer:  w.websocketProducer,
		mediaStorage:       w.mediaStorage,
		processedMessages:  w.processedMessages,
		natsProducer:       w.natsProducer,
		loggerWrapper:      w.loggerWrapper,
		qrcodeCount:        0,
		passkeyCeremony:    w.passkeyCeremony,
	}

	mycli.eventHandlerID = mycli.WAClient.AddEventHandler(mycli.myEventHandler)

	// Armazena o MyClient no map para permitir atualizações posteriores
	w.myClientPointer.set(cd.Instance.Id, mycli)

	if client.Store.ID != nil {
		w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Already logged in with JID: %s", cd.Instance.Id, client.Store.ID.String())
		// An already-paired device keeps the props it paired with, but still
		// needs the current handshake version.
		err = connectPaired(client, version)
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Erro de conexão WebSocket (EOF). Tentando reconectar em 5 segundos...", cd.Instance.Id)
				time.Sleep(5 * time.Second)
				err = connectPaired(client, version)
				if err != nil {
					w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Falha na segunda tentativa de conexão: %v", cd.Instance.Id, err)
					_ = w.instanceRepository.UpdateConnected(cd.Instance.Id, false, fmt.Sprintf("Connection failed after retry: %v", err))
					w.ScheduleAutoReconnect(cd.Instance.Id)
					return
				}
			} else {
				// Qualquer outra falha (incluindo proxy) encerra sem fallback.
				// Nunca conectamos sem proxy quando um proxy está configurado.
				w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to connect: %v", cd.Instance.Id, err)
				_ = w.instanceRepository.UpdateConnected(cd.Instance.Id, false, fmt.Sprintf("Connection failed: %v", err))
				// Connect() failed at the transport level (proxy down, DNS, timeout).
				// events.Disconnected never fires when no connection was established,
				// so schedule the bounded retry here instead of waiting for it.
				w.ScheduleAutoReconnect(cd.Instance.Id)
				return
			}
		}
	} else {
		qrChan, err := client.GetQRChannel(context.Background())
		if err != nil {
			if !errors.Is(err, whatsmeow.ErrQRStoreContainsID) {
				w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to get QR channel: %v", cd.Instance.Id, err)
				return
			}
		} else {
			// Pairing: this instance's OS name and version must be the ones in
			// the shared props when the handshake reads them.
			err = connectWithDeviceProps(client, osName, version)
			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Erro de conexão WebSocket (EOF). Tentando reconectar em 5 segundos...", cd.Instance.Id)
					time.Sleep(5 * time.Second)
					err = connectWithDeviceProps(client, osName, version)
					if err != nil {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Falha na segunda tentativa de conexão: %v", cd.Instance.Id, err)
						w.ScheduleAutoReconnect(cd.Instance.Id)
						return
					}
				} else {
					// Qualquer falha de proxy (auth, rede, etc.) encerra sem fallback.
					// Nunca conectamos sem proxy quando um proxy está configurado.
					w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to connect: %v", cd.Instance.Id, err)
					// Atualizar status para deixar claro que o proxy falhou
					_ = w.instanceRepository.UpdateConnected(cd.Instance.Id, false, fmt.Sprintf("Proxy connection failed: %v", err))
					w.ScheduleAutoReconnect(cd.Instance.Id)
					return
				}
			}

			for evt := range qrChan {
				w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Received QR code event %s", cd.Instance.Id, evt.Event)
				if evt.Event == "code" {
					// Incrementar contador de QR codes
					mycli.qrcodeCount++

					// Log com status do limite
					if w.config.QrcodeMaxCount > 0 {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] QR code generated #%d (max: %d)", cd.Instance.Id, mycli.qrcodeCount, w.config.QrcodeMaxCount)
					} else {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] QR code generated #%d (limit disabled)", cd.Instance.Id, mycli.qrcodeCount)
					}

					// Verificar se atingiu o limite máximo (0 = desabilitado)
					if w.config.QrcodeMaxCount > 0 && mycli.qrcodeCount >= w.config.QrcodeMaxCount {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] Maximum QR code count reached (%d), forcing logout and QRTimeout", cd.Instance.Id, w.config.QrcodeMaxCount)

						// 1. Forçar logout da instância
						if mycli.WAClient.IsConnected() {
							w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Forcing client logout due to QR limit", cd.Instance.Id)
							err := mycli.WAClient.Logout(context.Background())
							if err != nil {
								w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] Error during forced logout: %v", cd.Instance.Id, err)
							}
						}

						// 2. Limpar QR code no banco
						cd.Instance.Qrcode = ""
						err := w.instanceRepository.UpdateQrcode(cd.Instance.Id, "")
						if err != nil {
							w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Error clearing QR code: %v", cd.Instance.Id, err)
						}

						// 3. Atualizar status da instância como desconectada
						cd.Instance.Connected = false
						cd.Instance.DisconnectReason = fmt.Sprintf("QR code limit reached (%d)", w.config.QrcodeMaxCount)
						err = w.instanceRepository.UpdateConnected(cd.Instance.Id, false, cd.Instance.DisconnectReason)
						if err != nil {
							w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Error updating instance status: %v", cd.Instance.Id, err)
						}

						// 4. Limpar recursos
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] Cleaning up resources due to QR limit", cd.Instance.Id)
						w.clients.Remove(cd.Instance.Id)
						w.myClientPointer.remove(cd.Instance.Id)
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Stop signal sent due to QR limit", cd.Instance.Id)

						// 6. Enviar evento QRTimeout
						postMap := make(map[string]interface{})
						postMap["event"] = "QRTimeout"
						postMap["data"] = map[string]interface{}{
							"reason":      fmt.Sprintf("Maximum QR code count (%d) reached", w.config.QrcodeMaxCount),
							"qrcount":     mycli.qrcodeCount,
							"maxCount":    w.config.QrcodeMaxCount,
							"forceLogout": true,
						}
						postMap["instanceToken"] = mycli.token
						postMap["instanceId"] = mycli.userID
						postMap["instanceName"] = cd.Instance.Name

						queueName := strings.ToLower(fmt.Sprintf("%s.%s", cd.Instance.Id, postMap["event"]))
						values, err := json.Marshal(postMap)
						if err == nil {
							go w.CallWebhook(cd.Instance, queueName, values)
							if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
								go mycli.service.SendToGlobalQueues(postMap["event"].(string), values, mycli.userID)
							}
						}

						w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] QRTimeout event sent due to QR limit enforcement", cd.Instance.Id)
						return
					}

					if w.config.LogType != "json" {
						fmt.Println("QR code:\n", evt.Code)
					}

					image, _ := qrcode.Encode(evt.Code, qrcode.Medium, 256)
					base64qrcode := "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)

					base64WithCode := base64qrcode + "|" + evt.Code

					cd.Instance.Qrcode = base64WithCode

					err := w.instanceRepository.UpdateQrcode(cd.Instance.Id, base64WithCode)
					if err != nil {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Error updating instance: %s", cd.Instance.Id, err)
					}

					postMap := make(map[string]interface{})

					postMap["event"] = "QRCode"

					dataMap := make(map[string]interface{})

					dataMap["qrcode"] = base64qrcode
					dataMap["code"] = evt.Code
					dataMap["count"] = mycli.qrcodeCount
					dataMap["maxCount"] = w.config.QrcodeMaxCount

					postMap["data"] = dataMap

					postMap["instanceToken"] = mycli.token
					postMap["instanceId"] = mycli.userID
					postMap["instanceName"] = cd.Instance.Name

					var queueName string

					if _, ok := postMap["event"]; ok {
						queueName = strings.ToLower(fmt.Sprintf("%s.%s", cd.Instance.Id, postMap["event"]))
					}

					values, err := json.Marshal(postMap)
					if err != nil {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to marshal JSON for queue", cd.Instance.Id)
						return
					}

					go w.CallWebhook(cd.Instance, queueName, values)

					if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
						go mycli.service.SendToGlobalQueues(postMap["event"].(string), values, mycli.userID)
					}
				} else if evt.Event == "timeout" {
					cd.Instance.Qrcode = ""

					err := w.instanceRepository.UpdateQrcode(cd.Instance.Id, "")
					if err != nil {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Error updating instance: %s", cd.Instance.Id, err)
					}

					// This used to send on an unbuffered channel whose only
					// receiver is the select loop further down THIS goroutine —
					// so it blocked forever and the QR loop never exited,
					// stranding the instance until restart. Remove closes the
					// entry instead, which never blocks.
					w.loggerWrapper.GetLogger(cd.Instance.Id).LogWarn("[%s] QR timeout, stopping the client", cd.Instance.Id)
					w.clients.Remove(cd.Instance.Id)
					w.myClientPointer.remove(cd.Instance.Id)

					postMap := make(map[string]interface{})

					postMap["event"] = "QRTimeout"

					dataMap := make(map[string]interface{})

					postMap["data"] = dataMap

					postMap["instanceToken"] = mycli.token
					postMap["instanceId"] = mycli.userID
					postMap["instanceName"] = cd.Instance.Name

					var queueName string

					if _, ok := postMap["event"]; ok {
						queueName = strings.ToLower(fmt.Sprintf("%s.%s", cd.Instance.Id, postMap["event"]))
					}

					values, err := json.Marshal(postMap)
					if err != nil {
						w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to marshal JSON for queue", cd.Instance.Id)
						return
					}

					go w.CallWebhook(cd.Instance, queueName, values)

					if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
						go mycli.service.SendToGlobalQueues(postMap["event"].(string), values, mycli.userID)
					}
				} else if evt.Event == "success" {
					w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] QR pairing ok!", cd.Instance.Id)
				} else {
					w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] Login event: %s", cd.Instance.Id, evt.Event)
				}
			}
		}
	}

	// Wait on the entry claimed at the top of this function, not on a lookup:
	// if the instance has since been restarted, the newer StartClient owns the
	// map slot and this goroutine must still notice its OWN stop signal.
	for {
		select {
		case <-entry.Done():
			w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("Received kill signal for user '%s'", cd.Instance.Id)
			client.Disconnect()

			// Only tear the instance down if this goroutine still owns it. If a
			// newer StartClient took over, that one owns the registry slot and
			// the MyClient, and clearing them here would leave the instance
			// looking started with nothing serving it.
			if w.clients.RemoveOwned(cd.Instance.Id, entry) {
				w.myClientPointer.remove(cd.Instance.Id)
			}

			// Limpar cache de userInfo para esta instância
			w.userInfoCache.Delete(cd.Instance.Token)
			w.loggerWrapper.GetLogger(cd.Instance.Id).LogInfo("[%s] UserInfo cache cleared for token: %s", cd.Instance.Id, cd.Instance.Token)

			cd.Instance.Connected = false

			err := w.instanceRepository.UpdateConnected(cd.Instance.Id, cd.Instance.Connected, cd.Instance.DisconnectReason)
			if err != nil {
				w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Error updating instance: %s", cd.Instance.Id, err)
			}

			postMap := make(map[string]interface{})

			postMap["event"] = "LoggedOut"

			dataMap := make(map[string]interface{})

			dataMap["reason"] = "Logged out"

			postMap["data"] = dataMap

			postMap["instanceToken"] = mycli.token
			postMap["instanceId"] = mycli.userID
			postMap["instanceName"] = cd.Instance.Name

			var queueName string

			if _, ok := postMap["event"]; ok {
				queueName = strings.ToLower(fmt.Sprintf("%s.%s", cd.Instance.Id, postMap["event"]))
			}

			values, err := json.Marshal(postMap)
			if err != nil {
				w.loggerWrapper.GetLogger(cd.Instance.Id).LogError("[%s] Failed to marshal JSON for queue", cd.Instance.Id)
				return
			}

			go w.CallWebhook(cd.Instance, queueName, values)

			if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
				go mycli.service.SendToGlobalQueues(postMap["event"].(string), values, mycli.userID)
			}

			return
		case <-time.After(1 * time.Second):
			// Continua aguardando sinal de kill
		}
	}
}

func schedulePresenceUpdates(mycli *MyClient) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Verificar se a instância ainda existe
			_, err := mycli.instanceRepository.GetInstanceByID(mycli.userID)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Instance no longer exists, stopping presence updates", mycli.userID)
				return // Encerra a goroutine se a instância não existir mais
			}

			processPresenceUpdates(mycli)

			ticker.Stop()
			randomInterval := time.Duration(1+rand.Intn(3)) * time.Hour
			ticker = time.NewTicker(randomInterval)

		case <-mycli.entry.Done():
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Received stop signal, stopping presence updates", mycli.userID)
			return // Encerra a goroutine quando receber sinal de kill
		}
	}
}

func processPresenceUpdates(mycli *MyClient) {
	// Second guard on the same rule as the Connected handler: this loop is only
	// started for alwaysOnline instances, but the setting can be turned off
	// while it is running, and going "available" again would re-silence the
	// operator's phone.
	if !mycli.Instance().AlwaysOnline {
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] alwaysOnline is off, skipping presence refresh", mycli.userID)
		return
	}

	now := time.Now()
	location, _ := time.LoadLocation("America/Sao_Paulo")
	nowSp := now.In(location)

	if nowSp.Hour() >= 1 && nowSp.Hour() < 24 {
		err := mycli.WAClient.SendPresence(context.Background(), types.PresenceUnavailable)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to set presence as unavailable %v", mycli.userID, err)
		} else {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Marked self as unavailable", mycli.userID)
		}

		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Second)

		err = mycli.WAClient.SendPresence(context.Background(), types.PresenceAvailable)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to set presence as available %v", mycli.userID, err)
		} else {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Marked self as available", mycli.userID)
		}
	}
}

// ensureNCTSalt makes sure the account-wide NCT salt is stored. The salt is needed to
// derive <cstoken> for cold contacts and avoid WhatsApp error 463. It normally arrives in
// the initial history sync / app-state sync, but instances paired before this feature existed
// won't get it via incremental sync — so when it's missing we force a one-time full resync of
// the regular app-state patches (which carry the nct_salt_sync action).
func (mycli *MyClient) ensureNCTSalt() {
	client := mycli.WAClient
	if client == nil || client.Store == nil || client.Store.NCTSalt == nil {
		return
	}
	go func() {
		ctx := context.Background()
		salt, err := client.Store.NCTSalt.GetNCTSalt(ctx)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to read NCT salt: %v", mycli.userID, err)
			return
		}
		if len(salt) > 0 {
			return
		}
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] NCT salt missing, forcing full app-state resync to fetch it", mycli.userID)
		for _, name := range appstate.AllPatchNames {
			if err := client.FetchAppState(ctx, name, true, false); err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Full app-state resync of %s failed: %v", mycli.userID, name, err)
			}
		}
		if salt, err := client.Store.NCTSalt.GetNCTSalt(ctx); err == nil && len(salt) > 0 {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] NCT salt acquired after full resync (%d bytes)", mycli.userID, len(salt))
		} else {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] NCT salt still missing after full resync", mycli.userID)
		}
	}()
}

// AccountLimitsCacheEntry holds the last successfully fetched WhatsApp account limits
// for an instance. The MEX queries can be slow/rate-limited, so they are fetched once on
// connect and cached here for the /instance/limits endpoint to serve instantly.
type AccountLimitsCacheEntry struct {
	ReachoutActive bool
	ReachoutEnds   int64 // unix seconds
	ReachoutType   string
	CappingStatus  string
	TotalQuota     int
	UsedQuota      int
	CycleEnds      int64 // unix seconds
	FetchedAt      time.Time
}

var accountLimitsCache sync.Map // instanceID(string) -> *AccountLimitsCacheEntry

// GetCachedAccountLimits returns the last fetched account limits for an instance, if any.
func GetCachedAccountLimits(instanceID string) (*AccountLimitsCacheEntry, bool) {
	v, ok := accountLimitsCache.Load(instanceID)
	if !ok {
		return nil, false
	}
	return v.(*AccountLimitsCacheEntry), true
}

// logAccountLimits queries WhatsApp's MEX endpoints for the account's new-chat message
// capping and reachout timelock state, logs them, and caches the result. Error 463 on sends
// to NEW contacts is caused by these account-level limits, not by local code.
func (mycli *MyClient) logAccountLimits() {
	client := mycli.WAClient
	if client == nil {
		return
	}
	go func() {
		ctx := context.Background()
		entry := &AccountLimitsCacheEntry{FetchedAt: time.Now()}
		got := false
		if capInfo, err := walimits.GetNewChatMessageCappingInfo(ctx, client); err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to fetch new-chat message capping info: %v", mycli.userID, err)
		} else if capInfo != nil {
			entry.CappingStatus = string(capInfo.CappingStatus)
			entry.TotalQuota = capInfo.TotalQuota
			entry.UsedQuota = capInfo.UsedQuota
			entry.CycleEnds = capInfo.CycleEndTimestamp.Unix()
			got = true
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] NEW-CHAT CAPPING: status=%s used=%d/%d cycleEnds=%s ote=%s mv=%s",
				mycli.userID, capInfo.CappingStatus, capInfo.UsedQuota, capInfo.TotalQuota, capInfo.CycleEndTimestamp.Time, capInfo.OTEStatus, capInfo.MVStatus)
		}
		if tl, err := walimits.GetAccountReachoutTimelock(ctx, client); err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to fetch reachout timelock: %v", mycli.userID, err)
		} else if tl != nil {
			entry.ReachoutActive = tl.IsActive
			if tl.IsActive {
				entry.ReachoutEnds = tl.TimeEnforcementEnds.Unix()
			}
			entry.ReachoutType = string(tl.EnforcementType)
			got = true
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] REACHOUT TIMELOCK: active=%t ends=%s type=%s",
				mycli.userID, tl.IsActive, tl.TimeEnforcementEnds.Time, tl.EnforcementType)
		}
		if got {
			accountLimitsCache.Store(mycli.userID, entry)
		}
	}()
}

func (mycli *MyClient) myEventHandler(rawEvt interface{}) {
	userID := mycli.userID
	postMap := make(map[string]interface{})
	postMap["data"] = rawEvt
	doWebhook := false

	switch evt := rawEvt.(type) {
	case *events.AppStateSyncComplete:
		if len(mycli.WAClient.Store.PushName) > 0 && evt.Name == appstate.WAPatchCriticalBlock {
			err := mycli.WAClient.SendPresence(context.Background(), types.PresenceUnavailable)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to send unavailable presence %v", mycli.userID, err)
			} else {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Marked self as unavailable", mycli.userID)
			}
		}
	case *events.Connected, *events.PushNameSetting:
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] events.Connected to Whatsapp for user '%s'", mycli.userID, mycli.WAClient.Store.PushName)
		// The connection recovered — refill the automatic reconnection budget so
		// a later drop gets a fresh set of attempts.
		mycli.service.ResetAutoReconnect(mycli.userID)
		// Self-heal: ensure the account-wide NCT salt is present so <cstoken> can be
		// derived for cold contacts (fixes error 463). Already-paired instances won't
		// receive it via incremental app-state sync, so force a one-time full resync
		// of the regular patches when it's missing.
		mycli.ensureNCTSalt()
		// Diagnostic: log WhatsApp's own account limits (new-chat quota + reachout
		// timelock). When these are exhausted/active, sends to NEW contacts get 463
		// regardless of tokens — this surfaces exactly why and until when.
		mycli.logAccountLimits()
		if len(mycli.WAClient.Store.PushName) > 0 {
			doWebhook = true
			postMap["event"] = "Connected"

			if postMap["data"] != nil {
				jsonBytes, err := json.Marshal(postMap["data"])
				if err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal postMap['data']: %v", mycli.userID, err)
					return
				}

				var dataMap map[string]interface{}
				err = json.Unmarshal(jsonBytes, &dataMap)
				if err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to unmarshal postMap['data'] to map[string]interface{}: %v", mycli.userID, err)
					return
				}

				postMap["data"] = dataMap
			} else {
				postMap["data"] = make(map[string]interface{})
			}

			dataMap := postMap["data"].(map[string]interface{})

			dataMap["status"] = "open"
			dataMap["jid"] = mycli.WAClient.Store.ID.String()
			dataMap["pushName"] = mycli.WAClient.Store.PushName

			// jid, ok := utils.ParseJID(mycli.WAClient.Store.ID.ToNonAD().User)
			// if ok {
			// 	profilePicUrl, err := mycli.clientPointer[mycli.userID].GetProfilePictureInfo(jid, &whatsmeow.GetProfilePictureParams{
			// 		Preview: false,
			// 	})
			// 	if err != nil {
			// 		w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to get profile picture info: %v", mycli.userID, err)
			// 	} else {
			// 		dataMap["profilePicUrl"] = profilePicUrl.URL
			// 	}
			// }

			postMap["data"] = dataMap

			// Presence decides whether the phone still notifies. WhatsApp stops
			// pushing to the handset while any linked device reports the account
			// as online, because it assumes the messages are being read there.
			//
			// The AlwaysOnline setting exists for operators who want that, but it
			// was never consulted here: every instance announced itself available
			// and silenced its own phone. Honour the setting, and default to
			// unavailable so notifications keep working.
			if mycli.Instance().AlwaysOnline {
				go schedulePresenceUpdates(mycli)

				if err := mycli.WAClient.SendPresence(context.Background(), types.PresenceAvailable); err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to send available presence %v", mycli.userID, err)
				} else {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Marked self as available (alwaysOnline enabled)", mycli.userID)
				}
			} else {
				if err := mycli.WAClient.SendPresence(context.Background(), types.PresenceUnavailable); err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to send unavailable presence %v", mycli.userID, err)
				} else {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Marked self as unavailable so the phone keeps notifying", mycli.userID)
				}
			}

			mycli.Instance().Connected = true
			mycli.Instance().DisconnectReason = ""
			if err := mycli.instanceRepository.UpdateConnected(mycli.Instance().Id, mycli.Instance().Connected, mycli.Instance().DisconnectReason); err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error updating instance: %s", mycli.Instance().Id, err)
			}

			if err := mycli.instanceRepository.UpdateQrcode(mycli.Instance().Id, ""); err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error updating instance: %s", mycli.Instance().Id, err)
			}
		}
	case *events.PairPasskeyRequest:
		// WhatsApp requires a passkey (WebAuthn / Shortcake / CRSC) to link this
		// account. Start a browser-driven ceremony; the manager/extension polls it.
		if mycli.passkeyCeremony != nil {
			if pk, err := json.Marshal(evt.PublicKey); err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal passkey public key: %v", mycli.userID, err)
				mycli.passkeyCeremony.SetError(mycli.userID, err.Error())
			} else {
				token := mycli.passkeyCeremony.Start(mycli.userID, pk)
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Passkey pairing required — ceremony started (token=%s)", mycli.userID, token)
				doWebhook = true
				postMap["event"] = "PasskeyRequest"
				postMap["data"] = map[string]interface{}{"token": token}
			}
		}

	case *events.PairPasskeyConfirmation:
		// Confirmation code available. We never auto-confirm (SkipHandoffUX forced
		// false) — the user must verify the code and confirm via the extension.
		if mycli.passkeyCeremony != nil {
			mycli.passkeyCeremony.SetConfirmation(mycli.userID, evt.Code, false)
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Passkey confirmation code ready: %s", mycli.userID, evt.Code)
		}

	case *events.PairPasskeyError:
		if mycli.passkeyCeremony != nil {
			mycli.passkeyCeremony.SetError(mycli.userID, evt.Error.Error())
		}
		mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Passkey pairing error (continuation=%t): %v", mycli.userID, evt.Continuation, evt.Error)

	case *events.PairSuccess:
		doWebhook = true
		postMap["event"] = "PairSuccess"
		if mycli.passkeyCeremony != nil {
			mycli.passkeyCeremony.Clear(mycli.userID)
		}
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("QR Pair Success for user '%s' with JID '%s' - '%s'", mycli.userID, evt.ID.String(), mycli.WAClient.Store.ID.String())

		instance, err := mycli.instanceRepository.GetInstanceByID(mycli.userID)
		if err != nil {
			// Without a row there is nothing to write the new JID to, and the
			// next line would dereference nil. That used to panic exactly when
			// the database was already struggling — losing the pairing along
			// with the process. The device is paired regardless; the row is
			// reconciled on the next Connected event.
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error getting instance: %s", mycli.userID, err)
			return
		}

		instance.Qrcode = ""
		instance.Connected = true
		instance.DisconnectReason = ""
		instance.Jid = mycli.WAClient.Store.ID.String()

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Updating JID: %s in Instance: %s", mycli.userID, mycli.WAClient.Store.ID.String(), instance.Jid)

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Attempting to update instance in DB: %+v", mycli.userID, instance)
		err = mycli.instanceRepository.Update(instance)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error updating instance: %s", mycli.userID, err)
		} else {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Instance successfully updated", mycli.userID)
		}

		myUserInfo, found := mycli.userInfoCache.Get(mycli.token)

		if !found {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] No user info cached on pairing?", mycli.userID)
		} else {
			txtid := myUserInfo.(Values).Get("Id")
			token := myUserInfo.(Values).Get("Token")

			updatedUserInfo := utils.UpdateUserInfo(myUserInfo, "Jid", evt.ID.String())

			mycli.userInfoCache.Set(token, updatedUserInfo, cache.NoExpiration)
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] User information set for user '%s'", mycli.userID, txtid)
		}

		if postMap["data"] != nil {
			jsonBytes, err := json.Marshal(postMap["data"])
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal postMap['data']: %v", mycli.userID, err)
				return
			}

			var dataMap map[string]interface{}
			err = json.Unmarshal(jsonBytes, &dataMap)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to unmarshal postMap['data'] to map[string]interface{}: %v", mycli.userID, err)
				return
			}

			postMap["data"] = dataMap
		} else {
			postMap["data"] = make(map[string]interface{})
		}

		dataMap := postMap["data"].(map[string]interface{})

		dataMap["status"] = "open"
		dataMap["jid"] = mycli.WAClient.Store.ID.String()

		if mycli.WAClient.Store.PushName != "" {
			dataMap["pushName"] = mycli.WAClient.Store.PushName
		}

		postMap["data"] = dataMap
	case *events.StreamReplaced:
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Received StreamReplaced event", mycli.userID)
		return
	case *events.TemporaryBan:
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] User received temporary ban for %s", mycli.userID, evt.Code.String())
		doWebhook = true
		postMap["event"] = "TemporaryBan"

		if postMap["data"] != nil {
			jsonBytes, err := json.Marshal(postMap["data"])
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal postMap['data']: %v", mycli.userID, err)
				return
			}

			var dataMap map[string]interface{}
			err = json.Unmarshal(jsonBytes, &dataMap)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to unmarshal postMap['data'] to map[string]interface{}: %v", mycli.userID, err)
				return
			}

			postMap["data"] = dataMap
		} else {
			postMap["data"] = make(map[string]interface{})
		}

		dataMap := postMap["data"].(map[string]interface{})

		dataMap["reason"] = evt.Code.String()
		dataMap["expire"] = evt.Expire

		postMap["data"] = dataMap
	case *events.Message:
		doWebhook = true
		postMap["event"] = "Message"
		// Message received

		// Log message arrival with detailed info
		messageSize := "unknown"
		if evt.Message.GetDocumentMessage() != nil && evt.Message.GetDocumentMessage().FileLength != nil {
			messageSize = fmt.Sprintf("%d bytes", *evt.Message.GetDocumentMessage().FileLength)
		} else if evt.Message.GetVideoMessage() != nil && evt.Message.GetVideoMessage().FileLength != nil {
			messageSize = fmt.Sprintf("%d bytes", *evt.Message.GetVideoMessage().FileLength)
		} else if evt.Message.GetImageMessage() != nil && evt.Message.GetImageMessage().FileLength != nil {
			messageSize = fmt.Sprintf("%d bytes", *evt.Message.GetImageMessage().FileLength)
		} else if evt.Message.GetAudioMessage() != nil && evt.Message.GetAudioMessage().FileLength != nil {
			messageSize = fmt.Sprintf("%d bytes", *evt.Message.GetAudioMessage().FileLength)
		}

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] ===== MESSAGE RECEIVED ===== ID: %s, From: %s, Type: %s, Size: %s", mycli.userID, evt.Info.ID, evt.Info.Chat.String(), evt.Info.Type, messageSize)

		// Persist the message text so it can be searched later (MCP tools).
		// Done before the ignore-group/ignore-status filters below so the archive
		// stays complete even when those events are not forwarded to webhooks.
		mycli.archiveIncomingMessage(evt)

		// se readMessages for true ele marca como lida
		if mycli.Instance().ReadMessages {
			messageIDs := []string{evt.Info.ID}
			err := mycli.WAClient.MarkRead(context.Background(), messageIDs, time.Now(), evt.Info.Sender, evt.Info.Sender)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to auto-mark message as read: %v", mycli.userID, err)
			} else {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Auto-marked message as read from %s", mycli.userID, evt.Info.Chat.String())
			}
		}

		// se ignoreStatus for true e o chat for broadcast ou o id for broadcast retorna
		if mycli.Instance().IgnoreStatus && (strings.Contains(evt.Info.Chat.String(), "@broadcast") || strings.Contains(evt.Info.ID, "@broadcast")) {
			return
		}

		// se ignoreGroup for true e o chat for grupo retorna
		if mycli.Instance().IgnoreGroups && strings.Contains(evt.Info.Chat.String(), "@g.us") {
			return
		}

		// Verifica advanced settings para ignorar grupos
		if (mycli.config.EventIgnoreGroup || mycli.Instance().IgnoreGroups) && strings.Contains(evt.Info.Chat.String(), "@g.us") {
			return
		}

		// Verifica advanced settings para ignorar status/broadcast
		if (mycli.config.EventIgnoreStatus || mycli.Instance().IgnoreStatus) && (strings.Contains(evt.Info.Chat.String(), "@broadcast") || strings.Contains(evt.Info.ID, "@broadcast")) {
			return
		}

		// Trata o caso especial onde Sender é @lid e SenderAlt é @s.whatsapp.net
		// Neste caso, devemos inverter: Sender e Chat devem ser @s.whatsapp.net, SenderAlt deve ser @lid
		senderStr := evt.Info.Sender.String()
		senderAltStr := evt.Info.SenderAlt.String()
		chatStr := evt.Info.Chat.String()

		if strings.Contains(senderStr, "@lid") && strings.Contains(senderAltStr, "@s.whatsapp.net") {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Detected LID/WhatsApp JID swap case - Sender: %s, SenderAlt: %s", mycli.userID, senderStr, senderAltStr)

			// Limpa os IDs antes de fazer a troca
			cleanSenderAlt := cleanSenderID(senderAltStr)
			cleanSender := cleanSenderID(senderStr)

			// Inverte: Sender e Chat recebem o @s.whatsapp.net, SenderAlt recebe o @lid
			if cleanedWhatsAppJID, err := types.ParseJID(cleanSenderAlt); err == nil {
				evt.Info.Sender = cleanedWhatsAppJID
				// Se Chat também é @lid, atualiza para @s.whatsapp.net
				if strings.Contains(chatStr, "@lid") {
					evt.Info.Chat = cleanedWhatsAppJID
				}
			}

			if cleanedLID, err := types.ParseJID(cleanSender); err == nil {
				evt.Info.SenderAlt = cleanedLID
			}

			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] JID swap completed - New Sender: %s, New SenderAlt: %s, New Chat: %s",
				mycli.userID, evt.Info.Sender.String(), evt.Info.SenderAlt.String(), evt.Info.Chat.String())
		} else {
			// Comportamento normal: apenas limpa os IDs
			cleanSender := cleanSenderID(senderStr)
			if cleanedJID, err := types.ParseJID(cleanSender); err == nil {
				evt.Info.Sender = cleanedJID
			}

			cleanSenderAlt := cleanSenderID(senderAltStr)
			if cleanedLID, err := types.ParseJID(cleanSenderAlt); err == nil {
				evt.Info.SenderAlt = cleanedLID
			}
		}

		// Auto-marca mensagens como lidas se configurado
		if mycli.Instance().ReadMessages && !evt.Info.IsFromMe {
			go func() {
				time.Sleep(1 * time.Second) // Pequeno delay para parecer mais natural
				err := mycli.WAClient.MarkRead(context.Background(), []types.MessageID{evt.Info.ID}, evt.Info.Timestamp, evt.Info.Chat, evt.Info.Sender)
				if err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to auto-mark message as read: %v", mycli.userID, err)
				} else {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Auto-marked message as read from %s", mycli.userID, evt.Info.Chat.String())
				}
			}()
		}

		parsedMessageType := utils.GetMessageType(evt.Message)
		if parsedMessageType == "ignore" || strings.HasPrefix(parsedMessageType, "unknown_protocol_") {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Message ignored because it's a unknown protocol message", mycli.userID)
			return
		}

		if postMap["data"] != nil {
			jsonBytes, err := json.Marshal(postMap["data"])
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal postMap['data']: %v", mycli.userID, err)
				return
			}

			var dataMap map[string]interface{}
			err = json.Unmarshal(jsonBytes, &dataMap)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to unmarshal postMap['data'] to map[string]interface{}: %v", mycli.userID, err)
				return
			}

			postMap["data"] = dataMap
		} else {
			postMap["data"] = make(map[string]interface{})
		}

		dataMap, ok := postMap["data"].(map[string]interface{})
		if !ok {
			dataMap = make(map[string]interface{})
		}

		if evt.Message.GetPollUpdateMessage() != nil {
			fmt.Printf("[POLL DEBUG] 🎯 PollUpdateMessage detected!\n")
			fmt.Printf("[POLL DEBUG] � BEFORE accessing evt.Info - Sender: %s, Server: %s\n", evt.Info.Sender.String(), evt.Info.Sender.Server)
			fmt.Printf("[POLL DEBUG] 📍 BEFORE accessing evt.Info - SenderAlt: %s\n", evt.Info.SenderAlt.String())
			fmt.Printf("[POLL DEBUG] �� mycli.WAClient is nil: %v\n", mycli.WAClient == nil)
			if mycli.WAClient != nil {
				fmt.Printf("[POLL DEBUG] ✅ mycli.WAClient is initialized: %s\n", mycli.WAClient.Store.ID)
			}

			decrypted, err := mycli.WAClient.DecryptPollVote(context.Background(), evt)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to decrypt vote: %v", mycli.userID, err)
			} else {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Selected options in decrypted vote:", mycli.userID)
				for _, option := range decrypted.SelectedOptions {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("- %X", option)

				}

				// NOVO: Salvar voto no banco de dados de forma NÃO-INVASIVA
				if mycli.pollService != nil {
					go func() {
						defer func() {
							if r := recover(); r != nil {
								mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Panic ao salvar voto: %v", mycli.userID, r)
							}
						}()

						pollKey := evt.Message.GetPollUpdateMessage().GetPollCreationMessageKey()
						if pollKey == nil {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] PollCreationMessageKey not found", mycli.userID)
							return
						}

						pollInfo := &types.MessageInfo{
							ID: pollKey.GetID(),
							MessageSource: types.MessageSource{
								Chat: evt.Info.Chat, // Usar o chat do evento atual
							},
						}

						// Construir modelo de voto usando helper seguro
						// evt.Info já passou pelo JID swap, então Sender = número real
						pollVote := poll_service.BuildPollVoteFromEvent(
							pollInfo,
							&evt.Info,
							decrypted,
							"", // CompanyID não disponível no MyClient, será vazio
							mycli.Instance().Id,
						)

						// Salvar no banco com timeout de segurança
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()

						if err := mycli.pollService.SavePollVote(ctx, pollVote); err != nil {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to save poll vote to database: %v", mycli.userID, err)
						} else {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Poll vote saved to database successfully", mycli.userID)
						}
					}()
				}
			}
		}

		var quotedMessage *waE2E.Message
		var stanzaID string

		if evt.Message.GetExtendedTextMessage() != nil {
			quotedMessage = evt.Message.GetExtendedTextMessage().GetContextInfo().GetQuotedMessage()
			stanzaID = evt.Message.GetExtendedTextMessage().GetContextInfo().GetStanzaID()
		} else if evt.Message.GetImageMessage() != nil {
			quotedMessage = evt.Message.GetImageMessage().GetContextInfo().GetQuotedMessage()
			stanzaID = evt.Message.GetImageMessage().GetContextInfo().GetStanzaID()
		} else if evt.Message.GetAudioMessage() != nil {
			quotedMessage = evt.Message.GetAudioMessage().GetContextInfo().GetQuotedMessage()
			stanzaID = evt.Message.GetAudioMessage().GetContextInfo().GetStanzaID()
		} else if evt.Message.GetDocumentMessage() != nil {
			quotedMessage = evt.Message.GetDocumentMessage().GetContextInfo().GetQuotedMessage()
			stanzaID = evt.Message.GetDocumentMessage().GetContextInfo().GetStanzaID()
		} else if evt.Message.GetVideoMessage() != nil {
			quotedMessage = evt.Message.GetVideoMessage().GetContextInfo().GetQuotedMessage()
			stanzaID = evt.Message.GetVideoMessage().GetContextInfo().GetStanzaID()
		}

		if stanzaID != "" && quotedMessage != nil {
			quotedMap := make(map[string]interface{})

			quotedMap["stanzaID"] = stanzaID
			quotedMap["quotedMessage"] = quotedMessage

			dataMap["quoted"] = quotedMap
			dataMap["isQuoted"] = true
		}

		if mycli.config.WebhookFiles {
			isMedia := false

			img := evt.Message.GetImageMessage()
			audio := evt.Message.GetAudioMessage()
			document := evt.Message.GetDocumentMessage()
			video := evt.Message.GetVideoMessage()
			sticker := evt.Message.GetStickerMessage()

			// Check for associated child messages (like media in replies)
			var associatedImg *waE2E.ImageMessage
			var associatedAudio *waE2E.AudioMessage
			var associatedDocument *waE2E.DocumentMessage
			var associatedVideo *waE2E.VideoMessage
			var associatedSticker *waE2E.StickerMessage

			if evt.Message.GetAssociatedChildMessage() != nil {
				childMsg := evt.Message.GetAssociatedChildMessage().GetMessage()
				if childMsg != nil {
					associatedImg = childMsg.GetImageMessage()
					associatedAudio = childMsg.GetAudioMessage()
					associatedDocument = childMsg.GetDocumentMessage()
					associatedVideo = childMsg.GetVideoMessage()
					associatedSticker = childMsg.GetStickerMessage()
				}
			}

			if img != nil || audio != nil || document != nil || video != nil || sticker != nil ||
				associatedImg != nil || associatedAudio != nil || associatedDocument != nil ||
				associatedVideo != nil || associatedSticker != nil {
				isMedia = true
			}

			if isMedia {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing media message - ID: %s", mycli.userID, evt.Info.ID)

				var data []byte
				var err error
				var extension string
				var mimeType string
				var mediaSize int64

				// Create context with timeout for large files
				downloadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				downloadStart := time.Now()

				// Handle regular media messages
				if img != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Downloading image - ID: %s", mycli.userID, evt.Info.ID)
					data, err = mycli.WAClient.Download(downloadCtx, img)
					extension = ".jpg"
					mimeType = "image/jpeg"
					if img.FileLength != nil {
						mediaSize = int64(*img.FileLength)
					}
				} else if audio != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Downloading audio - ID: %s", mycli.userID, evt.Info.ID)
					data, err = mycli.WAClient.Download(downloadCtx, audio)
					extension = ".ogg"
					mimeType = "audio/ogg"
					if audio.FileLength != nil {
						mediaSize = int64(*audio.FileLength)
					}
				} else if document != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Downloading document - ID: %s, FileName: %s, Size: %d bytes", mycli.userID, evt.Info.ID, document.GetFileName(), document.GetFileLength())
					data, err = mycli.WAClient.Download(downloadCtx, document)
					extension = getExtensionFromMimeType(document.GetMimetype())
					mimeType = document.GetMimetype()
					if document.FileLength != nil {
						mediaSize = int64(*document.FileLength)
					}
				} else if video != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Downloading video - ID: %s, Size: %d bytes", mycli.userID, evt.Info.ID, video.GetFileLength())
					data, err = mycli.WAClient.Download(downloadCtx, video)
					extension = ".mp4"
					mimeType = "video/mp4"
					if video.FileLength != nil {
						mediaSize = int64(*video.FileLength)
					}
				} else if sticker != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Downloading sticker - ID: %s", mycli.userID, evt.Info.ID)
					data, err = mycli.WAClient.Download(downloadCtx, sticker)
					extension = ".png"
					mimeType = "image/png"
					if sticker.FileLength != nil {
						mediaSize = int64(*sticker.FileLength)
					}

					if err == nil {
						webpReader := bytes.NewReader(data)
						img, decErr := webp.Decode(webpReader)
						if decErr != nil {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to decode webp sticker, keeping raw webp: %v", mycli.userID, decErr)
							extension = ".webp"
							mimeType = "image/webp"
						} else {
							var pngBuffer bytes.Buffer
							if encErr := png.Encode(&pngBuffer, img); encErr != nil {
								mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to encode png from sticker, keeping raw webp: %v", mycli.userID, encErr)
								extension = ".webp"
								mimeType = "image/webp"
							} else {
								data = pngBuffer.Bytes()
							}
						}
					}
					// Handle associated child media messages
				} else if associatedImg != nil {
					data, err = mycli.WAClient.Download(context.Background(), associatedImg)
					extension = ".jpg"
					mimeType = "image/jpeg"
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing associated child image message", mycli.userID)
				} else if associatedAudio != nil {
					data, err = mycli.WAClient.Download(context.Background(), associatedAudio)
					extension = ".ogg"
					mimeType = "audio/ogg"
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing associated child audio message", mycli.userID)
				} else if associatedDocument != nil {
					data, err = mycli.WAClient.Download(context.Background(), associatedDocument)
					extension = getExtensionFromMimeType(associatedDocument.GetMimetype())
					mimeType = associatedDocument.GetMimetype()
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing associated child document message", mycli.userID)
				} else if associatedVideo != nil {
					data, err = mycli.WAClient.Download(context.Background(), associatedVideo)
					extension = ".mp4"
					mimeType = "video/mp4"
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing associated child video message", mycli.userID)
				} else if associatedSticker != nil {
					data, err = mycli.WAClient.Download(context.Background(), associatedSticker)
					extension = ".png"
					mimeType = "image/png"
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing associated child sticker message", mycli.userID)

					if err == nil {
						webpReader := bytes.NewReader(data)
						img, decErr := webp.Decode(webpReader)
						if decErr != nil {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to decode webp sticker, keeping raw webp: %v", mycli.userID, decErr)
							extension = ".webp"
							mimeType = "image/webp"
						} else {
							var pngBuffer bytes.Buffer
							if encErr := png.Encode(&pngBuffer, img); encErr != nil {
								mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Failed to encode png from associated sticker, keeping raw webp: %v", mycli.userID, encErr)
								extension = ".webp"
								mimeType = "image/webp"
							} else {
								data = pngBuffer.Bytes()
							}
						}
					}
				}

				downloadDuration := time.Since(downloadStart)

				if err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to download media - ID: %s, Size: %d bytes, Duration: %v, Error: %v", mycli.userID, evt.Info.ID, mediaSize, downloadDuration, err)

					// Check if it's a timeout error
					if downloadCtx.Err() == context.DeadlineExceeded {
						mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Download timeout exceeded (5 minutes) for large file - ID: %s, Size: %d bytes", mycli.userID, evt.Info.ID, mediaSize)
					}

					// Don't return here - continue processing the message without media
					mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Continuing message processing without media download - ID: %s", mycli.userID, evt.Info.ID)
				} else {
					actualSize := len(data)
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Media download successful - ID: %s, Expected: %d bytes, Actual: %d bytes, Duration: %v", mycli.userID, evt.Info.ID, mediaSize, actualSize, downloadDuration)

					// Check for size mismatch
					if mediaSize > 0 && int64(actualSize) != mediaSize {
						mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Size mismatch detected - ID: %s, Expected: %d, Got: %d", mycli.userID, evt.Info.ID, mediaSize, actualSize)
					}

					// Log large file processing
					if actualSize > 13*1024*1024 { // 13MB
						mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Processing large file (>13MB) - ID: %s, Size: %d bytes", mycli.userID, evt.Info.ID, actualSize)
					}
				}

				messageMap, ok := dataMap["Message"].(map[string]interface{})
				if !ok {
					messageMap = make(map[string]interface{})
				}

				// Only process storage if download was successful
				if err == nil && len(data) > 0 {
					if mycli.config.MinioEnabled {
						fileName := evt.Info.ID + extension
						storageStart := time.Now()

						mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Uploading to S3/Minio - ID: %s, FileName: %s, Size: %d bytes", mycli.userID, evt.Info.ID, fileName, len(data))

						mediaURL, err := mycli.mediaStorage.Store(context.Background(), data, fileName, mimeType)
						storageDuration := time.Since(storageStart)

						if err != nil {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to store media in S3/Minio - ID: %s, Size: %d bytes, Duration: %v, Error: %v", mycli.userID, evt.Info.ID, len(data), storageDuration, err)

							// Continue processing without storage URL
							mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Continuing message processing without S3 URL - ID: %s", mycli.userID, evt.Info.ID)
						} else {
							mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] S3/Minio upload successful - ID: %s, Size: %d bytes, Duration: %v, URL: %s", mycli.userID, evt.Info.ID, len(data), storageDuration, mediaURL)
							messageMap["mediaUrl"] = mediaURL
							messageMap["mimetype"] = mimeType
						}
					} else {
						mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Encoding to base64 - ID: %s, Size: %d bytes", mycli.userID, evt.Info.ID, len(data))
						encodeStart := time.Now()

						encodeData := base64.StdEncoding.EncodeToString(data)
						encodeDuration := time.Since(encodeStart)

						mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Base64 encoding completed - ID: %s, Original: %d bytes, Encoded: %d chars, Duration: %v", mycli.userID, evt.Info.ID, len(data), len(encodeData), encodeDuration)
						messageMap["base64"] = encodeData
					}
				} else {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Skipping media storage due to download failure - ID: %s", mycli.userID, evt.Info.ID)
				}

				dataMap["Message"] = messageMap
			}
		}

		isGroup := strings.HasSuffix(evt.Info.Chat.String(), "@g.us")
		if isGroup {
			groupData, err := mycli.WAClient.GetGroupInfo(context.Background(), evt.Info.Chat)
			if err == nil {
				dataMap["groupData"] = groupData
			}
		}

		delete(dataMap, "RawMessage")

		if message, ok := dataMap["Message"].(map[string]interface{}); ok {
			if imageMessage, ok := message["imageMessage"].(map[string]interface{}); ok {
				delete(imageMessage, "JPEGThumbnail")
				message["imageMessage"] = imageMessage
				dataMap["Message"] = message
			}

			if videoMessage, ok := message["videoMessage"].(map[string]interface{}); ok {
				delete(videoMessage, "JPEGThumbnail")
				message["videoMessage"] = videoMessage
				dataMap["Message"] = message
			}

			if documentMessage, ok := message["documentMessage"].(map[string]interface{}); ok {
				delete(documentMessage, "JPEGThumbnail")
				message["documentMessage"] = documentMessage
				dataMap["Message"] = message
			}
		}

		postMap["data"] = dataMap

		// ===== BUTTON CLICK EVENT DETECTION =====
		// Detecta cliques em botões e emite evento separado "ButtonClick"
		// Suporta 3 formatos: ButtonsResponseMessage, InteractiveResponseMessage (NativeFlow), TemplateButtonReplyMessage
		var buttonClickData map[string]interface{}

		if resp := evt.Message.GetButtonsResponseMessage(); resp != nil {
			// Legacy buttons response
			buttonClickData = map[string]interface{}{
				"buttonId":   resp.GetSelectedButtonID(),
				"buttonText": resp.GetSelectedDisplayText(),
				"type":       "buttons_response",
			}
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Button click detected (legacy): buttonId=%s, buttonText=%s", mycli.userID, resp.GetSelectedButtonID(), resp.GetSelectedDisplayText())
		} else if resp := evt.Message.GetInteractiveResponseMessage(); resp != nil {
			// NativeFlow interactive response (quick_reply, cta_url, cta_call, cta_copy)
			if nf := resp.GetNativeFlowResponseMessage(); nf != nil {
				buttonId, buttonText := parseNativeFlowResponseParams(nf.GetParamsJSON())
				buttonClickData = map[string]interface{}{
					"buttonId":   buttonId,
					"buttonText": buttonText,
					"type":       "native_flow_response",
					"name":       nf.GetName(),
					"paramsJSON": nf.GetParamsJSON(),
				}
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Button click detected (native_flow): name=%s, buttonId=%s, buttonText=%s", mycli.userID, nf.GetName(), buttonId, buttonText)
			}
		} else if resp := evt.Message.GetTemplateButtonReplyMessage(); resp != nil {
			// Template button reply
			buttonClickData = map[string]interface{}{
				"buttonId":   resp.GetSelectedID(),
				"buttonText": resp.GetSelectedDisplayText(),
				"type":       "template_button_reply",
			}
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Button click detected (template): buttonId=%s, buttonText=%s", mycli.userID, resp.GetSelectedID(), resp.GetSelectedDisplayText())
		} else if resp := evt.Message.GetListResponseMessage(); resp != nil {
			// List response (single select)
			buttonClickData = map[string]interface{}{
				"buttonId":    resp.GetSingleSelectReply().GetSelectedRowID(),
				"buttonText":  resp.GetTitle(),
				"type":        "list_response",
				"description": resp.GetDescription(),
			}
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] List selection detected: rowId=%s, title=%s", mycli.userID, resp.GetSingleSelectReply().GetSelectedRowID(), resp.GetTitle())
		}

		// Se detectou clique em botão, emite evento separado "ButtonClick"
		if buttonClickData != nil {
			buttonClickMap := map[string]interface{}{
				"event": "ButtonClick",
				"data": map[string]interface{}{
					"buttonId":   buttonClickData["buttonId"],
					"buttonText": buttonClickData["buttonText"],
					"type":       buttonClickData["type"],
					"phone":      dataMap["Sender"],
					"jid":        dataMap["Sender"],
					"pushName":   dataMap["PushName"],
					"messageId":  dataMap["ID"],
					"chat":       dataMap["Chat"],
					"fromMe":     dataMap["FromMe"],
					"timestamp":  evt.Info.Timestamp.Unix(),
					"extraData":  buttonClickData,
				},
				"instanceToken": mycli.token,
				"instanceId":    mycli.userID,
				"instanceName":  mycli.Instance().Name,
			}

			buttonClickJSON, err := json.Marshal(buttonClickMap)
			if err == nil {
				buttonClickQueue := strings.ToLower(fmt.Sprintf("%s.buttonclick", userID))
				go mycli.service.CallWebhook(mycli.Instance(), buttonClickQueue, buttonClickJSON)
				if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
					go mycli.service.SendToGlobalQueues("ButtonClick", buttonClickJSON, mycli.userID)
				}
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] ===== BUTTON CLICK EVENT DISPATCHED ===== Type: %s, ButtonId: %s", mycli.userID, buttonClickData["type"], buttonClickData["buttonId"])
			}
		}

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] ===== MESSAGE PROCESSING COMPLETED ===== ID: %s, From: %s, Type: %s, Webhook: %v", mycli.userID, evt.Info.ID, evt.Info.Chat.String(), evt.Info.Type, doWebhook)
	case *events.Receipt:
		doWebhook = true
		postMap["event"] = "Receipt"

		// se ignoreGroup for true e o chat for grupo retorna
		if mycli.Instance().IgnoreGroups && strings.Contains(evt.Chat.String(), "@g.us") {
			return
		}

		if mycli.config.EventIgnoreGroup && strings.Contains(evt.Chat.String(), "@g.us") {
			return
		}

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Receipt received with ID: %s from %s with type %s", mycli.userID, evt.MessageIDs[0], evt.SourceString(), evt.Type)

		if evt.Type == types.ReceiptTypeRead || evt.Type == types.ReceiptTypeReadSelf {

			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Message was read by %s", mycli.userID, evt.SourceString())
			if evt.Type == types.ReceiptTypeRead {
				postMap["state"] = "Read"
				for _, v := range evt.MessageIDs {
					messageKey := fmt.Sprintf("%s_%s_%s", mycli.userID, v, "Read")
					if _, found := mycli.processedMessages.Get(messageKey); found {
						mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Message duplicated ignored: %s", mycli.userID, v)
						continue
					}

					mycli.processedMessages.Set(messageKey, true, 30*time.Minute)

					var message message_model.Message

					message.MessageID = v
					message.Timestamp = evt.Timestamp.Format("2006-01-02 15:04:05")
					message.Status = "Read"
					message.Source = evt.Chat.ToNonAD().User

					if mycli.config.DatabaseSaveMessages {
						go mycli.messageRepository.InsertMessage(message)
					}
				}
			} else {
				postMap["state"] = "ReadSelf"
			}
		} else if evt.Type == types.ReceiptTypeDelivered {
			postMap["state"] = "Delivered"

			var message message_model.Message

			message.MessageID = evt.MessageIDs[0]
			message.Timestamp = evt.Timestamp.Format("2006-01-02 15:04:05")
			message.Status = "Delivered"
			message.Source = evt.Chat.ToNonAD().User

			messageKey := fmt.Sprintf("%s_%s_%s", mycli.userID, evt.MessageIDs[0], "Delivered")
			if _, found := mycli.processedMessages.Get(messageKey); found {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Message duplicated ignored: %s", mycli.userID, evt.MessageIDs[0])
				return
			}

			mycli.processedMessages.Set(messageKey, true, 30*time.Minute)

			if mycli.config.DatabaseSaveMessages {
				go mycli.messageRepository.InsertMessage(message)
			}

			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Message delivered to %s", mycli.userID, evt.SourceString())
		} else {
			return
		}
	case *events.Presence:
		doWebhook = true
		postMap["event"] = "Presence"

		if evt.Unavailable {
			postMap["state"] = "offline"
			if evt.LastSeen.IsZero() {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] User is now offline", mycli.userID)
			} else {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] User is now offline since %s", mycli.userID, evt.LastSeen.Format("2006-01-02 15:04:05"))
			}
		} else {
			postMap["state"] = "online"
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] User is now online", mycli.userID)
		}
	case *events.Archive:
		doWebhook = true
		postMap["event"] = "Archive"

		dataMap := postMap["data"].(map[string]interface{})
		dataMap["JID"] = evt.JID
		dataMap["Timestamp"] = evt.Timestamp
		dataMap["Action"] = evt.Action
		dataMap["FromFullSync"] = evt.FromFullSync
		postMap["data"] = dataMap

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Chat archived", mycli.userID)
	case *events.HistorySync:
		doWebhook = true
		postMap["event"] = "HistorySync"

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] History sync event received %+v", mycli.userID, evt.Data.SyncType)
	case *events.AppState:
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] App state event received %+v", mycli.userID, evt)
	case *events.LoggedOut:
		doWebhook = true
		postMap["event"] = "LoggedOut"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Logged out for reason %s", mycli.userID, evt.Reason.String())
		// The device was unlinked — reconnecting can't help, it needs a new QR or
		// pairing code. Clear the budget so the next real drop starts from zero.
		mycli.service.ResetAutoReconnect(mycli.userID)

		// Clear the stored JID as well. It is what marks an instance as paired,
		// and auto-reconnect reads it to decide whether to retry forever. Left
		// behind, an unlinked instance would retry a session that no longer
		// exists instead of surfacing a fresh QR.
		mycli.Instance().Jid = ""
		if err := mycli.instanceRepository.UpdateJid(mycli.Instance().Id, ""); err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error clearing JID after logout: %v", mycli.userID, err)
		}

		// Limpar cache de userInfo para esta instância
		mycli.userInfoCache.Delete(mycli.Instance().Token)
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] UserInfo cache cleared for token: %s", mycli.userID, mycli.Instance().Token)

		mycli.Instance().DisconnectReason = evt.Reason.String()
		mycli.Instance().Connected = false
		err := mycli.instanceRepository.UpdateConnected(mycli.Instance().Id, mycli.Instance().Connected, mycli.Instance().DisconnectReason)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error updating instance: %s", mycli.Instance().Id, err)
		}

		if postMap["data"] != nil {
			jsonBytes, err := json.Marshal(postMap["data"])
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal postMap['data']: %v", mycli.userID, err)
				return
			}

			var dataMap map[string]interface{}
			err = json.Unmarshal(jsonBytes, &dataMap)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to unmarshal postMap['data'] to map[string]interface{}: %v", mycli.userID, err)
				return
			}

			postMap["data"] = dataMap
		} else {
			postMap["data"] = make(map[string]interface{})
		}

		dataMap := postMap["data"].(map[string]interface{})

		dataMap["reason"] = evt.Reason.String()

		// Enviar evento LoggedOut para webhook/RabbitMQ ANTES de matar o canal
		postMap["instanceToken"] = mycli.Instance().Token
		postMap["instanceId"] = mycli.userID
		postMap["instanceName"] = mycli.Instance().Name

		values, err := json.Marshal(postMap)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal JSON for LoggedOut event", mycli.userID)
		} else {
			var queueName string
			if _, ok := postMap["event"]; ok {
				queueName = strings.ToLower(fmt.Sprintf("%s.%s", mycli.userID, postMap["event"]))
			}

			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] ===== DISPATCHING LOGGEDOUT EVENT ===== Queue: %s", mycli.userID, queueName)

			// Enviar para webhook/RabbitMQ
			go mycli.service.CallWebhook(mycli.Instance(), queueName, values)

			if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Sending LoggedOut to global queues - AMQP: %v, NATS: %v", mycli.userID, mycli.config.AmqpGlobalEnabled, mycli.config.NatsGlobalEnabled)
				go mycli.service.SendToGlobalQueues(postMap["event"].(string), values, mycli.userID)
			}
		}

		// Stop the instance's goroutines AFTER the event has gone out. This was
		// a blocking send that could hang the event handler outright.
		mycli.entry.Stop()
	case *events.ChatPresence:
		doWebhook = true
		postMap["event"] = "ChatPresence"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Chat presence received %+v", mycli.userID, evt)
	case *events.CallOffer:
		doWebhook = true
		postMap["event"] = "CallOffer"

		// Verifica se deve rejeitar chamadas automaticamente
		if mycli.Instance().RejectCall {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Auto-rejecting call from %s", mycli.userID, evt.CallCreator.String())

			// Rejeita a chamada
			mycli.WAClient.RejectCall(context.Background(), evt.CallCreator, evt.CallID)

			// Envia mensagem de rejeição se configurada
			if mycli.Instance().MsgRejectCall != "" {
				msg := &waE2E.Message{
					ExtendedTextMessage: &waE2E.ExtendedTextMessage{
						Text: &mycli.Instance().MsgRejectCall,
					},
				}

				_, err := mycli.WAClient.SendMessage(context.Background(), evt.CallCreator, msg)
				if err != nil {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to send reject call message: %v", mycli.userID, err)
				} else {
					mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Sent reject call message to %s", mycli.userID, evt.CallCreator.String())
				}
			}
			return
		}

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call offer %+v", mycli.userID, evt)

		// Hand the offer to the VoIP stack so the call can actually be answered.
		// Runs after the auto-reject branch above, which returns early.
		mycli.service.CallRegistry().HandleOffer(
			context.Background(), mycli.userID, mycli.WAClient, evt.From, evt.Data,
		)
	case *events.CallAccept:
		doWebhook = true
		postMap["event"] = "CallAccept"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call accept %+v", mycli.userID, evt)
		mycli.service.CallRegistry().HandleAccept(context.Background(), mycli.userID, evt.From, evt.Data)
	case *events.CallPreAccept:
		doWebhook = true
		postMap["event"] = "CallPreAccept"
		mycli.service.CallRegistry().HandleAccept(context.Background(), mycli.userID, evt.From, evt.Data)
	case *events.CallTransport:
		doWebhook = true
		postMap["event"] = "CallTransport"
		mycli.service.CallRegistry().HandleTransport(context.Background(), mycli.userID, evt.From, evt.Data)
	case *events.CallReject:
		doWebhook = true
		postMap["event"] = "CallReject"
		mycli.service.CallRegistry().HandleTerminate(mycli.userID, evt.From, evt.Data)
	case *events.CallTerminate:
		doWebhook = true
		postMap["event"] = "CallTerminate"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call terminate %+v", mycli.userID, evt)
		mycli.service.CallRegistry().HandleTerminate(mycli.userID, evt.From, evt.Data)
	case *events.CallOfferNotice:
		doWebhook = true
		postMap["event"] = "CallOfferNotice"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call offer notice %+v", mycli.userID, evt)
	case *events.CallRelayLatency:
		doWebhook = true
		postMap["event"] = "CallRelayLatency"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got call relay latency %+v", mycli.userID, evt)
	case *events.OfflineSyncCompleted:
		doWebhook = true
		postMap["event"] = "OfflineSyncCompleted"
	case *events.ConnectFailure:
		doWebhook = true
		postMap["event"] = "ConnectFailure"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Connection failed with reason %s", mycli.userID, evt.Reason.String())

		// Limpar cache de userInfo para esta instância
		mycli.userInfoCache.Delete(mycli.Instance().Token)
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] UserInfo cache cleared for token: %s", mycli.userID, mycli.Instance().Token)

		mycli.Instance().DisconnectReason = evt.Reason.String()
		mycli.Instance().Connected = false
		err := mycli.instanceRepository.UpdateConnected(mycli.Instance().Id, mycli.Instance().Connected, mycli.Instance().DisconnectReason)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error updating instance: %s", mycli.Instance().Id, err)
		}
	case *events.Disconnected:
		doWebhook = true
		postMap["event"] = "Disconnected"

		// Limpar cache de userInfo para esta instância (mas não para reconexão automática)
		mycli.userInfoCache.Delete(mycli.Instance().Token)
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] UserInfo cache cleared for token: %s", mycli.userID, mycli.Instance().Token)

		mycli.Instance().DisconnectReason = "Disconnected emitted because the websocket is closed by the server."
		mycli.Instance().Connected = false
		err := mycli.instanceRepository.UpdateConnected(mycli.Instance().Id, mycli.Instance().Connected, mycli.Instance().DisconnectReason)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error updating instance: %s", mycli.Instance().Id, err)
		}

		// Bring the instance back up, bounded to maxAutoReconnectAttempts tries.
		// The budget is refilled by events.Connected, so a connection that keeps
		// recovering is always retried while a dead one stops after 3 attempts.
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Disconnected detected, scheduling auto-reconnect", mycli.userID)
		mycli.service.ScheduleAutoReconnect(mycli.userID)
	case *events.LabelEdit:
		doWebhook = true
		postMap["event"] = "LabelEdit"
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Got label edit %+v", mycli.userID, evt.Action)

		label := label_model.Label{
			InstanceID:   mycli.userID,
			LabelID:      evt.LabelID,
			LabelName:    utils.GetStringValue(evt.Action.Name),
			LabelColor:   fmt.Sprintf("%d", evt.Action.Color),
			PredefinedId: fmt.Sprintf("%d", evt.Action.PredefinedID),
		}

		err := mycli.labelRepository.UpsertLabel(label)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to upsert label: %v", mycli.userID, err)
		}
	case *events.LabelAssociationChat:
		doWebhook = true
		postMap["event"] = "LabelAssociationChat"

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Label association chat received %+v", mycli.userID, evt)
	case *events.LabelAssociationMessage:
		doWebhook = true
		postMap["event"] = "LabelAssociationMessage"

		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Label association message received %+v", mycli.userID, evt)
	case *events.Contact:
		doWebhook = true
		postMap["event"] = "Contact"
	case *events.PushName:
		doWebhook = true
		postMap["event"] = "PushName"
	case *events.IdentityChange:
		doWebhook = false
	case *events.GroupInfo:
		doWebhook = true
		postMap["event"] = "GroupInfo"
	case *events.JoinedGroup:
		doWebhook = true
		postMap["event"] = "JoinedGroup"
	case *events.NewsletterJoin:
		doWebhook = true
		postMap["event"] = "NewsletterJoin"
	case *events.NewsletterLeave:
		doWebhook = true
		postMap["event"] = "NewsletterLeave"
	case *events.UndecryptableMessage:
		jsonEvt, err := json.Marshal(evt)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Undecryptable message received: %s", mycli.userID, evt.Info.ID)
		}
		mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Undecryptable message received all: %+v", mycli.userID, string(jsonEvt))

		if evt.UnavailableType == "view_once" {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Undecryptable message received view_once: %s", mycli.userID, evt.Info.ID)

			doWebhook = true
			postMap["event"] = "Message"

			postMap["data"] = evt
		} else if strings.HasPrefix(evt.Info.ID, "66") || strings.HasPrefix(evt.Info.ID, "67") {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] ID 66 or 67 found, reconnecting client", mycli.userID)
			mycli.WAClient.Disconnect()
			err := connectShared(mycli.WAClient)
			if err != nil {
				mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Error reconnecting client: %s", mycli.userID, err)
			}
		} else {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] ID is not 66 or 67 or view_once, skipping", mycli.userID)
		}
	default:
		mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] Unhandled event %s: %+v", mycli.userID, fmt.Sprintf("%T", evt), evt)
		return
	}

	if doWebhook {
		postMap["instanceToken"] = mycli.token
		postMap["instanceId"] = mycli.userID
		postMap["instanceName"] = mycli.Instance().Name

		values, err := json.Marshal(postMap)
		if err != nil {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogError("[%s] Failed to marshal JSON for queue", mycli.userID)
			return
		}

		var queueName string
		if _, ok := postMap["event"]; ok {
			queueName = strings.ToLower(fmt.Sprintf("%s.%s", userID, postMap["event"]))
		}

		// Log webhook dispatch
		eventType := "unknown"
		if event, ok := postMap["event"].(string); ok {
			eventType = event
		}

		dataSize := len(values)
		mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] ===== DISPATCHING WEBHOOK ===== Event: %s, Queue: %s, DataSize: %d bytes", mycli.userID, eventType, queueName, dataSize)

		go mycli.service.CallWebhook(mycli.Instance(), queueName, values)

		if mycli.config.AmqpGlobalEnabled || mycli.config.NatsGlobalEnabled {
			mycli.loggerWrapper.GetLogger(mycli.userID).LogInfo("[%s] Sending to global queues - Event: %s, AMQP: %v, NATS: %v", mycli.userID, eventType, mycli.config.AmqpGlobalEnabled, mycli.config.NatsGlobalEnabled)
			go mycli.service.SendToGlobalQueues(postMap["event"].(string), values, mycli.userID)
		}
	} else {
		mycli.loggerWrapper.GetLogger(mycli.userID).LogWarn("[%s] ===== WEBHOOK SKIPPED ===== doWebhook=false", mycli.userID)
	}
}

func (w *whatsmeowService) CallWebhook(instance *instance_model.Instance, queueName string, jsonData []byte) {
	var data map[string]interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return
	}

	eventType, ok := data["event"].(string)
	if !ok {
		return
	}

	eventArray := strings.Split(instance.Events, ",")

	var subscriptions []string

	if len(eventArray) < 1 {
		subscriptions = append(subscriptions, event_types.MESSAGE)
	} else {
		for _, arg := range eventArray {
			if !event_types.IsEventType(arg) {
				w.loggerWrapper.GetLogger(instance.Id).LogWarn("[%s] Message type discarded: %s", instance.Id, arg)
				continue
			}
			if !utils.Find(subscriptions, arg) {
				subscriptions = append(subscriptions, arg)
			}

		}
	}

	if contains(subscriptions, "ALL") {
		w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
		w.sendToQueueOrWebhook(instance, queueName, jsonData)
		return
	}

	w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] subscriptions %s eventType %s", instance.Id, subscriptions, eventType)

	switch eventType {
	case "Message":
		if contains(subscriptions, "MESSAGE") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		} else {
			// Forward to GROUP/NEWSLETTER subscribers even without MESSAGE subscription
			if dataMap, ok := data["data"].(map[string]interface{}); ok {
				if infoMap, ok := dataMap["Info"].(map[string]interface{}); ok {
					if chat, ok := infoMap["Chat"].(string); ok {
						if strings.HasSuffix(chat, "@g.us") && contains(subscriptions, "GROUP") {
							w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s (Group)", instance.Id, eventType)
							w.sendToQueueOrWebhook(instance, queueName, jsonData)
						} else if strings.HasSuffix(chat, "@newsletter") && contains(subscriptions, "NEWSLETTER") {
							w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s (Newsletter)", instance.Id, eventType)
							w.sendToQueueOrWebhook(instance, queueName, jsonData)
						}
					}
				}
			}
		}
	case "SendMessage":
		if contains(subscriptions, "SEND_MESSAGE") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		} else {
			if dataMap, ok := data["data"].(map[string]interface{}); ok {
				if infoMap, ok := dataMap["Info"].(map[string]interface{}); ok {
					if chat, ok := infoMap["Chat"].(string); ok {
						if strings.HasSuffix(chat, "@g.us") && contains(subscriptions, "GROUP") {
							w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s (Group)", instance.Id, eventType)
							w.sendToQueueOrWebhook(instance, queueName, jsonData)
						} else if strings.HasSuffix(chat, "@newsletter") && contains(subscriptions, "NEWSLETTER") {
							w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s (Newsletter)", instance.Id, eventType)
							w.sendToQueueOrWebhook(instance, queueName, jsonData)
						}
					}
				}
			}
		}
	case "Receipt":
		if contains(subscriptions, "READ_RECEIPT") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		} else {
			if dataMap, ok := data["data"].(map[string]interface{}); ok {
				if chat, ok := dataMap["Chat"].(string); ok {
					if strings.HasSuffix(chat, "@g.us") && contains(subscriptions, "GROUP") {
						w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s (Group)", instance.Id, eventType)
						w.sendToQueueOrWebhook(instance, queueName, jsonData)
					} else if strings.HasSuffix(chat, "@newsletter") && contains(subscriptions, "NEWSLETTER") {
						w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s (Newsletter)", instance.Id, eventType)
						w.sendToQueueOrWebhook(instance, queueName, jsonData)
					}
				}
			}
		}
	case "Presence":
		if contains(subscriptions, "PRESENCE") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "HistorySync":
		if contains(subscriptions, "HISTORY_SYNC") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "ChatPresence", "Archive":
		if contains(subscriptions, "CHAT_PRESENCE") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "CallOffer", "CallAccept", "CallTerminate", "CallOfferNotice", "CallRelayLatency":
		if contains(subscriptions, "CALL") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "Connected", "PairSuccess", "TemporaryBan", "LoggedOut", "ConnectFailure", "Disconnected":
		if contains(subscriptions, "CONNECTION") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "LabelEdit", "LabelAssociationChat", "LabelAssociationMessage":
		if contains(subscriptions, "LABEL") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "Contact", "PushName":
		if contains(subscriptions, "CONTACT") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "GroupInfo", "JoinedGroup":
		if contains(subscriptions, "GROUP") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "NewsletterJoin", "NewsletterLeave":
		if contains(subscriptions, "NEWSLETTER") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "QRCode", "QRTimeout", "QRSuccess":
		if contains(subscriptions, "QRCODE") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}
	case "ButtonClick":
		if contains(subscriptions, "BUTTON_CLICK") || contains(subscriptions, "MESSAGE") {
			w.loggerWrapper.GetLogger(instance.Id).LogInfo("[%s] Event received of type %s", instance.Id, eventType)
			w.sendToQueueOrWebhook(instance, queueName, jsonData)
		}

	default:
		return
	}
}

func contains(subscriptions []string, event string) bool {
	for _, sub := range subscriptions {
		if strings.EqualFold(sub, event) {
			return true
		}
	}
	return false
}

func (w *whatsmeowService) sendToQueueOrWebhook(instance *instance_model.Instance, queueName string, jsonData []byte) {
	logger := w.loggerWrapper.GetLogger(instance.Id)

	// Each transport is independent. This used to return on the first failure,
	// so a broken RabbitMQ silently stopped every webhook for the instance —
	// the transports have nothing to do with each other and one being down must
	// not suppress the others.
	if instance.RabbitmqEnable == "enabled" || instance.RabbitmqEnable == "true" {
		if err := w.rabbitmqProducer.Produce(queueName, jsonData, instance.RabbitmqEnable, instance.Id); err != nil {
			logger.LogError("[%s] Failed to send message to rabbitmq: %s", instance.Id, err)
		} else {
			logger.LogInfo("[%s] Message sent to rabbitmq successfully", instance.Id)
		}
	}

	if instance.NatsEnable == "enabled" || instance.NatsEnable == "true" {
		if err := w.natsProducer.Produce(queueName, jsonData, instance.NatsEnable, instance.Id); err != nil {
			logger.LogError("[%s] Failed to send message to nats: %s", instance.Id, err)
		} else {
			logger.LogInfo("[%s] Message sent to nats successfully", instance.Id)
		}
	}

	if instance.WebSocketEnable == "enabled" || instance.WebSocketEnable == "true" {
		if err := w.websocketProducer.Produce(queueName, jsonData, instance.Id, instance.Token); err != nil {
			logger.LogError("[%s] Failed to send message to websocket: %s", instance.Id, err)
		} else {
			logger.LogInfo("[%s] Message sent to websocket successfully", instance.Id)
		}
	}

	// Deduplicate: the same URL configured both as the legacy single webhook and
	// in the list would otherwise get every event twice.
	seen := make(map[string]bool, len(instance.Webhooks)+1)
	allWebhooks := make([]string, 0, len(instance.Webhooks)+1)

	for _, candidate := range append([]string{instance.Webhook}, instance.Webhooks...) {
		if candidate == "" || candidate == "disabled" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		allWebhooks = append(allWebhooks, candidate)
	}

	for _, webhookURL := range allWebhooks {
		// Produce hands the delivery to a background goroutine, so an error here
		// only means it was refused outright. The delivery result itself is
		// logged by the producer.
		if err := w.webhookProducer.Produce(queueName, jsonData, webhookURL, instance.Id); err != nil {
			logger.LogError("[%s] Failed to queue webhook %s: %s", instance.Id, webhookURL, err)
		} else {
			logger.LogInfo("[%s] Webhook %s queued for delivery", instance.Id, webhookURL)
		}
	}
}

func (w *whatsmeowService) StartInstance(instanceId string) error {
	instance, err := w.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		return err
	}

	if instance.Proxy == "" && w.config.ProxyHost != "" && w.config.ProxyPort != "" && w.config.ProxyUsername != "" && w.config.ProxyPassword != "" {
		proxyConfig := ProxyConfig{
			Protocol: utils.NormalizeProxyProtocol(w.config.ProxyProtocol, w.config.ProxyPort),
			Host:     w.config.ProxyHost,
			Port:     w.config.ProxyPort,
			Username: w.config.ProxyUsername,
			Password: w.config.ProxyPassword,
		}

		proxyJSON, err := json.Marshal(proxyConfig)
		if err != nil {
			w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to marshal proxy config: %v", instanceId, err)
			return err
		}

		instance.Proxy = string(proxyJSON)

		err = w.instanceRepository.UpdateProxy(instance.Id, instance.Proxy)
		if err != nil {
			w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Failed to update instance: %s", instanceId, err)
			return err
		}
	}

	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Starting client", instance.Id)

	v := Values{map[string]string{
		"Id":     instance.Id,
		"Jid":    instance.Jid,
		"Token":  instance.Token,
		"Events": instance.Events,
		"osName": instance.OsName,
		"Proxy":  instance.Proxy,
	}}

	w.userInfoCache.Set(instance.Token, v, cache.NoExpiration)

	eventArray := strings.Split(instance.Events, ",")

	var subscribedEvents []string

	if len(eventArray) < 1 {
		subscribedEvents = append(subscribedEvents, event_types.MESSAGE)
	} else {
		for _, arg := range eventArray {
			if !event_types.IsEventType(arg) {
				w.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Message type discarded: %s", instanceId, arg)
				continue
			}
			if !utils.Find(subscribedEvents, arg) {
				subscribedEvents = append(subscribedEvents, arg)
			}

		}
	}

	clientData := &ClientData{
		Instance:      instance,
		Subscriptions: subscribedEvents,
		Phone:         "",
		IsProxy:       false,
	}

	// Legacy rows store the literal "null" rather than an empty string; feeding
	// that through as a host would be harmless, but parsing it as config is not.
	if instance.Proxy != "" && instance.Proxy != "null" {
		var proxyConfig ProxyConfig
		err := json.Unmarshal([]byte(instance.Proxy), &proxyConfig)
		if err != nil {
			w.loggerWrapper.GetLogger(instanceId).LogError("[%s] error unmarshalling proxy config", instanceId)
			return err
		}

		if proxyConfig.Host != "" {
			clientData.IsProxy = true
		}
	}

	go w.StartClient(clientData)

	return nil
}

func (w *whatsmeowService) ConnectOnStartup(clientName string) {
	w.loggerWrapper.GetLogger(clientName).LogInfo("Connecting all instances on startup")
	var instances []*instance_model.Instance
	var err error

	if clientName != "" {
		instances, err = w.instanceRepository.GetAllConnectedInstancesByClientName(clientName)
		if err != nil {
			w.loggerWrapper.GetLogger(clientName).LogError("[%s] Error getting all connected instances: %s", clientName, err)
			return
		}
	} else {
		instances, err = w.instanceRepository.GetAllConnectedInstances()
		if err != nil {
			w.loggerWrapper.GetLogger(clientName).LogError("[%s] Error getting all connected instances: %s", clientName, err)
			return
		}
	}

	w.loggerWrapper.GetLogger(clientName).LogInfo("[%s] Found %d connected instances", clientName, len(instances))

	for _, instance := range instances {
		w.loggerWrapper.GetLogger(clientName).LogInfo("[%s] Starting client for user '%s'", clientName, instance.Id)

		err := w.StartInstance(instance.Id)
		if err != nil {
			w.loggerWrapper.GetLogger(clientName).LogError("[%s] Error starting client: %s", clientName, err)
		}
	}
}

func getExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "audio/ogg":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	default:
		// Se não encontrar um tipo conhecido, extrai a extensão do mimetype
		parts := strings.Split(mimeType, "/")
		if len(parts) > 1 {
			return "." + parts[1]
		}
		return ".bin"
	}
}

func (w *whatsmeowService) SendToGlobalQueues(eventType string, payload []byte, userId string) {
	w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Starting sendToGlobalQueues for event: %s", userId, eventType)

	// AMQP: AMQP_SPECIFIC_EVENTS tem prioridade sobre AMQP_GLOBAL_EVENTS
	if w.config.AmqpGlobalEnabled {
		var shouldSendToAmqp bool
		var amqpQueueName string

		// Se AMQP_SPECIFIC_EVENTS estiver configurada, ela tem prioridade
		if len(w.config.AmqpSpecificEvents) > 0 {
			w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Using AMQP_SPECIFIC_EVENTS (priority over AMQP_GLOBAL_EVENTS)", userId)
			// Verifica se o evento específico está na lista
			if utils.Find(w.config.AmqpSpecificEvents, eventType) {
				shouldSendToAmqp = true
				amqpQueueName = strings.ToLower(eventType)
				w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Event %s found in AMQP_SPECIFIC_EVENTS", userId, eventType)
			}
		} else {
			// Fallback para AMQP_GLOBAL_EVENTS (modo antigo com grupos de eventos)
			w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Using AMQP_GLOBAL_EVENTS (fallback mode)", userId)

			// Mapeia o evento do Whatsmeow para o tipo de evento global
			var globalEventType string
			switch eventType {
			case "Message":
				globalEventType = "MESSAGE"
			case "SendMessage":
				globalEventType = "SEND_MESSAGE"
			case "Receipt":
				globalEventType = "READ_RECEIPT"
			case "Presence":
				globalEventType = "PRESENCE"
			case "HistorySync":
				globalEventType = "HISTORY_SYNC"
			case "ChatPresence", "Archive":
				globalEventType = "CHAT_PRESENCE"
			case "CallOffer", "CallAccept", "CallTerminate", "CallOfferNotice", "CallRelayLatency":
				globalEventType = "CALL"
			case "Connected", "PairSuccess", "TemporaryBan", "LoggedOut", "ConnectFailure", "Disconnected":
				globalEventType = "CONNECTION"
			case "LabelEdit", "LabelAssociationChat", "LabelAssociationMessage":
				globalEventType = "LABEL"
			case "Contact", "PushName":
				globalEventType = "CONTACT"
			case "GroupInfo", "JoinedGroup":
				globalEventType = "GROUP"
			case "NewsletterJoin", "NewsletterLeave":
				globalEventType = "NEWSLETTER"
			case "QRCode", "QRTimeout", "QRSuccess":
				globalEventType = "QRCODE"
			default:
				w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Event %s not mapped to global event type", userId, eventType)
				return
			}

			// Verifica se o grupo de eventos está na lista
			if utils.Find(w.config.AmqpGlobalEvents, globalEventType) {
				shouldSendToAmqp = true
				amqpQueueName = strings.ToLower(eventType)
				w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Event group %s found in AMQP_GLOBAL_EVENTS", userId, globalEventType)
			}
		}

		// Envia para RabbitMQ se necessário
		if shouldSendToAmqp {
			w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Sending to AMQP queue: %s", userId, amqpQueueName)
			err := w.rabbitmqProducer.Produce(amqpQueueName, payload, "global", userId)
			if err != nil {
				w.loggerWrapper.GetLogger(userId).LogError("[%s] Failed to send message to RabbitMQ global queue %s: %v", userId, amqpQueueName, err)
			} else {
				w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Successfully sent message to RabbitMQ global queue %s", userId, amqpQueueName)
			}
		} else {
			w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Event %s not configured for AMQP", userId, eventType)
		}
	}

	// NATS: Mantém o comportamento original por enquanto (só NATS_GLOBAL_EVENTS)
	if w.config.NatsGlobalEnabled {
		// Mapeia o evento para grupo (necessário para NATS por enquanto)
		var globalEventType string
		switch eventType {
		case "Message":
			globalEventType = "MESSAGE"
		case "SendMessage":
			globalEventType = "SEND_MESSAGE"
		case "Receipt":
			globalEventType = "READ_RECEIPT"
		case "Presence":
			globalEventType = "PRESENCE"
		case "HistorySync":
			globalEventType = "HISTORY_SYNC"
		case "ChatPresence", "Archive":
			globalEventType = "CHAT_PRESENCE"
		case "CallOffer", "CallAccept", "CallTerminate", "CallOfferNotice", "CallRelayLatency":
			globalEventType = "CALL"
		case "Connected", "PairSuccess", "TemporaryBan", "LoggedOut", "ConnectFailure", "Disconnected":
			globalEventType = "CONNECTION"
		case "LabelEdit", "LabelAssociationChat", "LabelAssociationMessage":
			globalEventType = "LABEL"
		case "Contact", "PushName":
			globalEventType = "CONTACT"
		case "GroupInfo", "JoinedGroup":
			globalEventType = "GROUP"
		case "NewsletterJoin", "NewsletterLeave":
			globalEventType = "NEWSLETTER"
		case "QRCode", "QRTimeout", "QRSuccess":
			globalEventType = "QRCODE"
		default:
			globalEventType = ""
		}

		// Verifica se o evento está na lista de eventos globais NATS
		if globalEventType != "" && utils.Find(w.config.NatsGlobalEvents, globalEventType) {
			queueName := strings.ToLower(eventType)
			w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Sending to NATS subject: %s", userId, queueName)

			err := w.natsProducer.Produce(queueName, payload, "global", userId)
			if err != nil {
				w.loggerWrapper.GetLogger(userId).LogError("[%s] Failed to send message to NATS global subject %s: %v", userId, queueName, err)
			} else {
				w.loggerWrapper.GetLogger(userId).LogInfo("[%s] Successfully sent message to NATS global subject %s", userId, queueName)
			}
		}
	}
}

var (
	cachedWebVersion   *clientVersion
	cachedWebVersionAt time.Time
	cachedWebVersionMu sync.Mutex
	webVersionCacheTTL = 1 * time.Hour
)

func fetchWhatsAppWebVersion() (*clientVersion, error) {
	cachedWebVersionMu.Lock()
	defer cachedWebVersionMu.Unlock()

	if cachedWebVersion != nil && time.Since(cachedWebVersionAt) < webVersionCacheTTL {
		return cachedWebVersion, nil
	}

	resp, err := http.Get("https://web.whatsapp.com/sw.js")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch WhatsApp Web version: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	content := string(body)

	// Múltiplas estratégias para encontrar client_revision
	patterns := []string{
		`"client_revision":\s*(\d+)`,              // Formato direto
		`\\"client_revision\\":\s*(\d+)`,          // Formato escaped
		`client_revision\\?\\"?:[\s]*(\d+)`,       // Formato mais flexível
		`["']client_revision["'][\s]*:[\s]*(\d+)`, // Com aspas variadas
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)

		if len(matches) >= 2 {
			clientRevision, err := strconv.Atoi(matches[1])
			if err != nil {
				continue // Tenta próximo padrão
			}

			// Log qual padrão funcionou
			if clientRevision > 0 {
				cachedWebVersion = &clientVersion{
					Major: 2,
					Minor: 3000,
					Patch: clientRevision,
				}
				cachedWebVersionAt = time.Now()
				return cachedWebVersion, nil
			}
		}
	}

	// Se chegou aqui, nenhum padrão funcionou - log do conteúdo para debug
	// Mostra apenas uma parte para não logar muito
	previewLength := 500
	if len(content) > previewLength {
		content = content[:previewLength] + "..."
	}

	return nil, fmt.Errorf("could not find client revision in the fetched content. Content preview: %s", content)
}

func (w *whatsmeowService) UpdateInstanceSettings(instanceId string) error {
	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Updating instance settings in runtime", instanceId)

	// Busca a instância atualizada do banco
	instance, err := w.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error getting instance from DB: %v", instanceId, err)
		return err
	}

	// Verifica se o MyClient existe
	myClient, exists := w.myClientPointer.get(instanceId)
	if !exists {
		w.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] MyClient not found in runtime, instance may not be connected", instanceId)
		return fmt.Errorf("instance %s not found in runtime", instanceId)
	}

	// Atualiza as configurações no MyClient em execução
	myClient.setInstance(instance)
	myClient.setRuntimeTargets(instance.Webhook, instance.RabbitmqEnable, instance.NatsEnable, instance.WebSocketEnable)

	// Atualiza as subscriptions se os eventos mudaram
	eventArray := strings.Split(instance.Events, ",")
	var subscribedEvents []string

	if len(eventArray) < 1 {
		subscribedEvents = append(subscribedEvents, event_types.MESSAGE)
	} else {
		for _, arg := range eventArray {
			if !event_types.IsEventType(arg) {
				w.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] Message type discarded: %s", instanceId, arg)
				continue
			}
			if !utils.Find(subscribedEvents, arg) {
				subscribedEvents = append(subscribedEvents, arg)
			}
		}
	}

	myClient.subscriptions = subscribedEvents

	// Atualiza o cache do userInfo com as novas configurações
	v := Values{map[string]string{
		"Id":     instance.Id,
		"Jid":    instance.Jid,
		"Token":  instance.Token,
		"Events": instance.Events,
		"osName": instance.OsName,
		"Proxy":  instance.Proxy,
	}}
	w.userInfoCache.Set(instance.Token, v, cache.NoExpiration)

	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance settings and cache updated in runtime successfully", instanceId)
	return nil
}

func (w *whatsmeowService) UpdateInstanceAdvancedSettings(instanceId string) error {
	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Updating advanced settings in runtime", instanceId)

	// Busca a instância atualizada do banco
	instance, err := w.instanceRepository.GetInstanceByID(instanceId)
	if err != nil {
		w.loggerWrapper.GetLogger(instanceId).LogError("[%s] Error getting instance from DB: %v", instanceId, err)
		return err
	}

	// Verifica se o MyClient existe
	myClient, exists := w.myClientPointer.get(instanceId)
	if !exists {
		w.loggerWrapper.GetLogger(instanceId).LogWarn("[%s] MyClient not found in runtime, instance may not be connected", instanceId)
		return fmt.Errorf("instance %s not found in runtime", instanceId)
	}

	// Atualiza a instância no MyClient com as advanced settings atualizadas
	myClient.setInstance(instance)

	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Advanced settings updated in runtime successfully", instanceId)
	return nil
}

func (w *whatsmeowService) ClearInstanceCache(instanceId string, token string) error {
	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Clearing instance cache - Token: %s", instanceId, token)

	// Limpar userInfoCache
	w.userInfoCache.Delete(token)

	w.myClientPointer.remove(instanceId)

	// Remove stops the instance and forgets it. The old code closed the stop
	// channel here while StartClient was still sending on it, which panics the
	// whole process with "send on closed channel" — the entry's stop signal is
	// idempotent instead.
	if w.clients.Remove(instanceId) {
		w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Client stopped and cleared", instanceId)
	}

	w.loggerWrapper.GetLogger(instanceId).LogInfo("[%s] Instance cache completely cleared", instanceId)
	return nil
}

func NewWhatsmeowService(
	instanceRepository instance_repository.InstanceRepository,
	authDB *sql.DB,
	messageRepository message_repository.MessageRepository,
	labelRepository label_repository.LabelRepository,
	config *config.Config,
	clients *whatsmeow_registry.Clients,
	rabbitmqProducer producer_interfaces.Producer,
	webhookProducer producer_interfaces.Producer,
	websocketProducer producer_interfaces.Producer,
	sqliteDB *sql.DB,
	exPath string,
	mediaStorage storage_interfaces.MediaStorage,
	natsProducer producer_interfaces.Producer,
	loggerWrapper *logger_wrapper.LoggerManager,
) WhatsmeowService {
	// Inicializar PollService de forma segura
	pollSvc := poll_service.NewPollService(authDB, loggerWrapper)

	return &whatsmeowService{
		instanceRepository: instanceRepository,
		authDB:             authDB,
		messageRepository:  messageRepository,
		labelRepository:    labelRepository,
		pollService:        pollSvc, // NOVO: Serviço de enquetes
		config:             config,
		userInfoCache:      cache.New(5*time.Minute, 10*time.Minute),
		clients:            clients,
		myClientPointer:    newMyClientStore(),
		rabbitmqProducer:   rabbitmqProducer,
		webhookProducer:    webhookProducer,
		websocketProducer:  websocketProducer,
		sqliteDB:           sqliteDB,
		exPath:             exPath,
		mediaStorage:       mediaStorage,
		processedMessages:  cache.New(30*time.Minute, 1*time.Hour),
		natsProducer:       natsProducer,
		loggerWrapper:      loggerWrapper,
		passkeyCeremony:    ceremony.NewStore(),
		reconnect:          newReconnectTracker(),
		callRegistry:       voip_registry.NewRegistry(nil),
	}
}

// GetPollService retorna o serviço de polls (evita dupla inicialização)
func (w *whatsmeowService) GetPollService() poll_service.PollService {
	return w.pollService
}

// PasskeyCeremonyStore returns the shared passkey ceremony store (read by the public
// /passkey-ceremony endpoints, written by the whatsmeow event goroutine).
func (w *whatsmeowService) PasskeyCeremonyStore() *ceremony.Store {
	return w.passkeyCeremony
}

// SubmitPasskeyResponse forwards a WebAuthn assertion to WhatsApp for the given instance
// and advances its ceremony to awaiting-confirmation.
func (w *whatsmeowService) SubmitPasskeyResponse(instanceId string, resp *types.WebAuthnResponse) error {
	client := w.clients.Get(instanceId)
	if client == nil {
		return fmt.Errorf("instance %s not running", instanceId)
	}
	if err := client.SendPasskeyResponse(context.Background(), resp); err != nil {
		if w.passkeyCeremony != nil {
			w.passkeyCeremony.SetError(instanceId, err.Error())
		}
		return err
	}
	if w.passkeyCeremony != nil {
		w.passkeyCeremony.SetAwaitingConfirmation(instanceId)
	}
	return nil
}

// ConfirmPasskey confirms the pairing code for the given instance, finishing the ceremony.
func (w *whatsmeowService) ConfirmPasskey(instanceId string) error {
	client := w.clients.Get(instanceId)
	if client == nil {
		return fmt.Errorf("instance %s not running", instanceId)
	}
	if err := client.SendPasskeyConfirmation(context.Background()); err != nil {
		if w.passkeyCeremony != nil {
			w.passkeyCeremony.SetError(instanceId, err.Error())
		}
		return err
	}
	if w.passkeyCeremony != nil {
		w.passkeyCeremony.SetConfirmed(instanceId)
	}
	return nil
}

// cleanSenderID remove a parte ":numero" do sender ID para exibir apenas o remoteJid correto
// Exemplo: "557499879409:3@s.whatsapp.net" -> "557499879409@s.whatsapp.net"
func cleanSenderID(senderID string) string {
	// Procura pelo padrão ":numero" antes do @
	if colonIndex := strings.Index(senderID, ":"); colonIndex != -1 {
		if atIndex := strings.Index(senderID, "@"); atIndex != -1 && colonIndex < atIndex {
			// Remove a parte ":numero" mantendo apenas o número principal e o domínio
			return senderID[:colonIndex] + senderID[atIndex:]
		}
	}
	return senderID
}

// parseNativeFlowResponseParams extracts the selected id and label without
// changing the raw paramsJSON carried by the webhook. Key spelling varies by
// flow; legacy id/display_text take precedence over the selection aliases.
func parseNativeFlowResponseParams(paramsJSON string) (buttonID, buttonText string) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return "", ""
	}
	return firstStringParam(params, "id", "selectedRowId", "selected_row_id", "selectedId", "selected_id"),
		firstStringParam(params, "display_text", "title", "selectedDisplayText")
}

// firstStringParam returns the first key present in params whose value is a
// non-empty string. Native-flow reply params spell the same field differently
// depending on the flow, so callers pass every spelling they accept.
func firstStringParam(params map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := params[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}
