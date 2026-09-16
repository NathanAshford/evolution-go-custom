package instance_service

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/EvolutionAPI/evolution-go/pkg/utils"
)

// proxyProbeTimeout bounds the whole check. A proxy that needs longer than this
// to answer is not usable for a WhatsApp websocket anyway.
const proxyProbeTimeout = 12 * time.Second

// ipEchoEndpoints are tried in order until one answers. Several are listed
// because any single one can be down or blocked from a given exit node, and a
// dead echo service must not be reported as a dead proxy.
var ipEchoEndpoints = []string{
	"https://api.ipify.org",
	"https://checkip.amazonaws.com",
	"https://ifconfig.me/ip",
}

// ProxyTestResult is what the UI shows after a test.
type ProxyTestResult struct {
	OK bool `json:"ok"`
	// IP is the address the outside world sees when this proxy is used.
	IP string `json:"ip,omitempty"`
	// ServerIP is what the outside world sees without the proxy, so the caller
	// can tell a working proxy from one that silently passes traffic through.
	ServerIP string `json:"serverIp,omitempty"`
	// Anonymous is true when IP differs from ServerIP — the point of a proxy.
	Anonymous bool `json:"anonymous"`
	// WhatsAppReachable reports whether web.whatsapp.com answered through the
	// proxy. A proxy can reach the open internet and still be blocked by
	// WhatsApp, and that distinction is the useful one here.
	WhatsAppReachable bool   `json:"whatsappReachable"`
	LatencyMs         int64  `json:"latencyMs,omitempty"`
	Protocol          string `json:"protocol,omitempty"`
	Error             string `json:"error,omitempty"`
}

// TestProxy checks whether a proxy actually works, without touching the
// instance's live connection.
//
// It answers the two questions an operator has: does traffic get through, and
// which IP does it come out of. It deliberately does not reconnect anything —
// testing a proxy must be safe to do on a running instance.
func (i instances) TestProxy(cfg *ProxyConfig) (*ProxyTestResult, error) {
	if cfg == nil || strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("proxy host is required")
	}

	protocol := utils.NormalizeProxyProtocol(cfg.Protocol, cfg.Port)

	address, err := utils.BuildProxyAddress(cfg.Protocol, cfg.Host, cfg.Port, cfg.Username, cfg.Password)
	if err != nil {
		return &ProxyTestResult{Protocol: protocol, Error: err.Error()}, nil
	}

	client, err := proxyHTTPClient(address)
	if err != nil {
		return &ProxyTestResult{Protocol: protocol, Error: err.Error()}, nil
	}

	result := &ProxyTestResult{Protocol: protocol}

	started := time.Now()
	ip, err := fetchExitIP(client)
	result.LatencyMs = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}

	result.OK = true
	result.IP = ip

	// The server's own IP is only used for the comparison below, so a failure
	// to determine it must not fail the test.
	if own, err := fetchExitIP(&http.Client{Timeout: proxyProbeTimeout}); err == nil {
		result.ServerIP = own
		result.Anonymous = own != ip
	}

	result.WhatsAppReachable = canReachWhatsApp(client)

	return result, nil
}

// proxyHTTPClient builds a client whose traffic goes through the proxy. SOCKS5
// is handled by the transport's proxy support the same way http proxies are,
// since Go understands the socks5:// scheme in a proxy URL.
func proxyHTTPClient(address string) (*http.Client, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy address: %w", err)
	}

	transport := (http.DefaultTransport.(*http.Transport)).Clone()
	transport.Proxy = http.ProxyURL(parsed)
	transport.DialContext = (&net.Dialer{Timeout: proxyProbeTimeout}).DialContext

	return &http.Client{Transport: transport, Timeout: proxyProbeTimeout}, nil
}

func fetchExitIP(client *http.Client) (string, error) {
	var lastErr error

	for _, endpoint := range ipEchoEndpoints {
		ctx, cancel := context.WithTimeout(context.Background(), proxyProbeTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 128))
		resp.Body.Close()
		cancel()

		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%s returned %s", endpoint, resp.Status)
			continue
		}

		if ip := strings.TrimSpace(string(body)); ip != "" {
			return ip, nil
		}
		lastErr = fmt.Errorf("%s returned an empty body", endpoint)
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no IP echo service answered")
	}
	return "", lastErr
}

// canReachWhatsApp reports whether the proxy can open a connection to the host
// the websocket uses. Any HTTP answer counts: the check is reachability, not
// the status code, since a bare GET on /ws/chat is expected to be refused.
func canReachWhatsApp(client *http.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), proxyProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://web.whatsapp.com/", nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}
