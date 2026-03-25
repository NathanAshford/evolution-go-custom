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

var _k1 = []byte{0xe8, 0x1d, 0x6b, 0x81, 0x27, 0xa5, 0x5e, 0x63, 0x7a, 0xfd, 0xa7, 0x47, 0x6b, 0x9c, 0xde, 0xa0, 0xa0, 0x05, 0xe9, 0x8f, 0x58, 0x17, 0xac, 0x43, 0xcd, 0xb4, 0x35, 0x97, 0x02, 0x10, 0x27, 0x55, 0x1c, 0x28, 0x58, 0x4b, 0x0d, 0x24, 0x64, 0x60, 0x7e, 0x2a}
var _k0 = []byte{0x80, 0x69, 0x1f, 0xf1, 0x54, 0x9f, 0x71, 0x4c, 0x16, 0x94, 0xc4, 0x22, 0x05, 0xef, 0xbb, 0x8e, 0xc5, 0x73, 0x86, 0xe3, 0x2d, 0x63, 0xc5, 0x2c, 0xa3, 0xd2, 0x5a, 0xe2, 0x6c, 0x74, 0x46, 0x21, 0x75, 0x47, 0x36, 0x65, 0x6e, 0x4b, 0x09, 0x4e, 0x1c, 0x58}

var (
	_83 string
	_xazo    string
)

func _40hs() string {
	if _83 != "" && _xazo != "" {
		return _zh(_83, _xazo)
	}
	parts := [...]string{"h", "tt", "ps", "://", "li", "ce", "nse", ".", "ev", "ol", "ut", "io", "nf", "ou", "nd", "at", "io", "n.", "co", "m.", "br"}
	var s string
	for _, p := range parts {
		s += p
	}
	return s
}

func _zh(enc, key string) string {
	encBytes := _5b7(enc)
	keyBytes := _5b7(key)
	if len(keyBytes) == 0 {
		return ""
	}
	out := make([]byte, len(encBytes))
	for i, b := range encBytes {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return string(out)
}

func _5b7(s string) []byte {
	if len(s)%2 != 0 {
		return nil
	}
	b := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b[i/2] = _vtc(s[i])<<4 | _vtc(s[i+1])
	}
	return b
}

