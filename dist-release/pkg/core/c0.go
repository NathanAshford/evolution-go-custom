package core

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (_p0 = 5 * time.Second; _p1 = 30 * time.Minute; _p2 = 30 * time.Minute; _p3 = 2)

var (
	_k0  = []byte{0x0a, 0x98, 0x2f, 0x9e, 0xf9, 0xb4, 0xe2, 0x7c, 0x9f, 0x08, 0xc5, 0x46, 0xbc, 0x4f, 0xd3, 0x0f, 0x82, 0x96, 0xe4, 0x55, 0x16, 0x6a, 0xc4, 0xf3, 0x6e, 0x69, 0x2e, 0x20, 0x2b, 0x38, 0x12, 0xe0, 0xa9, 0xfa, 0xd1, 0xbc, 0xf0, 0x3d, 0x34, 0xcc, 0x1c, 0x0e}
	_k1  = []byte{0x62, 0xec, 0x5b, 0xee, 0x8a, 0x8e, 0xcd, 0x53, 0xf3, 0x61, 0xa6, 0x23, 0xd2, 0x3c, 0xb6, 0x21, 0xe7, 0xe0, 0x8b, 0x39, 0x63, 0x1e, 0xad, 0x9c, 0x00, 0x0f, 0x41, 0x55, 0x45, 0x5c, 0x73, 0x94, 0xc0, 0x95, 0xbf, 0x92, 0x93, 0x52, 0x59, 0xe2, 0x7e, 0x7c}
	_h0  = &http.Client{Timeout: 10 * time.Second}
	_s0  atomic.Value
	_rd0 string
)

func init() { _s0.Store([]byte{0}); _rd0 = _d0() }

type RuntimeContext struct {
	_a0 string; _a1 string; _a2 atomic.Bool; _a3 [32]byte
	_a4 sync.RWMutex; _a5 string; _a6 string; _a7 string; _a8 string
}

func (r *RuntimeContext) ContextHash() [32]byte   { r._a4.RLock(); defer r._a4.RUnlock(); return r._a3 }
func (r *RuntimeContext) IsActive() bool           { return r._a2.Load() }
func (r *RuntimeContext) APIKey() string            { r._a4.RLock(); defer r._a4.RUnlock(); return r._a0 }
func (r *RuntimeContext) InstanceID() string        { return r._a1 }
func (r *RuntimeContext) RegistrationURL() string   { r._a4.RLock(); defer r._a4.RUnlock(); return r._a5 }

func _d0() string {
	d := make([]byte, len(_k0)); for i, b := range _k0 { d[i] = b ^ _k1[i%len(_k1)] }; return string(d)
}

func _sg(b []byte, s string) string {
	m := hmac.New(sha256.New, []byte(s)); m.Write(b); r := m.Sum(nil)
	const t = "0123456789abcdef"; d := make([]byte, len(r)*2)
	for i, v := range r { d[i*2] = t[v>>4]; d[i*2+1] = t[v&0x0f] }; return string(d)
}

func _ps(p string, pl interface{}, k string) (*http.Response, error) {
	b, e := json.Marshal(pl); if e != nil { return nil, e }
	r, e := http.NewRequest(http.MethodPost, _rd0+p, bytes.NewReader(b)); if e != nil { return nil, e }
	r.Header.Set("Content-Type", "application/json"); r.Header.Set("X-API-Key", k); r.Header.Set("X-Signature", _sg(b, k))
	return _h0.Do(r)
}

func _gu(p string) (*http.Response, error) { return _h0.Get(_rd0 + p) }

func _pu(p string, pl interface{}) (*http.Response, error) {
	b, e := json.Marshal(pl); if e != nil { return nil, e }
	r, e := http.NewRequest(http.MethodPost, _rd0+p, bytes.NewReader(b)); if e != nil { return nil, e }
	r.Header.Set("Content-Type", "application/json"); return _h0.Do(r)
}

func _re(rs *http.Response) error {
	b, _ := io.ReadAll(rs.Body); var e struct{ M string `json:"message"`; E string `json:"error"` }
	if json.Unmarshal(b, &e) == nil { m := e.M; if m == "" { m = e.E }; if m != "" { return fmt.Errorf("%s (HTTP %d)", strings.ToLower(m), rs.StatusCode) } }
	return fmt.Errorf("HTTP %d", rs.StatusCode)
}

