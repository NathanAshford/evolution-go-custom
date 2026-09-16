package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var _k1 = []byte{0x80, 0x41, 0x91, 0x24, 0x6c, 0xf3, 0x66, 0x67, 0x41, 0x5e, 0xde, 0xd2, 0x7d, 0xd9, 0x10, 0x99, 0x3a, 0x99, 0x6f, 0xb9, 0xc6, 0x1d, 0xce, 0xe4, 0x1a, 0x10, 0x5a, 0x5d, 0x16, 0xe7, 0xc8, 0xd2, 0xd0, 0x1f, 0xc8, 0xbb, 0x61, 0x82, 0x7a, 0x4a, 0x28, 0x3a}
var _k0 = []byte{0xe8, 0x35, 0xe5, 0x54, 0x1f, 0xc9, 0x49, 0x48, 0x2d, 0x37, 0xbd, 0xb7, 0x13, 0xaa, 0x75, 0xb7, 0x5f, 0xef, 0x00, 0xd5, 0xb3, 0x69, 0xa7, 0x8b, 0x74, 0x76, 0x35, 0x28, 0x78, 0x83, 0xa9, 0xa6, 0xb9, 0x70, 0xa6, 0x95, 0x02, 0xed, 0x17, 0x64, 0x4a, 0x48}

var (
	_eo2m string
	_wa    string
)

func _hcg() string {
	if _eo2m != "" && _wa != "" {
		return _kw3v(_eo2m, _wa)
	}
	parts := [...]string{"h", "tt", "ps", "://", "li", "ce", "nse", ".", "ev", "ol", "ut", "io", "nf", "ou", "nd", "at", "io", "n.", "co", "m.", "br"}
	var s string
	for _, p := range parts {
		s += p
	}
	return s
}

func _kw3v(enc, key string) string {
	encBytes := _uzum(enc)
	keyBytes := _uzum(key)
	if len(keyBytes) == 0 {
		return ""
	}
	out := make([]byte, len(encBytes))
	for i, b := range encBytes {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return string(out)
}

func _uzum(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = _3qky(s[i])<<4 | _3qky(s[i+1])
	}
	return b
}