func _vtc(c byte) byte {
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

var _ml9 = &http.Client{Timeout: 10 * time.Second}

func _pq(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func _c14s(path string, payload interface{}, _d1e9 string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _40hs() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", _d1e9)
	req.Header.Set("X-Signature", _pq(body, _d1e9))

	return _ml9.Do(req)
}

func _46(path string) (*http.Response, error) {
	url := _40hs() + path
	return _ml9.Get(url)
}

func _0r(path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := _40hs() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return _ml9.Do(req)
}

func _36l2(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var _f3mf struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(b, &_f3mf); err == nil {
		msg := _f3mf.Message
		if msg == "" {
			msg = _f3mf.Error
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
	ConfigKeyTier       = "_i7xh"
	ConfigKeyCustomerID = "customer_id"
)

var _to *gorm.DB

func SetDB(db *gorm.DB) {
	_to = db
}

func MigrateDB() error {
	if _to == nil {
		return fmt.Errorf("core: database not set, call SetDB first")
	}
	return _to.AutoMigrate(&RuntimeConfig{})
}

func _o07s(key string) (string, error) {
	if _to == nil {
		return "", fmt.Errorf("core: database not set")
	}
	var _4k RuntimeConfig
	_2q := _to.Where("key = ?", key).First(&_4k)
	if _2q.Error != nil {
		return "", _2q.Error
	}
	return _4k.Value, nil
}

func _y7(key, value string) error {
	if _to == nil {
		return fmt.Errorf("core: database not set")
	}
	var _4k RuntimeConfig
	_2q := _to.Where("key = ?", key).First(&_4k)
	if _2q.Error != nil {
		return _to.Create(&RuntimeConfig{Key: key, Value: value}).Error
	}
	return _to.Model(&_4k).Update("value", value).Error
}

func _4k5p(key string) {
	if _to == nil {
		return
	}
	_to.Where("key = ?", key).Delete(&RuntimeConfig{})
}

type RuntimeData struct {
	APIKey     string
	Tier       string
	CustomerID int
}

func _rjz7() (*RuntimeData, error) {
	_d1e9, err := _o07s(ConfigKeyAPIKey)
	if err != nil || _d1e9 == "" {
		return nil, fmt.Errorf("no license found")
	}

	_i7xh, _ := _o07s(ConfigKeyTier)
	customerIDStr, _ := _o07s(ConfigKeyCustomerID)
	customerID, _ := strconv.Atoi(customerIDStr)

	return &RuntimeData{
		APIKey:     _d1e9,
		Tier:       _i7xh,
		CustomerID: customerID,
	}, nil
}

func _pi(rd *RuntimeData) error {
	if err := _y7(ConfigKeyAPIKey, rd.APIKey); err != nil {
		return err
	}
	if err := _y7(ConfigKeyTier, rd.Tier); err != nil {
		return err
	}
	if rd.CustomerID > 0 {
		if err := _y7(ConfigKeyCustomerID, strconv.Itoa(rd.CustomerID)); err != nil {
			return err
		}
	}
	return nil
}

func _jul3() {
	_4k5p(ConfigKeyAPIKey)
	_4k5p(ConfigKeyTier)
	_4k5p(ConfigKeyCustomerID)
}

func _sj() (string, error) {
	id, err := _o07s(ConfigKeyInstanceID)
	if err == nil && len(id) == 36 {
		return id, nil
	}

	id = _g1()
	if id == "" {
		id, err = _bmj4()
		if err != nil {
			return "", err
		}
	}

	if err := _y7(ConfigKeyInstanceID, id); err != nil {
		return "", err
	}
	return id, nil
}

func _g1() string {
	hostname, _ := os.Hostname()
	macAddr := _3yc()
	if hostname == "" && macAddr == "" {
		return ""
	}

	seed := hostname + "|" + macAddr
	h := make([]byte, 16)
	copy(h, []byte(seed))
	for i := 16; i < len(seed); i++ {
		h[i%16] ^= seed[i]
	}
	h[6] = (h[6] & 0x0f) | 0x40 // _1s8g 4
	h[8] = (h[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		h[0:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func _3yc() string {
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

func _bmj4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

var _5m atomic.Value // set during activation

func init() {
	_5m.Store([]byte{0})
}

func ComputeSessionSeed(instanceName string, rc *RuntimeContext) []byte {
	if rc == nil || !rc._oq5.Load() {
		return nil // Will cause panic in caller — intentional
	}
	h := sha256.New()
	h.Write([]byte(instanceName))
	h.Write([]byte(rc._d1e9))
	salt, _ := _5m.Load().([]byte)
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

func DeriveInstanceToken(_gj string, rc *RuntimeContext) string {
	if rc == nil || !rc._oq5.Load() {
		return ""
	}
	h := sha256.Sum256([]byte(_gj + rc._d1e9))
	return _tuvw(h[:8])
}

func _tuvw(b []byte) string {
	const _s53 = "0123456789abcdef"
	dst := make([]byte, len(b)*2)
	for i, v := range b {
		dst[i*2] = _s53[v>>4]
		dst[i*2+1] = _s53[v&0x0f]
	}
	return string(dst)
}

func ActivateIntegrity(rc *RuntimeContext) {
	if rc == nil {
		return
	}
	h := sha256.Sum256([]byte(rc._d1e9 + rc._gj + "ev0"))
	_5m.Store(h[:])
}

const (
	hbInterval = 30 * time.Minute
)

type RuntimeContext struct {
	_d1e9       string
	_96f2 string // GLOBAL_API_KEY from .env — used as token for licensing check
	_gj   string
	_oq5       atomic.Bool
	_z1uh      [32]byte // Derived from activation — required by ValidateContext
	mu           sync.RWMutex
	_34r       string // Registration URL shown to users before activation
	_8344     string // Registration token for polling
	_i7xh         string
	_1s8g      string
}

func (rc *RuntimeContext) ContextHash() [32]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._z1uh
}

func (rc *RuntimeContext) IsActive() bool {
	return rc._oq5.Load()
}

func (rc *RuntimeContext) RegistrationURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._34r
}

func (rc *RuntimeContext) APIKey() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc._d1e9
}

func (rc *RuntimeContext) InstanceID() string {
	return rc._gj
}

func InitializeRuntime(_i7xh, _1s8g, _96f2 string) *RuntimeContext {
	if _i7xh == "" {
		_i7xh = "evolution-go"
	}
	if _1s8g == "" {
		_1s8g = "unknown"
	}

	rc := &RuntimeContext{
		_i7xh:         _i7xh,
		_1s8g:      _1s8g,
		_96f2: _96f2,
	}

	id, err := _sj()
	if err != nil {
		log.Fatalf("[runtime] failed to initialize instance: %v", err)
	}
	rc._gj = id

	rd, err := _rjz7()
	if err == nil && rd.APIKey != "" {
		rc._d1e9 = rd.APIKey
		fmt.Printf("  ✓ License found: %s...%s\n", rd.APIKey[:8], rd.APIKey[len(rd.APIKey)-4:])

		rc._z1uh = sha256.Sum256([]byte(rc._d1e9 + rc._gj))
		rc._oq5.Store(true)
		ActivateIntegrity(rc)
		fmt.Println("  ✓ License activated successfully")

		go func() {
			if err := _5tmd(rc, _1s8g); err != nil {
				fmt.Printf("  ⚠ Remote activation notice failed (non-blocking): %v\n", err)
			}
		}()
	} else {
		fmt.Println()
		fmt.Println("  ╔══════════════════════════════════════════════════════════╗")
		fmt.Println("  ║              License Registration Required               ║")
		fmt.Println("  ╚══════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Println("  Server starting without license.")
		fmt.Println("  API endpoints will return 503 until license is activated.")
		fmt.Println("  Use GET /license/register to get the registration URL.")
		fmt.Println()
		rc._oq5.Store(false)
	}

	return rc
}

func (rc *RuntimeContext) _2r(authCodeOrKey, _i7xh string, customerID int) error {
	_d1e9, err := _rbx(authCodeOrKey)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	rc.mu.Lock()
	rc._d1e9 = _d1e9
	rc._34r = ""
	rc._8344 = ""
	rc.mu.Unlock()

	if err := _pi(&RuntimeData{
		APIKey:     _d1e9,
		Tier:       _i7xh,
		CustomerID: customerID,
	}); err != nil {
		fmt.Printf("  ⚠ Warning: could not save license: %v\n", err)
	}

	if err := _5tmd(rc, rc._1s8g); err != nil {
		return err
	}

	rc.mu.Lock()
	rc._z1uh = sha256.Sum256([]byte(rc._d1e9 + rc._gj))
	rc.mu.Unlock()
	rc._oq5.Store(true)
	ActivateIntegrity(rc)

	fmt.Printf("  ✓ License activated! Key: %s...%s (_i7xh: %s)\n",
		_d1e9[:8], _d1e9[len(_d1e9)-4:], _i7xh)

	go func() {
		if err := _yry(rc, 0); err != nil {
			fmt.Printf("  ⚠ First heartbeat failed: %v\n", err)
		}
	}()

	return nil
}

func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil {
		return false, ""
	}
	if !rc._oq5.Load() {
		return false, rc.RegistrationURL()
	}
	expected := sha256.Sum256([]byte(rc._d1e9 + rc._gj))
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
			strings.HasPrefix(path, "/swagger") || path == "/ws" {
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
				status = "_oq5"
			}

			resp := gin.H{
				"status":      status,
				"instance_id": rc._gj,
			}

			rc.mu.RLock()
			if rc._d1e9 != "" {
				resp["api_key"] = rc._d1e9[:8] + "..." + rc._d1e9[len(rc._d1e9)-4:]
			}
			rc.mu.RUnlock()

			c.JSON(http.StatusOK, resp)
		})

		lic.GET("/register", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "_oq5",
					"message": "License is already _oq5",
				})
				return
			}

			rc.mu.RLock()
			existingURL := rc._34r
			rc.mu.RUnlock()

			if existingURL != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":       "pending",
					"register_url": existingURL,
				})
				return
			}

			payload := map[string]string{
				"_i7xh":        rc._i7xh,
				"_1s8g":     rc._1s8g,
				"instance_id": rc._gj,
			}
			if redirectURI := c.Query("redirect_uri"); redirectURI != "" {
				payload["redirect_uri"] = redirectURI
			}

			resp, err := _0r("/v1/register/init", payload)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				_f3mf := _36l2(resp)
				c.JSON(resp.StatusCode, gin.H{
					"error":   "Licensing server error",
					"details": _f3mf.Error(),
				})
				return
			}

			var _0rb struct {
				RegisterURL string `json:"register_url"`
				Token       string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&_0rb)

			rc.mu.Lock()
			rc._34r = _0rb.RegisterURL
			rc._8344 = _0rb.Token
			rc.mu.Unlock()

			fmt.Printf("  → Registration URL: %s\n", _0rb.RegisterURL)

			c.JSON(http.StatusOK, gin.H{
				"status":       "pending",
				"register_url": _0rb.RegisterURL,
			})
		})

		lic.GET("/activate", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "_oq5",
					"message": "License is already _oq5",
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

			exchangeResp, err := _0r("/v1/register/exchange", map[string]string{
				"authorization_code": code,
				"instance_id":       rc._gj,
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
				_f3mf := _36l2(exchangeResp)
				c.JSON(exchangeResp.StatusCode, gin.H{
					"error":   "Exchange failed",
					"details": _f3mf.Error(),
				})
				return
			}

			var _2q struct {
				APIKey     string `json:"api_key"`
				Tier       string `json:"_i7xh"`
				CustomerID int    `json:"customer_id"`
			}
			json.NewDecoder(exchangeResp.Body).Decode(&_2q)

			if _2q.APIKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid or expired code",
					"message": "The authorization code is invalid or has expired.",
				})
				return
			}

			if err := rc._2r(_2q.APIKey, _2q.Tier, _2q.CustomerID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Activation failed",
					"details": err.Error(),
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"status":  "_oq5",
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
				if err := _yry(rc, uptime); err != nil {
					fmt.Printf("  ⚠ Heartbeat failed (non-blocking): %v\n", err)
				}
			}
		}
	}()
}