func _ex(c string) (string, error) {
	rs, e := _pu("/v1/register/exchange", map[string]string{"authorization_code": c}); if e != nil { return "", e }
	defer rs.Body.Close(); if rs.StatusCode != 200 { return "", _re(rs) }
	var r struct{ K string `json:"api_key"` }; json.NewDecoder(rs.Body).Decode(&r)
	if r.K == "" { return "", fmt.Errorf("exchange empty") }; return r.K, nil
}

func _rk(c string) (string, error) { k, e := _ex(c); if e == nil && k != "" { return k, nil }; return c, nil }

func _ai(rc *RuntimeContext, v string) error {
	rs, e := _ps("/v1/activate", map[string]string{"instance_id": rc._a1, "version": v}, rc._a0); if e != nil { return e }
	defer rs.Body.Close(); if rs.StatusCode != 200 { return _re(rs) }
	var r struct{ S string `json:"status"` }; json.NewDecoder(rs.Body).Decode(&r)
	if r.S != "active" { return fmt.Errorf("status: %s", r.S) }; return nil
}

type _rtd struct{ K string `json:"k"`; T string `json:"t"`; C int `json:"c"` }

func _dp() string { h, _ := os.UserHomeDir(); return filepath.Join(h, ".evolution-go") }

func _lrd() (*_rtd, error) {
	b, e := os.ReadFile(filepath.Join(_dp(), ".runtime.dat")); if e != nil { return nil, e }
	var d _rtd; if e := json.Unmarshal(b, &d); e != nil { return nil, e }; return &d, nil
}

func _srd(d *_rtd) error { os.MkdirAll(_dp(), 0700); b, _ := json.Marshal(d); return os.WriteFile(filepath.Join(_dp(), ".runtime.dat"), b, 0600) }

func _lid() (string, error) {
	p := filepath.Join(_dp(), ".instance.id")
	if b, e := os.ReadFile(p); e == nil && len(b) > 0 { return strings.TrimSpace(string(b)), nil }
	id := _ghi(); os.MkdirAll(_dp(), 0700)
	if e := os.WriteFile(p, []byte(id), 0600); e != nil { return "", e }; return id, nil
}

func _ghi() string {
	ii, e := net.Interfaces(); if e != nil { return uuid.New().String() }
	var mc string; for _, i := range ii { if len(i.HardwareAddr) > 0 && i.Flags&net.FlagLoopback == 0 { mc = i.HardwareAddr.String(); break } }
	hn, _ := os.Hostname(); if mc == "" && hn == "" { return uuid.New().String() }
	h := sha256.Sum256([]byte(mc + "|" + hn)); return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", h[:4], h[4:6], h[6:8], h[8:10], h[10:16])
}

func InitializeRuntime(tier, version string) *RuntimeContext {
	if tier == "" { tier = "community" }; if version == "" { version = "unknown" }
	rc := &RuntimeContext{_a7: tier, _a8: version}
	id, e := _lid(); if e != nil { log.Fatalf("[runtime] %v", e) }; rc._a1 = id
	rd, e := _lrd(); if e == nil && rd.K != "" {
		rc._a0 = rd.K; fmt.Printf("  \u2713 License found: %s...%s\n", rd.K[:8], rd.K[len(rd.K)-4:])
		if e := _ai(rc, version); e != nil {
			fmt.Printf("  \u26a0 Activation failed: %v\n", e); rc._a2.Store(false)
		} else { rc._a3 = sha256.Sum256([]byte(rc._a0 + rc._a1)); rc._a2.Store(true); _ia(rc); fmt.Println("  \u2713 License activated successfully") }
	} else {
		fmt.Println("\n  \u2554\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2557\n  \u2551              License Registration Required               \u2551\n  \u255a\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u2550\u255d")
		fmt.Println("\n  Server starting without license.\n  API endpoints will return 503 until license is activated.\n  Use GET /license/register to get the registration URL.\n")
		rc._a2.Store(false)
	}; return rc
}