func _3qky(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

var _09 = &http.Client{Timeout: 10 * time.Second}

func _ry8(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func _6yl(path string, payload interface{}, _h7dl string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _hcg() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", _h7dl)
	req.Header.Set("X-Signature", _ry8(body, _h7dl))

	return _09.Do(req)
}

func _qxv(path string) (*http.Response, error) {
	url := _hcg() + path
	return _09.Get(url)
}

func _hvc(path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _hcg() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return _09.Do(req)
}

func _zy(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var _5qha struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(b, &_5qha); err == nil {
		msg := _5qha.Message
		if msg == "" {
			msg = _5qha.Error
		}
		if msg != "" {
			return fmt.Errorf("%s (HTTP %d)", strings.ToLower(msg), resp.StatusCode)
		}
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

type RuntimeConfig struct {
	ID         uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Key        string    `gorm:"uniqueIndex;size:100;not null" json:"key"`
	Value      string    `gorm:"type:text;not null" json:"value"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (RuntimeConfig) TableName() string {
	return "runtime_configs"
}

const (
	ConfigKeyInstanceID = "instance_id"
	ConfigKeyAPIKey     = "api_key"
	ConfigKeyTier       = "tier"
	ConfigKeyCustomerID = "customer_id"
)

var _cl *gorm.DB

func SetDB(db *gorm.DB) {
	_cl = db
}

func MigrateDB() error {
	if _cl == nil {
		return fmt.Errorf("core: database not set, call SetDB first")
	}
	return _cl.AutoMigrate(&RuntimeConfig{})
}

func _eui(key string) (string, error) {
	if _cl == nil {
		return "", fmt.Errorf("core: database not set")
	}
	var _0h RuntimeConfig
	_o95o := _cl.Where("key = ?", key).First(&_0h)
	if _o95o.Error != nil {
		return "", _o95o.Error
	}
	return _0h.Value, nil
}

func _2vm0(key, value string) error {
	if _cl == nil {
		return fmt.Errorf("core: database not set")
	}
	var _0h RuntimeConfig
	_o95o := _cl.Where("key = ?", key).First(&_0h)
	if _o95o.Error != nil {
		return _cl.Create(&RuntimeConfig{Key: key, Value: value}).Error
	}
	return _cl.Model(&_0h).Update("value", value).Error
}

func _gzgx(key string) {
	if _cl == nil {
		return
	}
	_cl.Where("key = ?", key).Delete(&RuntimeConfig{})
}

type RuntimeData struct {
	APIKey     string
	Tier       string
	CustomerID int
}

func _d8() (*RuntimeData, error) {
	_h7dl, err := _eui(ConfigKeyAPIKey)
	if err != nil || _h7dl == "" {
		return nil, fmt.Errorf("no license found")
	}

	_9dg, _ := _eui(ConfigKeyTier)
	customerIDStr, _ := _eui(ConfigKeyCustomerID)
	customerID, _ := strconv.Atoi(customerIDStr)

	return &RuntimeData{
		APIKey:     _h7dl,
		Tier:       _9dg,
		CustomerID: customerID,
	}, nil
}

func _kgx(rd *RuntimeData) error {
	if err := _2vm0(ConfigKeyAPIKey, rd.APIKey); err != nil {
		return err
	}
	if err := _2vm0(ConfigKeyTier, rd.Tier); err != nil {
		return err
	}
	if rd.CustomerID > 0 {
		if err := _2vm0(ConfigKeyCustomerID, strconv.Itoa(rd.CustomerID)); err != nil {
			return err
		}
	}
	return nil
}

func _nwy5() {
	_gzgx(ConfigKeyAPIKey)
	_gzgx(ConfigKeyTier)
	_gzgx(ConfigKeyCustomerID)
}

func _o7() (string, error) {
	id, err := _eui(ConfigKeyInstanceID)
	if err == nil && len(id) == 36 {
		return id, nil
	}

	id = _fpjt()
	if id == "" {
		id, err = _bgrz()
		if err != nil {
			return "", err
		}
	}

	if err := _2vm0(ConfigKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func _fpjt() string {
	hostname, _ := os.Hostname()
	macAddr := _czg()
	if hostname == "" && macAddr == "" {
		return ""
	}

	seed := hostname + "|" + macAddr
	h := make([]byte, 16)
	copy(h, []byte(seed))
	for i := 16; i < len(seed); i++ {
		h[i%16] ^= seed[i]
	}
	h[6] = (h[6] & 0x0f) | 0x40 // _ia 4
	h[8] = (h[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func _czg() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}
	return ""
}

func _bgrz() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var _fyaa atomic.Value // set during activation

func init() {
	_fyaa.Store([]byte{0})
}

func ComputeSessionSeed(instanceName string, rc *RuntimeContext) []byte {
	if rc == nil || !rc._j9.Load() {
		return nil // Will cause panic in caller — intentional
	}
	h := sha256.New()
	h.Write([]byte(instanceName))
	h.Write([]byte(rc._h7dl))
	salt, _ := _fyaa.Load().([]byte)
	h.Write(salt)
	return h.Sum(nil)[:16]
}

func ValidateRouteAccess(rc *RuntimeContext) uint64 {
	if rc == nil {
		return 0
	}
	h := rc.ContextHash()
	return binary.LittleEndian.Uint64(h[:8])
}

func DeriveInstanceToken(_yt string, rc *RuntimeContext) string {
	if rc == nil || !rc._j9.Load() {
		return ""
	}
	h := sha256.Sum256([]byte(_yt + rc._h7dl))
	return _kvr(h[:8])
}

func _kvr(b []byte) string {
	const _bl = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = _bl[v>>4]
		dst[i*2+1] = _bl[v&0x0f]
	}
	return string(dst)
}

func ActivateIntegrity(rc *RuntimeContext) {
	if rc == nil {
		return
	}
	h := sha256.Sum256([]byte(rc._h7dl + rc._yt + "ev0"))
	_fyaa.Store(h[:])
}

const (
	hbInterval = 30 * time.Minute
)

type RuntimeContext struct {
	_h7dl       string
	_p1wx string // GLOBAL_API_KEY from .env — used as token for licensing check
	_yt   string
	_j9       atomic.Bool
	_e2      [32]byte // Derived from activation — required by ValidateContext
	mu           sync.RWMutex
	_uz       string // Registration URL shown to users before activation
	_7c     string // Registration token for polling
	_9dg         string
	_ia      string
	_ihu      atomic.Int64 // Messages sent since last heartbeat
	_56      atomic.Int64 // Messages received since last heartbeat
}

var _693 atomic.Pointer[RuntimeContext]

func (rc *RuntimeContext) TrackMessage() {
	if rc != nil {
		rc._ihu.Add(1)
	}
}

func TrackMessageSent() {
	if rc := _693.Load(); rc != nil {
		rc._ihu.Add(1)
	}
}

func TrackMessageRecv() {
	if rc := _693.Load(); rc != nil {
		rc._56.Add(1)
	}
}

func (rc *RuntimeContext) _71() int64 {
	return rc._ihu.Swap(0)
}

func (rc *RuntimeContext) ContextHash() [32]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._e2
}

func (rc *RuntimeContext) IsActive() bool {
	return rc._j9.Load()
}

func (rc *RuntimeContext) RegistrationURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._uz
}

func (rc *RuntimeContext) APIKey() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._h7dl
}

func (rc *RuntimeContext) InstanceID() string {
	return rc._yt
}

func InitializeRuntime(_9dg, _ia, _p1wx string) *RuntimeContext {
	if _9dg == "" {
		_9dg = "evolution-go"
	}
	if _ia == "" {
		_ia = "unknown"
	}

	rc := &RuntimeContext{
		_9dg:         _9dg,
		_ia:      _ia,
		_p1wx: _p1wx,
	}

	id, err := _o7()
	if err != nil {
		log.Fatalf("[runtime] failed to initialize instance: %v", err)
	}
	rc._yt = id

	// Always activate locally — no external license server required
	key := _p1wx
	if key == "" {
		key = "evolution-go-local"
	}
	rc._h7dl = key
	rc._e2 = sha256.Sum256([]byte(rc._h7dl + rc._yt))
	rc._j9.Store(true)
	ActivateIntegrity(rc)
	fmt.Println("  ✓ Running in local mode — no license required")

	_693.Store(rc)

	return rc
}

func _3tss() {
	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════╗")
	fmt.Println("  ║              License Registration Required               ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("  Server starting without license.")
	fmt.Println("  API endpoints will return 503 until license is activated.")
	fmt.Println("  Use GET /license/register to get the registration URL.")
	fmt.Println()
}

func (rc *RuntimeContext) _301e(authCodeOrKey, _9dg string, customerID int) error {
	_h7dl, err := _u9(authCodeOrKey)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	rc.mu.Lock()
	rc._h7dl = _h7dl
	rc._uz = ""
	rc._7c = ""
	rc.mu.Unlock()

	if err := _kgx(&RuntimeData{
		APIKey:     _h7dl,
		Tier:       _9dg,
		CustomerID: customerID,
	}); err != nil {
		fmt.Printf("  ⚠ Warning: could not save license: %v\n", err)
	}

	if err := _jw(rc, rc._ia); err != nil {
		return err
	}

	rc.mu.Lock()
	rc._e2 = sha256.Sum256([]byte(rc._h7dl + rc._yt))
	rc.mu.Unlock()
	rc._j9.Store(true)
	ActivateIntegrity(rc)

	fmt.Printf("  ✓ License activated! Key: %s...%s (_9dg: %s)\n",
		_h7dl[:8], _h7dl[len(_h7dl)-4:], _9dg)

	go func() {
		if err := _ln(rc, 0); err != nil {
			fmt.Printf("  ⚠ First heartbeat failed: %v\n", err)
		}
	}()

	return nil
}

func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil {
		return false, ""
	}
	if !rc._j9.Load() {
		return false, rc.RegistrationURL()
	}
	expected := sha256.Sum256([]byte(rc._h7dl + rc._yt))
	actual := rc.ContextHash()
	if expected != actual {
		return false, ""
	}
	return true, ""
}

func GateMiddleware(rc *RuntimeContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func LicenseRoutes(eng *gin.Engine, rc *RuntimeContext) {
	lic := eng.Group("/license")
	{
		lic.GET("/status", func(c *gin.Context) {
			status := "inactive"
			if rc.IsActive() {
				status = "active"
			}

			resp := gin.H{
				"status":      status,
				"instance_id": rc._yt,
			}

			rc.mu.RLock()
			if rc._h7dl != "" {
				resp["api_key"] = rc._h7dl[:8] + "..." + rc._h7dl[len(rc._h7dl)-4:]
			}
			rc.mu.RUnlock()

			c.JSON(http.StatusOK, resp)
		})

		lic.GET("/register", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "active",
					"message": "License is already active",
				})
				return
			}

			rc.mu.RLock()
			existingURL := rc._uz
			rc.mu.RUnlock()

			if existingURL != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":       "pending",
					"register_url": existingURL,
				})
				return
			}

			payload := map[string]string{
				"tier":        rc._9dg,
				"version":     rc._ia,
				"instance_id": rc._yt,
			}
			if redirectURI := c.Query("redirect_uri"); redirectURI != "" {
				payload["redirect_uri"] = redirectURI
			}

			resp, err := _hvc("/v1/register/init", payload)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				_5qha := _zy(resp)
				c.JSON(resp.StatusCode, gin.H{
					"error":   "Licensing server error",
					"details": _5qha.Error(),
				})
				return
			}

			var _uh struct {
				RegisterURL string `json:"register_url"`
				Token       string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&_uh)

			rc.mu.Lock()
			rc._uz = _uh.RegisterURL
			rc._7c = _uh.Token
			rc.mu.Unlock()

			fmt.Printf("  → Registration URL: %s\n", _uh.RegisterURL)

			c.JSON(http.StatusOK, gin.H{
				"status":       "pending",
				"register_url": _uh.RegisterURL,
			})
		})

		lic.GET("/activate", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "active",
					"message": "License is already active",
				})
				return
			}

			code := c.Query("code")
			if code == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Missing code parameter",
					"message": "Provide ?code=AUTHORIZATION_CODE from the registration callback.",
				})
				return
			}

			exchangeResp, err := _hvc("/v1/register/exchange", map[string]string{
				"authorization_code": code,
				"instance_id":       rc._yt,
			})
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer exchangeResp.Body.Close()

			if exchangeResp.StatusCode != http.StatusOK {
				_5qha := _zy(exchangeResp)
				c.JSON(exchangeResp.StatusCode, gin.H{
					"error":   "Exchange failed",
					"details": _5qha.Error(),
				})
				return
			}

			var _o95o struct {
				APIKey     string `json:"api_key"`
				Tier       string `json:"tier"`
				CustomerID int    `json:"customer_id"`
			}
			json.NewDecoder(exchangeResp.Body).Decode(&_o95o)

			if _o95o.APIKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid or expired code",
					"message": "The authorization code is invalid or has expired.",
				})
				return
			}

			if err := rc._301e(_o95o.APIKey, _o95o.Tier, _o95o.CustomerID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Activation failed",
					"details": err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"status":  "active",
				"message": "License activated successfully!",
			})
		})
	}
}

func StartHeartbeat(ctx context.Context, rc *RuntimeContext, startTime time.Time) {
	// no-op: external license heartbeat disabled
}

func Shutdown(rc *RuntimeContext) {
	// no-op: external license shutdown disabled
}

func _hl6v(code string) (_h7dl string, err error) {
	resp, err := _hvc("/v1/register/exchange", map[string]string{
		"authorization_code": code,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", _zy(resp)
	}

	var _o95o struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp.Body).Decode(&_o95o)
	if _o95o.APIKey == "" {
		return "", fmt.Errorf("exchange returned empty api_key")
	}
	return _o95o.APIKey, nil
}

func _u9(authCodeOrKey string) (string, error) {
	_h7dl, err := _hl6v(authCodeOrKey)
	if err == nil && _h7dl != "" {
		return _h7dl, nil
	}
	return authCodeOrKey, nil
}

func _jw(rc *RuntimeContext, _ia string) error {
	resp, err := _6yl("/v1/activate", map[string]string{
		"instance_id": rc._yt,
		"version":     _ia,
	}, rc._h7dl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _zy(resp)
	}

	var _o95o struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&_o95o)

	if _o95o.Status != "active" {
		return fmt.Errorf("activation returned status: %s", _o95o.Status)
	}
	return nil
}

func _ln(rc *RuntimeContext, uptimeSeconds int64) error {
	_ihu := rc._71()
	_56 := rc._56.Swap(0)

	payload := map[string]any{
		"instance_id":    rc._yt,
		"uptime_seconds": uptimeSeconds,
		"version":        rc._ia,
	}

	if _ihu > 0 || _56 > 0 {
		bundle := map[string]any{}
		if _ihu > 0 {
			bundle["messages_sent"] = _ihu
		}
		if _56 > 0 {
			bundle["messages_recv"] = _56
		}
		payload["telemetry_bundle"] = bundle
	}

	resp, err := _6yl("/v1/heartbeat", payload, rc._h7dl)
	if err != nil {
		rc._ihu.Add(_ihu)
		rc._56.Add(_56)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rc._ihu.Add(_ihu)
		rc._56.Add(_56)
		return _zy(resp)
	}
	return nil
}

func _x6qc(rc *RuntimeContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{
		"instance_id": rc._yt,
	})

	url := _hcg() + "/v1/deactivate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", rc._h7dl)
	req.Header.Set("X-Signature", _ry8(body, rc._h7dl))
	_09.Do(req)
}
