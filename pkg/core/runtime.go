package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	hbInterval = 30 * time.Minute
	maxHBFails = 2
)

// RuntimeContext holds the licensing state. Required by middleware and routes.
// Removing this breaks the entire request chain.
type RuntimeContext struct {
	apiKey       string
	globalApiKey string // GLOBAL_API_KEY from .env — used as token for licensing check
	instanceID   string
	active       atomic.Bool
	ctxHash      [32]byte // Derived from activation — required by ValidateContext
	mu           sync.RWMutex
	regURL       string // Registration URL shown to users before activation
	regToken     string // Registration token for polling
	tier         string
	version      string
}

// ContextHash returns the activation hash. Used by middleware to validate requests.
// Without this, the middleware rejects ALL requests.
func (rc *RuntimeContext) ContextHash() [32]byte {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.ctxHash
}

// IsActive returns whether the license is currently active.
func (rc *RuntimeContext) IsActive() bool {
	return rc.active.Load()
}

// RegistrationURL returns the URL for the user to complete registration.
func (rc *RuntimeContext) RegistrationURL() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.regURL
}

// APIKey returns the current API key (empty if not yet registered).
func (rc *RuntimeContext) APIKey() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.apiKey
}

// InstanceID returns the hardware-based instance ID.
func (rc *RuntimeContext) InstanceID() string {
	return rc.instanceID
}

// InitializeRuntime handles the licensing lifecycle WITHOUT blocking startup:
// 1. Load or create instance ID (hardware-based)
// 2. If license exists on disk → activate immediately
// 3. If no license → set inactive state, server starts but middleware blocks API requests
// Returns RuntimeContext required by the rest of the application.
func InitializeRuntime(tier, version, globalApiKey string) *RuntimeContext {
	if tier == "" {
		tier = "evolution-go"
	}
	if version == "" {
		version = "unknown"
	}

	rc := &RuntimeContext{
		tier:         tier,
		version:      version,
		globalApiKey: globalApiKey,
	}

	// Step 1: Instance ID (hardware-based, persistent)
	id, err := loadOrCreateInstanceID()
	if err != nil {
		log.Fatalf("[runtime] failed to initialize instance: %v", err)
	}
	rc.instanceID = id

	// Step 2: Try loading existing license from disk
	rd, err := loadRuntimeData()
	if err == nil && rd.APIKey != "" {
		rc.apiKey = rd.APIKey
		fmt.Printf("  ✓ License found: %s...%s\n", rd.APIKey[:8], rd.APIKey[len(rd.APIKey)-4:])

		// Step 3: Try to activate
		if err := activateInstance(rc, version); err != nil {
			fmt.Printf("  ⚠ Activation failed: %v\n", err)
			fmt.Println("  Server will start — activate via /license/register")
			rc.active.Store(false)
		} else {
			rc.ctxHash = sha256.Sum256([]byte(rc.apiKey + rc.instanceID))
			rc.active.Store(true)
			ActivateIntegrity(rc)
			fmt.Println("  ✓ License activated successfully")
		}
	} else {
		// No license — server starts but API is blocked
		fmt.Println()
		fmt.Println("  ╔══════════════════════════════════════════════════════════╗")
		fmt.Println("  ║              License Registration Required               ║")
		fmt.Println("  ╚══════════════════════════════════════════════════════════╝")
		fmt.Println()
		fmt.Println("  Server starting without license.")
		fmt.Println("  API endpoints will return 503 until license is activated.")
		fmt.Println("  Use GET /license/register to get the registration URL.")
		fmt.Println()
		rc.active.Store(false)
	}

	return rc
}