func (rc *RuntimeContext) _ca(c, t string, ci int) error {
	k, e := _rk(c); if e != nil { return fmt.Errorf("exchange: %w", e) }
	rc._a4.Lock(); rc._a0 = k; rc._a5 = ""; rc._a6 = ""; rc._a4.Unlock()
	_srd(&_rtd{K: k, T: t, C: ci}); if e := _ai(rc, rc._a8); e != nil { return e }
	rc._a4.Lock(); rc._a3 = sha256.Sum256([]byte(rc._a0 + rc._a1)); rc._a4.Unlock()
	rc._a2.Store(true); _ia(rc)
	fmt.Printf("  \u2713 License activated! Key: %s...%s (tier: %s)\n", k[:8], k[len(k)-4:], t); return nil
}

func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil { return false, "" }; if !rc._a2.Load() { return false, rc.RegistrationURL() }
	e := sha256.Sum256([]byte(rc._a0 + rc._a1)); a := rc.ContextHash(); if e != a { return false, "" }; return true, ""
}

func GateMiddleware(rc *RuntimeContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/health" || p == "/server/ok" || p == "/favicon.ico" || p == "/license/status" || p == "/license/register" || p == "/license/activate" || strings.HasPrefix(p, "/manager") || strings.HasPrefix(p, "/assets") || strings.HasPrefix(p, "/swagger") || p == "/ws" { c.Next(); return }
		v, u := ValidateContext(rc); if !v {
			r := gin.H{"error": "service not activated", "code": "LICENSE_REQUIRED"}
			if u != "" { r["register_url"] = u; r["message"] = "Please open the registration URL in your browser to activate this instance" } else { r["register_url"] = "/license/register"; r["message"] = "License required. Call GET /license/register to start activation." }
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, r); return
		}; c.Set("_rch", rc.ContextHash()); c.Next()
	}
}

func LicenseRoutes(eng *gin.Engine, rc *RuntimeContext) {
	g := eng.Group("/license")
	g.GET("/status", func(c *gin.Context) {
		rc._a4.RLock(); defer rc._a4.RUnlock(); s := "inactive"; if rc._a2.Load() { s = "active" }
		r := gin.H{"status": s, "instance_id": rc._a1}; if rc._a0 != "" { r["api_key"] = rc._a0[:8] + "..." + rc._a0[len(rc._a0)-4:] }
		if !rc._a2.Load() && rc._a5 != "" { r["register_url"] = rc._a5 }; c.JSON(200, r)
	})
	g.GET("/register", func(c *gin.Context) {
		if rc.IsActive() { c.JSON(200, gin.H{"status": "active", "message": "License is already active"}); return }
		rc._a4.RLock(); eu, et := rc._a5, rc._a6; rc._a4.RUnlock()
		if eu != "" && et != "" { c.JSON(200, gin.H{"status": "pending", "register_url": eu, "message": "Registration already in progress."}); return }
		rs, e := _pu("/v1/register/init", map[string]string{"tier": rc._a7, "version": rc._a8, "instance_id": rc._a1})
		if e != nil { c.JSON(502, gin.H{"error": "Failed to contact licensing server", "details": e.Error()}); return }
		defer rs.Body.Close(); if rs.StatusCode != 200 { c.JSON(rs.StatusCode, gin.H{"error": "Licensing error", "details": _re(rs).Error()}); return }
		var ir struct{ U string `json:"register_url"`; T string `json:"token"` }; json.NewDecoder(rs.Body).Decode(&ir)
		rc._a4.Lock(); rc._a5 = ir.U; rc._a6 = ir.T; rc._a4.Unlock()
		fmt.Printf("  \u2192 Registration URL: %s\n", ir.U); go _pa(rc)
		c.JSON(200, gin.H{"status": "pending", "register_url": ir.U, "message": "Open the URL in your browser to register and activate."})
	})
	g.GET("/activate", func(c *gin.Context) {
		if rc.IsActive() { c.JSON(200, gin.H{"status": "active", "message": "License is already active"}); return }
		rc._a4.RLock(); tk := rc._a6; rc._a4.RUnlock()
		if tk == "" { c.JSON(400, gin.H{"error": "No pending registration", "message": "Call GET /license/register first."}); return }
		sr, e := _gu("/v1/register/status?token=" + tk); if e != nil { c.JSON(502, gin.H{"error": "Failed to contact licensing server"}); return }
		defer sr.Body.Close()
		var r struct{ S string `json:"status"`; K string `json:"api_key"`; A string `json:"authorization_code"`; T string `json:"tier"`; C int `json:"customer_id"` }
		json.NewDecoder(sr.Body).Decode(&r); ak := r.K; if ak == "" { ak = r.A }
		if r.S == "completed" && ak != "" { if e := rc._ca(ak, r.T, r.C); e != nil { c.JSON(500, gin.H{"error": "Activation failed", "details": e.Error()}); return }; c.JSON(200, gin.H{"status": "active", "message": "License activated!"}); return }
		c.JSON(200, gin.H{"status": r.S, "message": "Registration not yet completed."})
	})
}

