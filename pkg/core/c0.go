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

var _k1 = []byte{0xd3, 0xd0, 0x41, 0x5a, 0x76, 0xa7, 0x40, 0x81, 0x44, 0xa8, 0xac, 0x8c, 0x64, 0xaa, 0x13, 0x12, 0x16, 0x71, 0x9d, 0x13, 0x9a, 0x41, 0x57, 0xd9, 0x15, 0x62, 0x1a, 0x08, 0x1f, 0x6c, 0x6c, 0x7b, 0xd3, 0xa4, 0x81, 0xca, 0x85, 0xeb, 0x9f, 0x06, 0x81, 0x0a}
var _k0 = []byte{0xbb, 0xa4, 0x35, 0x2a, 0x05, 0x9d, 0x6f, 0xae, 0x28, 0xc1, 0xcf, 0xe9, 0x0a, 0xd9, 0x76, 0x3c, 0x73, 0x07, 0xf2, 0x7f, 0xef, 0x35, 0x3e, 0xb6, 0x7b, 0x04, 0x75, 0x7d, 0x71, 0x08, 0x0d, 0x0f, 0xba, 0xcb, 0xef, 0xe4, 0xe6, 0x84, 0xf2, 0x28, 0xe3, 0x78}

var (
	_6np1 string
	_96    string
)

func _cdo() string {
	if _6np1 != "" && _96 != "" {
		return _k54v(_6np1, _96)
	}
	parts := [...]string{"h", "tt", "ps", "://", "li", "ce", "nse", ".", "ev", "ol", "ut", "io", "nf", "ou", "nd", "at", "io", "n.", "co", "m.", "br"}
	var s string
	for _, p := range parts {
		s += p
	}
	return s
}

func _k54v(enc, key string) string {
	encBytes := _9wc0(enc)
	keyBytes := _9wc0(key)
	if len(keyBytes) == 0 {
		return ""
	}
	out := make([]byte, len(encBytes))
	for i, b := range encBytes {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return string(out)
}

func _9wc0(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = _gy4(s[i])<<4 | _gy4(s[i+1])
	}
	return b
}