// completeActivation finalizes the activation after registration callback.
// If the provided key is an authorization_code, exchanges it for a real API key first.
func (rc *RuntimeContext) completeActivation(authCodeOrKey, tier string, customerID int) error {
	// Exchange authorization_code for real API key
	apiKey, err := resolveAPIKey(authCodeOrKey)
	if err != nil {
		return fmt.Errorf("key exchange failed: %w", err)
	}

	rc.mu.Lock()
	rc.apiKey = apiKey
	rc.regURL = ""
	rc.regToken = ""
	rc.mu.Unlock()

	// Save to disk
	if err := saveRuntimeData(&RuntimeData{
		APIKey:     apiKey,
		Tier:       tier,
		CustomerID: customerID,
	}); err != nil {
		fmt.Printf("  ⚠ Warning: could not save license: %v\n", err)
	}

	// Activate with licensing server
	if err := activateInstance(rc, rc.version); err != nil {
		return err
	}

	// Compute context hash — required by middleware
	rc.mu.Lock()
	rc.ctxHash = sha256.Sum256([]byte(rc.apiKey + rc.instanceID))
	rc.mu.Unlock()
	rc.active.Store(true)
	ActivateIntegrity(rc)

	fmt.Printf("  ✓ License activated! Key: %s...%s (tier: %s)\n",
		apiKey[:8], apiKey[len(apiKey)-4:], tier)

	// Send first heartbeat immediately after activation
	go func() {
		if err := sendHeartbeat(rc, 0); err != nil {
			fmt.Printf("  ⚠ First heartbeat failed: %v\n", err)
		}
	}()

	return nil
}

// ValidateContext checks that the request context has a valid runtime.
// Called by the middleware on every request. Returns the registration URL
// if not yet activated, or empty string if active.
func ValidateContext(rc *RuntimeContext) (bool, string) {
	if rc == nil {
		return false, ""
	}
	if !rc.active.Load() {
		return false, rc.RegistrationURL()
	}
	// Verify context hash integrity
	expected := sha256.Sum256([]byte(rc.apiKey + rc.instanceID))
	actual := rc.ContextHash()
	if expected != actual {
		return false, ""
	}
	return true, ""
}

// GateMiddleware returns a Gin middleware that blocks all API requests when
// the license is not active. License routes (/license/*) always pass through.
// Before activation, returns the registration URL in the error response.
func GateMiddleware(rc *RuntimeContext) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Always pass through: health, license routes, frontend (manager/assets/static), swagger, favicon, ws
		if path == "/health" || path == "/server/ok" || path == "/favicon.ico" ||
			path == "/license/status" || path == "/license/register" || path == "/license/activate" ||
			strings.HasPrefix(path, "/manager") || strings.HasPrefix(path, "/assets") ||
			strings.HasPrefix(path, "/swagger") || path == "/ws" {
			c.Next()
			return
		}

		valid, _ := ValidateContext(rc)
		if !valid {
			// Build manager login URL from request host
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

		// Inject context hash into request context — routes DEPEND on this
		c.Set("_rch", rc.ContextHash())
		c.Next()
	}
}

