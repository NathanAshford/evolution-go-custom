package core

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpTransport = &http.Client{Timeout: 10 * time.Second}

// signPayload computes HMAC-SHA256 of body using the API key as secret.
func signPayload(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// postSigned sends a signed POST request to the licensing endpoint.
func postSigned(path string, payload interface{}, apiKey string) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := resolveEndpoint() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("X-Signature", signPayload(body, apiKey))

	return httpTransport.Do(req)
}

// getUnsigned sends an unsigned GET request (for public endpoints like register).
func getUnsigned(path string) (*http.Response, error) {
	url := resolveEndpoint() + path
	return httpTransport.Get(url)
}

// postUnsigned sends an unsigned POST request (for public endpoints like register/init).
func postUnsigned(path string, payload interface{}) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := resolveEndpoint() + path
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return httpTransport.Do(req)
}

// readErrorBody extracts an error message from a JSON error response.
func readErrorBody(resp *http.Response) error {
	b, _ := io.ReadAll(resp.Body)
	var errBody struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(b, &errBody); err == nil {
		msg := errBody.Message
		if msg == "" {
			msg = errBody.Error
		}
		if msg != "" {
			return fmt.Errorf("%s (HTTP %d)", strings.ToLower(msg), resp.StatusCode)
		}
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