func _pa(rc *RuntimeContext) {
	dl := time.Now().Add(_p1); for time.Now().Before(dl) {
		if rc.IsActive() { return }; rc._a4.RLock(); tk := rc._a6; rc._a4.RUnlock(); if tk == "" { return }
		rs, e := _gu("/v1/register/status?token=" + tk); if e != nil { time.Sleep(_p0); continue }
		var r struct{ S string `json:"status"`; K string `json:"api_key"`; A string `json:"authorization_code"`; T string `json:"tier"`; C int `json:"customer_id"` }
		json.NewDecoder(rs.Body).Decode(&r); rs.Body.Close(); ak := r.K; if ak == "" { ak = r.A }
		if r.S == "completed" && ak != "" { if e := rc._ca(ak, r.T, r.C); e != nil { fmt.Printf("  \u26a0 Activation failed: %v\n", e) }; return }
		if r.S == "expired" { rc._a4.Lock(); rc._a5 = ""; rc._a6 = ""; rc._a4.Unlock(); return }; time.Sleep(_p0)
	}; rc._a4.Lock(); rc._a5 = ""; rc._a6 = ""; rc._a4.Unlock()
}

func StartHeartbeat(ctx context.Context, rc *RuntimeContext, st time.Time) {
	go func() { tk := time.NewTicker(_p2); defer tk.Stop(); var f atomic.Int32; for { select {
	case <-ctx.Done(): return
	case <-tk.C: if !rc.IsActive() { continue }; u := int64(time.Since(st).Seconds())
		rs, e := _ps("/v1/heartbeat", map[string]any{"instance_id": rc._a1, "uptime_seconds": u}, rc._a0)
		if e != nil { if f.Add(1) >= int32(_p3) { rc._a2.Store(false) }; continue }
		rs.Body.Close(); if rs.StatusCode != 200 { if f.Add(1) >= int32(_p3) { rc._a2.Store(false) } } else { f.Store(0); rc._a2.Store(true) }
	} } }()
}

func Shutdown(rc *RuntimeContext) {
	if rc == nil || rc._a0 == "" { return }
	cx, cl := context.WithTimeout(context.Background(), 5*time.Second); defer cl()
	b, _ := json.Marshal(map[string]string{"instance_id": rc._a1})
	r, e := http.NewRequestWithContext(cx, http.MethodPost, _rd0+"/v1/deactivate", bytes.NewReader(b)); if e != nil { return }
	r.Header.Set("Content-Type", "application/json"); r.Header.Set("X-API-Key", rc._a0); r.Header.Set("X-Signature", _sg(b, rc._a0)); _h0.Do(r)
}

func ComputeSessionSeed(n string, rc *RuntimeContext) []byte {
	if rc == nil || !rc._a2.Load() { return nil }; h := sha256.New(); h.Write([]byte(n)); h.Write([]byte(rc._a0))
	s, _ := _s0.Load().([]byte); h.Write(s); return h.Sum(nil)[:16]
}

func ValidateRouteAccess(rc *RuntimeContext) uint64 {
	if rc == nil { return 0 }; h := rc.ContextHash(); return binary.LittleEndian.Uint64(h[:8])
}

func DeriveInstanceToken(id string, rc *RuntimeContext) string {
	if rc == nil || !rc._a2.Load() { return "" }; h := sha256.Sum256([]byte(id + rc._a0))
	const t = "0123456789abcdef"; d := make([]byte, 16); for i := 0; i < 8; i++ { d[i*2] = t[h[i]>>4]; d[i*2+1] = t[h[i]&0x0f] }; return string(d)
}

func _ia(rc *RuntimeContext) { if rc == nil { return }; h := sha256.Sum256([]byte(rc._a0 + rc._a1 + "ev0")); _s0.Store(h[:]) }