// LicenseRoutes registers the /license/* endpoints on the Gin engine.
// These routes are NOT behind auth middleware — they need to work before activation.
func LicenseRoutes(eng *gin.Engine, rc *RuntimeContext) {
	lic := eng.Group("/license")
	{
		// GET /license/status — check if license exists in DB
		lic.GET("/status", func(c *gin.Context) {
			status := "inactive"
			if rc.IsActive() {
				status = "active"
			}

			resp := gin.H{
				"status":      status,
				"instance_id": rc.instanceID,
			}

			rc.mu.RLock()
			if rc.apiKey != "" {
				resp["api_key"] = rc.apiKey[:8] + "..." + rc.apiKey[len(rc.apiKey)-4:]
			}
			rc.mu.RUnlock()

			c.JSON(http.StatusOK, resp)
		})

		// GET /license/register — check GLOBAL_API_KEY with licensing server
		// If token not found, initiate registration and return register_url
		// Accepts optional ?redirect_uri= for post-registration redirect
		lic.GET("/register", func(c *gin.Context) {
			if rc.IsActive() {
				c.JSON(http.StatusOK, gin.H{
					"status":  "active",
					"message": "License is already active",
				})
				return
			}

			// Already have a pending registration?
			rc.mu.RLock()
			existingURL := rc.regURL
			rc.mu.RUnlock()

			if existingURL != "" {
				c.JSON(http.StatusOK, gin.H{
					"status":       "pending",
					"register_url": existingURL,
				})
				return
			}

			// Start new registration
			payload := map[string]string{
				"tier":        rc.tier,
				"version":     rc.version,
				"instance_id": rc.instanceID,
			}
			if redirectURI := c.Query("redirect_uri"); redirectURI != "" {
				payload["redirect_uri"] = redirectURI
			}

			resp, err := postUnsigned("/v1/register/init", payload)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{
					"error":   "Failed to contact licensing server",
					"details": err.Error(),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errBody := readErrorBody(resp)
				c.JSON(resp.StatusCode, gin.H{
					"error":   "Licensing server error",
					"details": errBody.Error(),
				})
				return
			}

			var initResult struct {
				RegisterURL string `json:"register_url"`
				Token       string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&initResult)

			rc.mu.Lock()
			rc.regURL = initResult.RegisterURL
			rc.regToken = initResult.Token
			rc.mu.Unlock()

			fmt.Printf("  → Registration URL: %s\n", initResult.RegisterURL)

			c.JSON(http.StatusOK, gin.H{
				"status":       "pending",
				"register_url": initResult.RegisterURL,
			})
		})

		// GET /license/activate?code=XXX — exchange authorization code for api_key and save to DB
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

			// Exchange code for api_key via licensing server
			exchangeResp, err := postUnsigned("/v1/register/exchange", map[string]string{
				"authorization_code": code,
				"instance_id":       rc.instanceID,
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
				errBody := readErrorBody(exchangeResp)
				c.JSON(exchangeResp.StatusCode, gin.H{
					"error":   "Exchange failed",
					"details": errBody.Error(),
				})
				return
			}

			var result struct {
				APIKey     string `json:"api_key"`
				Tier       string `json:"tier"`
				CustomerID int    `json:"customer_id"`
			}
			json.NewDecoder(exchangeResp.Body).Decode(&result)

			if result.APIKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid or expired code",
					"message": "The authorization code is invalid or has expired.",
				})
				return
			}

			// Save to DB and activate
			if err := rc.completeActivation(result.APIKey, result.Tier, result.CustomerID); err != nil {
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

// StartHeartbeat runs periodic heartbeat in background.
// Deactivates the service after maxHBFails consecutive failures.
func StartHeartbeat(ctx context.Context, rc *RuntimeContext, startTime time.Time) {
	go func() {
		ticker := time.NewTicker(hbInterval)
		defer ticker.Stop()

		var failures atomic.Int32

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !rc.IsActive() {
					continue // Don't send heartbeat if not activated
				}
				uptime := int64(time.Since(startTime).Seconds())
				err := sendHeartbeat(rc, uptime)
				if err != nil {
					if failures.Add(1) >= int32(maxHBFails) {
						rc.active.Store(false)
					}
				} else {
					failures.Store(0)
					rc.active.Store(true)
				}
			}
		}
	}()
}

// Shutdown deactivates the instance with the licensing server.
func Shutdown(rc *RuntimeContext) {
	if rc == nil || rc.apiKey == "" {
		return
	}
	sendDeactivate(rc)
}

// ── Internal functions ──────────────────────────────────────────────

// exchangeCode trades an authorization_code for a real API key.
func exchangeCode(code string) (apiKey string, err error) {
	resp, err := postUnsigned("/v1/register/exchange", map[string]string{
		"authorization_code": code,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", readErrorBody(resp)
	}

	var result struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.APIKey == "" {
		return "", fmt.Errorf("exchange returned empty api_key")
	}
	return result.APIKey, nil
}

// resolveAPIKey resolves authorization_code to real api_key via exchange,
// or returns the key directly if already an api_key.
func resolveAPIKey(authCodeOrKey string) (string, error) {
	// Try exchange first — if it fails with 404/400, it might already be an api_key
	apiKey, err := exchangeCode(authCodeOrKey)
	if err == nil && apiKey != "" {
		return apiKey, nil
	}
	// Fallback: treat as api_key directly
	return authCodeOrKey, nil
}

func activateInstance(rc *RuntimeContext, version string) error {
	resp, err := postSigned("/v1/activate", map[string]string{
		"instance_id": rc.instanceID,
		"version":     version,
	}, rc.apiKey)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readErrorBody(resp)
	}

	var result struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "active" {
		return fmt.Errorf("activation returned status: %s", result.Status)
	}
	return nil
}

func sendHeartbeat(rc *RuntimeContext, uptimeSeconds int64) error {
	resp, err := postSigned("/v1/heartbeat", map[string]any{
		"instance_id":    rc.instanceID,
		"uptime_seconds": uptimeSeconds,
	}, rc.apiKey)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readErrorBody(resp)
	}
	return nil
}

func sendDeactivate(rc *RuntimeContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, _ := json.Marshal(map[string]string{
		"instance_id": rc.instanceID,
	})

	url := resolveEndpoint() + "/v1/deactivate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", rc.apiKey)
	req.Header.Set("X-Signature", signPayload(body, rc.apiKey))
	httpTransport.Do(req)
}