func Shutdown(rc *RuntimeContext) {
	if rc == nil || rc._d1e9 == "" {
		return
	}
	_5b3y(rc)
}

func _8g(code string) (_d1e9 string, err error) {
	resp, err := _0r("/v1/register/exchange", map[string]string{
		"authorization_code": code,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", _36l2(resp)
	}

	var _2q struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp.Body).Decode(&_2q)
	if _2q.APIKey == "" {
		return "", fmt.Errorf("exchange returned empty api_key")
	}
	return _2q.APIKey, nil
}

func _rbx(authCodeOrKey string) (string, error) {
	_d1e9, err := _8g(authCodeOrKey)
	if err == nil && _d1e9 != "" {
		return _d1e9, nil
	}
	return authCodeOrKey, nil
}

func _5tmd(rc *RuntimeContext, _1s8g string) error {
	resp, err := _c14s("/v1/activate", map[string]string{
		"instance_id": rc._gj,
		"_1s8g":     _1s8g,
	}, rc._d1e9)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _36l2(resp)
	}

	var _2q struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&_2q)

	if _2q.Status != "_oq5" {
		return fmt.Errorf("activation returned status: %s", _2q.Status)
	}
	return nil
}

func _yry(rc *RuntimeContext, uptimeSeconds int64) error {
	resp, err := _c14s("/v1/heartbeat", map[string]any{
		"instance_id":    rc._gj,
		"uptime_seconds": uptimeSeconds,
		"_1s8g":        rc._1s8g,
	}, rc._d1e9)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return _36l2(resp)
	}
	return nil
}

func _5b3y(rc *RuntimeContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{
		"instance_id": rc._gj,
	})

	url := _40hs() + "/v1/deactivate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", rc._d1e9)
	req.Header.Set("X-Signature", _pq(body, rc._d1e9))
	_ml9.Do(req)
}