func _gy4(c byte) byte {
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

var _3t = &http.Client{Timeout: 10 * time.Second}

func _4crw(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func _sn(path string, payload interface{}, _kni string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _cdo() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", _kni)
	req.Header.Set("X-Signature", _4crw(body, _kni))

	return _3t.Do(req)
}

func _6o(path string) (*http.Response, error) {
	url := _cdo() + path
	return _3t.Get(url)
}

func _dtnx(path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _cdo() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return _3t.Do(req)
}

func _3ya(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var _n6oe struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(b, &_n6oe); err == nil {
		msg := _n6oe.Message
		if msg == "" {
			msg = _n6oe.Error
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

var _k4 *gorm.DB

func SetDB(db *gorm.DB) {
	_k4 = db
}

func MigrateDB() error {
	if _k4 == nil {
		return fmt.Errorf("core: database not set, call SetDB first")
	}
	return _k4.AutoMigrate(&RuntimeConfig{})
}

func _at(key string) (string, error) {
	if _k4 == nil {
		return "", fmt.Errorf("core: database not set")
	}
	var _33 RuntimeConfig
	_tmzn := _k4.Where("key = ?", key).First(&_33)
	if _tmzn.Error != nil {
		return "", _tmzn.Error
	}
	return _33.Value, nil
}

func _yy(key, value string) error {
	if _k4 == nil {
		return fmt.Errorf("core: database not set")
	}
	var _33 RuntimeConfig
	_tmzn := _k4.Where("key = ?", key).First(&_33)
	if _tmzn.Error != nil {
		return _k4.Create(&RuntimeConfig{Key: key, Value: value}).Error
	}
	return _k4.Model(&_33).Update("value", value).Error
}

func _ettg(key string) {
	if _k4 == nil {
		return
	}
	_k4.Where("key = ?", key).Delete(&RuntimeConfig{})
}

type RuntimeData struct {
	APIKey     string
	Tier       string
	CustomerID int
}

func _2s() (*RuntimeData, error) {
	_kni, err := _at(ConfigKeyAPIKey)
	if err != nil || _kni == "" {
		return nil, fmt.Errorf("no license found")
	}

	_b56, _ := _at(ConfigKeyTier)
	customerIDStr, _ := _at(ConfigKeyCustomerID)
	customerID, _ := strconv.Atoi(customerIDStr)

	return &RuntimeData{
		APIKey:     _kni,
		Tier:       _b56,
		CustomerID: customerID,
	}, nil
}

func _yosh(rd *RuntimeData) error {
	if err := _yy(ConfigKeyAPIKey, rd.APIKey); err != nil {
		return err
	}
	if err := _yy(ConfigKeyTier, rd.Tier); err != nil {
		return err
	}
	if rd.CustomerID > 0 {
		if err := _yy(ConfigKeyCustomerID, strconv.Itoa(rd.CustomerID)); err != nil {
			return err
		}
	}
	return nil
}

func _31() {
	_ettg(ConfigKeyAPIKey)
	_ettg(ConfigKeyTier)
	_ettg(ConfigKeyCustomerID)
}

func _ggnz() (string, error) {
	id, err := _at(ConfigKeyInstanceID)
	if err == nil && len(id) == 36 {
		return id, nil
	}

	id = _tym7()
	if id == "" {
		id, err = _ebxz()
		if err != nil {
			return "", err
		}
	}

	if err := _yy(ConfigKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func _tym7() string {
	hostname, _ := os.Hostname()
	macAddr := _nteb()
	if hostname == "" && macAddr == "" {
		return ""
	}

	seed := hostname + "|" + macAddr
	h := make([]byte, 16)
	copy(h, []byte(seed))
	for i := 16; i < len(seed); i++ {
		h[i%16] ^= seed[i]
	}
	h[6] = (h[6] & 0x0f) | 0x40 // _64 4
	h[8] = (h[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func _nteb() string {
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

func _ebxz() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var _x1n atomic.Value // set during activation

func init() {
	_x1n.Store([]byte{0})
}

func ComputeSessionSeed(instanceName string, rc *RuntimeContext) []byte {
	if rc == nil || !rc._txz.Load() {
		return nil // Will cause panic in caller — intentional
	}
	h := sha256.New()
	h.Write([]byte(instanceName))
	h.Write([]byte(rc._kni))
	salt, _ := _x1n.Load().([]byte)
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

func DeriveInstanceToken(_z14 string, rc *RuntimeContext) string {
	if rc == nil || !rc._txz.Load() {
		return ""
	}
	h := sha256.Sum256([]byte(_z14 + rc._kni))
	return _zxx(h[:8])
}

func _zxx(b []byte) string {
	const _4jq = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = _4jq[v>>4]
		dst[i*2+1] = _4jq[v&0x0f]
	}
	return string(dst)
}

func ActivateIntegrity(rc *RuntimeContext) {
	if rc == nil {
		return
	}
	h := sha256.Sum256([]byte(rc._kni + rc._z14 + "ev0"))
	_x1n.Store(h[:])
}

const (
	hbInterval = 30 * time.Minute
)

type RuntimeContext struct {
	_kni       string
	_pl87 string // GLOBAL_API_KEY from .env — used as token for licensing check
	_z14   string
	_txz       atomic.Bool
	_s6a      [32]byte // Derived from activation — required by ValidateContext
	mu           sync.RWMutex
	_v8       string // Registration URL shown to users before activation
	_0z9m     string // Registration token for polling
	_b56         string
	_64      string
	_hpv      atomic.Int64 // Messages sent since last heartbeat
	_ti9      atomic.Int64 // Messages received since last heartbeat
}

var _rs atomic.Pointer[RuntimeContext]

func (rc *RuntimeContext) TrackMessage() {
	if rc != nil {
		rc._hpv.Add(1)
	}
}

func TrackMessageSent() {
	if rc := _rs.Load(); rc != nil {
		rc._hpv.Add(1)
	}
}

func TrackMessageRecv() {
	if rc := _rs.Load(); rc != nil {
		rc._ti9.Add(1)
	}
}

func (rc *RuntimeContext) _4g() int64 {
	return rc._hpv.Swap(0)
}

func (rc *RuntimeContext) ContextHash() [32]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._s6a
}

func (rc *RuntimeContext) IsActive() bool {
	return rc._txz.Load()
}

func (rc *RuntimeContext) RegistrationURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._v8
}

func (rc *RuntimeContext) APIKey() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._kni
}

func (rc *RuntimeContext) InstanceID() string {
	return rc._z14
}

func InitializeRuntime(_b56, _64, _pl87 string) *RuntimeContext {
	if _b56 == "" {
		_b56 = "evolution-go"
	}
	if _64 == "" {
		_64 = "unknown"
	}

	rc := &RuntimeContext{
		_b56:         _b56,
		_64:      _64,
		_pl87: _pl87,
	}

	id, err := _ggnz()
	if err != nil {
		log.Fatalf("[runtime] failed to initialize instance: %v", err)
	}
	rc._z14 = id

	rd, err := _2s()
	if err == nil && rd.APIKey != "" {
		rc._kni = rd.APIKey
		fmt.Printf("  ✓ License found: %s...%s\n", rd.APIKey[:8], rd.APIKey[len(rd.APIKey)-4:])

		rc._s6a = sha256.Sum256([]byte(rc._kni + rc._z14))
		rc._txz.Store(true)
		ActivateIntegrity(rc)
		fmt.Println("  ✓ License activated successfully")

		go func() {
			if err := _c4m(rc, _64); err != nil {
				fmt.Printf("  ⚠ Remote activation notice failed (non-blocking): %v\n", err)
			}
		}()
	} else if rc._pl87 != "" {
		rc._kni = rc._pl87
		if err := _c4m(rc, _64); err == nil {
			_yosh(&RuntimeData{APIKey: rc._pl87, Tier: _b56})
			rc._s6a = sha256.Sum256([]byte(rc._kni + rc._z14))
			rc._txz.Store(true)
			ActivateIntegrity(rc)
			fmt.Printf("  ✓ GLOBAL_API_KEY accepted — license saved and activated\n")
		} else {
			rc._kni = ""
			_g2()
			rc._txz.Store(false)
		}
	} else {
		if _rh(rc, _64) {
			fmt.Println("  ✓ License activated automatically via EVOLUTION_OPERATOR_EMAIL")
		} else {
			_g2()
			rc._txz.Store(false)
		}
	}

	_rs.Store(rc)

	return rc
}

func _rh(rc *RuntimeContext, _64 string) bool {
	email := strings.TrimSpace(os.Getenv("EVOLUTION_OPERATOR_EMAIL"))
	if email == "" {
		return false
	}

	payload := map[string]string{
		"email":       email,
		"tier":        rc._b56,
		"version":     _64,
		"instance_id": rc._z14,
	}

	resp, err := _dtnx("/v1/register/auto", payload)
	if err != nil {
		fmt.Printf("  ⚠ Auto-activation skipped — licensing server unreachable: %v\n", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_n6oe := _3ya(resp)
		if resp.StatusCode == http.StatusNotFound {
			fmt.Printf("  ℹ Auto-activation skipped — email not registered yet (first time?). Falling back to manual flow.\n")
		} else {
			fmt.Printf("  ⚠ Auto-activation rejected (%d): %v. Falling back to manual flow.\n",
				resp.StatusCode, _n6oe)
		}
		return false
	}

	var _tmzn struct {
		APIKey     string `json:"api_key"`
		CustomerID int    `json:"customer_id"`
		Tier       string `json:"tier"`
		Status     string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&_tmzn); err != nil {
		fmt.Printf("  ⚠ Auto-activation response malformed: %v\n", err)
		return false
	}
	if _tmzn.APIKey == "" {
		fmt.Printf("  ⚠ Auto-activation response missing api_key\n")
		return false
	}

	rc.mu.Lock()
	rc._kni = _tmzn.APIKey
	rc.mu.Unlock()

	if err := _yosh(&RuntimeData{
		APIKey:     _tmzn.APIKey,
		Tier:       rc._b56,
		CustomerID: _tmzn.CustomerID,
	}); err != nil {
		fmt.Printf("  ⚠ Auto-activation: could not save license to disk: %v\n", err)
	}

	rc.mu.Lock()
	rc._s6a = sha256.Sum256([]byte(rc._kni + rc._z14))
	rc.mu.Unlock()
	rc._txz.Store(true)
	ActivateIntegrity(rc)
	return true
}

func _g2() {
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

func (rc *RuntimeContext) _bf8(authCodeOrKey, _b56 string, customerID int) error {
	_kni, err := _58(authCodeOrKey)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	rc.mu.Lock()
	rc._kni = _kni
	rc._v8 = ""
	rc._0z9m = ""
	rc.mu.Unlock()

	if err := _yosh(&RuntimeData{
		APIKey:     _kni,
		Tier:       _b56,
		CustomerID: customerID,
	}); err != nil {
		fmt.Printf("  ⚠ Warning: could not save license: %v\n", err)
	}

	if err := _c4m(rc, rc._64); err != nil {
		return err
	}

	rc.mu.Lock()
	rc._s6a = sha256.Sum256([]byte(rc._kni + rc._z14))
	rc.mu.Unlock()
	rc._txz.Store(true)
	ActivateIntegrity(rc)

	fmt.Printf("  ✓ License activated! Key: %s...%s (_b56: %s)\n",
		_kni[:8], _kni[len(_kni)-4:], _b56)

	go func() {
		if err := _814l(rc, 0); err != nil {
			fmt.Printf("  ⚠ First heartbeat failed: %v\n", err)
		}
	}()

	return nil
}

func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil {
		return false, ""
	}
	if !rc._txz.Load() {
		return false, rc.RegistrationURL()
	}
	expected := sha256.Sum256([]byte(rc._kni + rc._z14))
	actual := rc.ContextHash()
	if expected != actual {
		return false, ""
	}
	return true, ""
}

func GateMiddleware(rc *RuntimeContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		if path == "/health" || path == "/server/ok" || path == "/favicon.ico" ||
			path == "/license/status" || path == "/license/register" || path == "/license/activate" ||
			strings.HasPrefix(path, "/manager") || strings.HasPrefix(path, "/assets") ||
			strings.HasPrefix(path, "/passkey-ceremony") ||
			strings.HasPrefix(path, "/swagger") || path == "/ws" ||
			strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".css") ||
			strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".woff") || strings.HasSuffix(path, ".ttf") {
			c.Next()
			return
		}

		valid, _ := ValidateContext(rc)
		if !valid {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			managerURL := fmt.Sprintf("%s://%s/manager/login", scheme, c.Request.Host)

			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"error":        "service not activated",
				"code":         "LICENSE_REQUIRED",
				"register_url": managerURL,
				"message":      "License required. Open the manager to activate your license.",
			})
			return
		}

		c.Set("_rch", rc.ContextHash())
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
				"instance_id": rc._z14,
			}

			rc.mu.RLock()
			if rc._kni != "" {
				resp["api_key"] = rc._kni[:8] + "..." + rc._kni[len(rc._kni)-4:]
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
			existingURL := rc._v8
			rc.mu.RUnlock()

			if existingURL != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":       "pending",
					"register_url": existingURL,
				})
				return
			}

			payload := map[string]string{
				"tier":        rc._b56,
				"version":     rc._64,
				"instance_id": rc._z14,
			}
			if redirectURI := c.Query("redirect_uri"); redirectURI != "" {
				payload["redirect_uri"] = redirectURI
			}

			resp, err := _dtnx("/v1/register/init", payload)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				_n6oe := _3ya(resp)
				c.JSON(resp.StatusCode, gin.H{
					"error":   "Licensing server error",
					"details": _n6oe.Error(),
				})
				return
			}

			var _3y struct {
				RegisterURL string `json:"register_url"`
				Token       string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&_3y)

			rc.mu.Lock()
			rc._v8 = _3y.RegisterURL
			rc._0z9m = _3y.Token
			rc.mu.Unlock()

			fmt.Printf("  → Registration URL: %s\n", _3y.RegisterURL)

			c.JSON(http.StatusOK, gin.H{
				"status":       "pending",
				"register_url": _3y.RegisterURL,
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

			exchangeResp, err := _dtnx("/v1/register/exchange", map[string]string{
				"authorization_code": code,
				"instance_id":       rc._z14,
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
				_n6oe := _3ya(exchangeResp)
				c.JSON(exchangeResp.StatusCode, gin.H{
					"error":   "Exchange failed",
					"details": _n6oe.Error(),
				})
				return
			}

			var _tmzn struct {
				APIKey     string `json:"api_key"`
				Tier       string `json:"tier"`
				CustomerID int    `json:"customer_id"`
			}
			json.NewDecoder(exchangeResp.Body).Decode(&_tmzn)

			if _tmzn.APIKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid or expired code",
					"message": "The authorization code is invalid or has expired.",
				})
				return
			}

			if err := rc._bf8(_tmzn.APIKey, _tmzn.Tier, _tmzn.CustomerID); err != nil {
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
	go func() {
		ticker := time.NewTicker(hbInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !rc.IsActive() {
					continue
				}
				uptime := int64(time.Since(startTime).Seconds())
				if err := _814l(rc, uptime); err != nil {
					fmt.Printf("  ⚠ Heartbeat failed (non-blocking): %v\n", err)
				}
			}
		}
	}()
}

func Shutdown(rc *RuntimeContext) {
	if rc == nil || rc._kni == "" {
		return
	}
	_wj(rc)
}

func _pl(code string) (_kni string, err error) {
	resp, err := _dtnx("/v1/register/exchange", map[string]string{
		"authorization_code": code,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", _3ya(resp)
	}

	var _tmzn struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp.Body).Decode(&_tmzn)
	if _tmzn.APIKey == "" {
		return "", fmt.Errorf("exchange returned empty api_key")
	}
	return _tmzn.APIKey, nil
}

func _58(authCodeOrKey string) (string, error) {
	_kni, err := _pl(authCodeOrKey)
	if err == nil && _kni != "" {
		return _kni, nil
	}
	return authCodeOrKey, nil
}

func _c4m(rc *RuntimeContext, _64 string) error {
	resp, err := _sn("/v1/activate", map[string]string{
		"instance_id": rc._z14,
		"version":     _64,
	}, rc._kni)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _3ya(resp)
	}

	var _tmzn struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&_tmzn)

	if _tmzn.Status != "active" {
		return fmt.Errorf("activation returned status: %s", _tmzn.Status)
	}
	return nil
}

func _814l(rc *RuntimeContext, uptimeSeconds int64) error {
	_hpv := rc._4g()
	_ti9 := rc._ti9.Swap(0)

	payload := map[string]any{
		"instance_id":    rc._z14,
		"uptime_seconds": uptimeSeconds,
		"version":        rc._64,
	}

	if _hpv > 0 || _ti9 > 0 {
		bundle := map[string]any{}
		if _hpv > 0 {
			bundle["messages_sent"] = _hpv
		}
		if _ti9 > 0 {
			bundle["messages_recv"] = _ti9
		}
		payload["telemetry_bundle"] = bundle
	}

	resp, err := _sn("/v1/heartbeat", payload, rc._kni)
	if err != nil {
		rc._hpv.Add(_hpv)
		rc._ti9.Add(_ti9)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		rc._hpv.Add(_hpv)
		rc._ti9.Add(_ti9)
		return _3ya(resp)
	}
	return nil
}

func _wj(rc *RuntimeContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{
		"instance_id": rc._z14,
	})

	url := _cdo() + "/v1/deactivate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", rc._kni)
	req.Header.Set("X-Signature", _4crw(body, rc._kni))
	_3t.Do(req)
}
