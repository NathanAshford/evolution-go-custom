package transport

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/pion/transport/v4"
	"github.com/pion/transport/v4/stdnet"
	"github.com/pion/webrtc/v4"
)

// ProxyConfig describes the instance proxy that call media should traverse.
type ProxyConfig struct {
	Protocol string
	Host     string
	Port     string
	Username string
	Password string
}

func (p *ProxyConfig) address() string { return net.JoinHostPort(p.Host, p.Port) }

// IsSocks reports whether the proxy can carry UDP.
//
// Only SOCKS5 has UDP ASSOCIATE. HTTP proxies expose CONNECT, which is a TCP
// tunnel and cannot carry the SRTP media path.
func (p *ProxyConfig) IsSocks() bool {
	return p != nil && strings.HasPrefix(strings.ToLower(p.Protocol), "socks")
}

// MediaSupport explains, in one line, whether a call's audio can flow through
// this proxy. Signaling is unaffected either way: the <call> stanzas travel on
// the whatsmeow websocket, which already honours the instance proxy — so a call
// still rings behind an HTTP proxy, it just cannot carry audio.
func (p *ProxyConfig) MediaSupport() string {
	switch {
	case p == nil || p.Host == "":
		return "direct (no proxy): media uses UDP directly"
	case p.IsSocks():
		return "socks5: media tunnelled over UDP ASSOCIATE"
	default:
		return "http proxy: signaling works (call rings), but media needs UDP and " +
			"HTTP CONNECT is TCP-only — switch the instance to a SOCKS5 proxy for audio"
	}
}

// socksNet is a pion network stack whose UDP sockets are relayed by a SOCKS5
// proxy. Everything other than packet listening falls through to the standard
// stack.
type socksNet struct {
	transport.Net
	proxy *ProxyConfig
	log   *slog.Logger
}

// ListenPacket is what pion/ice calls to obtain the sockets it gathers
// candidates on; routing it through the proxy is what puts media on the proxy's
// egress IP instead of the server's.
func (n *socksNet) ListenPacket(network string, address string) (net.PacketConn, error) {
	if !strings.HasPrefix(network, "udp") {
		return n.Net.ListenPacket(network, address)
	}

	conn, err := DialSocks5UDP(n.proxy.address(), n.proxy.Username, n.proxy.Password, 10*time.Second)
	if err != nil {
		n.log.Warn("socks5 udp association failed, falling back to direct UDP",
			"proxy", n.proxy.address(), "error", err)
		return n.Net.ListenPacket(network, address)
	}

	n.log.Info("call media routed through socks5 proxy", "proxy", n.proxy.address())
	return conn, nil
}

func (n *socksNet) ListenUDP(network string, locAddr *net.UDPAddr) (transport.UDPConn, error) {
	// pion asks for a concrete UDPConn here, which a SOCKS5 relay cannot
	// impersonate (it has no real local UDP identity). ListenPacket above is the
	// path ICE actually uses for host candidates.
	return n.Net.ListenUDP(network, locAddr)
}

// NewProxyAPI builds the pion API used to create peer connections, wiring the
// SOCKS5 network stack when the instance proxy supports UDP.
//
// A nil/empty proxy, or any non-SOCKS proxy, yields the default direct stack.
func NewProxyAPI(proxy *ProxyConfig, log *slog.Logger) (*webrtc.API, error) {
	if log == nil {
		log = slog.Default()
	}

	settings := webrtc.SettingEngine{}

	if proxy != nil && proxy.Host != "" && proxy.IsSocks() {
		base, err := stdnet.NewNet()
		if err != nil {
			return nil, fmt.Errorf("create base net: %w", err)
		}

		settings.SetNet(&socksNet{Net: base, proxy: proxy, log: log})
		log.Info("voip: using socks5 proxy for call media", "proxy", proxy.address())
	} else if proxy != nil && proxy.Host != "" {
		log.Warn("voip: instance proxy cannot carry call media", "detail", proxy.MediaSupport())
	}

	return webrtc.NewAPI(webrtc.WithSettingEngine(settings)), nil
}
