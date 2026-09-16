package transport

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

// SOCKS5 wire constants (RFC 1928).
const (
	socks5Version = 0x05

	socks5AuthNone     = 0x00
	socks5AuthUserPass = 0x02
	socks5AuthVersion  = 0x01

	socks5CmdUDPAssociate = 0x03

	socks5AddrIPv4   = 0x01
	socks5AddrDomain = 0x03
	socks5AddrIPv6   = 0x04

	socks5ReplySuccess = 0x00
)

// ErrUDPNotSupported reports a proxy that cannot carry the媒体 path.
//
// WhatsApp call media is SRTP over UDP. An HTTP proxy only offers CONNECT,
// which is a TCP tunnel, so UDP cannot traverse it — only a SOCKS5 proxy
// implementing UDP ASSOCIATE can.
var ErrUDPNotSupported = errors.New("proxy does not support UDP (call media requires UDP; use a SOCKS5 proxy)")

// Socks5UDPConn is a net.PacketConn whose datagrams are relayed by a SOCKS5
// proxy via UDP ASSOCIATE.
//
// The TCP control connection must stay open for the whole lifetime of the
// association: the proxy tears the UDP relay down as soon as it closes.
type Socks5UDPConn struct {
	control net.Conn // keeps the association alive
	relay   net.Conn // UDP socket towards the proxy's relay endpoint
}

// DialSocks5UDP opens a UDP association through a SOCKS5 proxy.
//
// proxyAddr is "host:port"; user/pass may be empty for an open proxy.
func DialSocks5UDP(proxyAddr, user, pass string, timeout time.Duration) (*Socks5UDPConn, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	control, err := net.DialTimeout("tcp", proxyAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("socks5: dial proxy: %w", err)
	}

	if err := control.SetDeadline(time.Now().Add(timeout)); err != nil {
		control.Close()
		return nil, err
	}

	if err := socks5Handshake(control, user, pass); err != nil {
		control.Close()
		return nil, err
	}

	// Requesting 0.0.0.0:0 tells the proxy we will send from any local address,
	// which is what ICE needs since its source port is not known up front.
	relayAddr, err := socks5UDPAssociate(control)
	if err != nil {
		control.Close()
		return nil, err
	}

	relay, err := net.DialTimeout("udp", relayAddr, timeout)
	if err != nil {
		control.Close()
		return nil, fmt.Errorf("socks5: dial udp relay: %w", err)
	}

	// Clear the handshake deadline; the caller manages I/O deadlines from here.
	if err := control.SetDeadline(time.Time{}); err != nil {
		control.Close()
		relay.Close()
		return nil, err
	}

	return &Socks5UDPConn{control: control, relay: relay}, nil
}

func socks5Handshake(conn net.Conn, user, pass string) error {
	methods := []byte{socks5AuthNone}
	if user != "" {
		methods = []byte{socks5AuthNone, socks5AuthUserPass}
	}

	req := append([]byte{socks5Version, byte(len(methods))}, methods...)
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("socks5: greeting: %w", err)
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks5: greeting reply: %w", err)
	}
	if resp[0] != socks5Version {
		return fmt.Errorf("socks5: bad version %d", resp[0])
	}

	switch resp[1] {
	case socks5AuthNone:
		return nil
	case socks5AuthUserPass:
		if user == "" {
			return errors.New("socks5: proxy requires credentials but none were configured")
		}
		return socks5UserPassAuth(conn, user, pass)
	default:
		return fmt.Errorf("socks5: no acceptable auth method (proxy offered %d)", resp[1])
	}
}

func socks5UserPassAuth(conn net.Conn, user, pass string) error {
	if len(user) > 255 || len(pass) > 255 {
		return errors.New("socks5: credentials too long")
	}

	buf := make([]byte, 0, 3+len(user)+len(pass))
	buf = append(buf, socks5AuthVersion, byte(len(user)))
	buf = append(buf, user...)
	buf = append(buf, byte(len(pass)))
	buf = append(buf, pass...)

	if _, err := conn.Write(buf); err != nil {
		return fmt.Errorf("socks5: auth: %w", err)
	}

	resp := make([]byte, 2)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("socks5: auth reply: %w", err)
	}
	if resp[1] != 0x00 {
		return errors.New("socks5: proxy rejected the credentials")
	}

	return nil
}

// socks5UDPAssociate issues UDP ASSOCIATE and returns the relay's "host:port".
func socks5UDPAssociate(conn net.Conn) (string, error) {
	// VER, CMD, RSV, ATYP=IPv4, 0.0.0.0, port 0
	req := []byte{socks5Version, socks5CmdUDPAssociate, 0x00, socks5AddrIPv4, 0, 0, 0, 0, 0, 0}
	if _, err := conn.Write(req); err != nil {
		return "", fmt.Errorf("socks5: udp associate: %w", err)
	}

	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		return "", fmt.Errorf("socks5: associate reply: %w", err)
	}
	if head[1] != socks5ReplySuccess {
		return "", fmt.Errorf("socks5: udp associate refused (code %d) — proxy likely does not support UDP", head[1])
	}

	host, err := readSocks5Addr(conn, head[3])
	if err != nil {
		return "", err
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", fmt.Errorf("socks5: associate port: %w", err)
	}

	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(portBuf)))), nil
}

func readSocks5Addr(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case socks5AddrIPv4:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case socks5AddrIPv6:
		buf := make([]byte, 16)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case socks5AddrDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		buf := make([]byte, lenBuf[0])
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	}
	return "", fmt.Errorf("socks5: unknown address type %d", atyp)
}

// EncodeUDPRequest wraps a payload in the SOCKS5 UDP request header
// (RFC 1928 §7) addressed to dst.
func EncodeUDPRequest(dst *net.UDPAddr, payload []byte) ([]byte, error) {
	if dst == nil {
		return nil, errors.New("socks5: nil destination")
	}

	// RSV(2) + FRAG(1) + ATYP(1) + ADDR + PORT(2)
	header := []byte{0x00, 0x00, 0x00}

	if ip4 := dst.IP.To4(); ip4 != nil {
		header = append(header, socks5AddrIPv4)
		header = append(header, ip4...)
	} else if ip6 := dst.IP.To16(); ip6 != nil {
		header = append(header, socks5AddrIPv6)
		header = append(header, ip6...)
	} else {
		return nil, errors.New("socks5: destination has no usable IP")
	}

	port := make([]byte, 2)
	binary.BigEndian.PutUint16(port, uint16(dst.Port))
	header = append(header, port...)

	return append(header, payload...), nil
}

// DecodeUDPReply strips the SOCKS5 UDP header, returning the payload and the
// address the datagram originated from.
func DecodeUDPReply(buf []byte) ([]byte, *net.UDPAddr, error) {
	if len(buf) < 10 {
		return nil, nil, errors.New("socks5: short udp reply")
	}
	// Fragmentation is not supported by any proxy worth using, and ICE never
	// needs it.
	if buf[2] != 0x00 {
		return nil, nil, errors.New("socks5: fragmented udp datagrams are not supported")
	}

	var (
		addr net.IP
		pos  = 4
	)

	switch buf[3] {
	case socks5AddrIPv4:
		if len(buf) < pos+4+2 {
			return nil, nil, errors.New("socks5: short ipv4 reply")
		}
		addr = net.IP(buf[pos : pos+4])
		pos += 4
	case socks5AddrIPv6:
		if len(buf) < pos+16+2 {
			return nil, nil, errors.New("socks5: short ipv6 reply")
		}
		addr = net.IP(buf[pos : pos+16])
		pos += 16
	case socks5AddrDomain:
		length := int(buf[pos])
		pos++
		if len(buf) < pos+length+2 {
			return nil, nil, errors.New("socks5: short domain reply")
		}
		addr = net.ParseIP(string(buf[pos : pos+length]))
		pos += length
	default:
		return nil, nil, fmt.Errorf("socks5: unknown address type %d", buf[3])
	}

	port := int(binary.BigEndian.Uint16(buf[pos : pos+2]))
	pos += 2

	return buf[pos:], &net.UDPAddr{IP: addr, Port: port}, nil
}

// --- net.PacketConn ---

func (c *Socks5UDPConn) ReadFrom(p []byte) (int, net.Addr, error) {
	buf := make([]byte, len(p)+262) // room for the largest SOCKS5 UDP header
	n, err := c.relay.Read(buf)
	if err != nil {
		return 0, nil, err
	}

	payload, addr, err := DecodeUDPReply(buf[:n])
	if err != nil {
		return 0, nil, err
	}

	return copy(p, payload), addr, nil
}

func (c *Socks5UDPConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	udpAddr, ok := addr.(*net.UDPAddr)
	if !ok {
		resolved, err := net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return 0, err
		}
		udpAddr = resolved
	}

	framed, err := EncodeUDPRequest(udpAddr, p)
	if err != nil {
		return 0, err
	}

	if _, err := c.relay.Write(framed); err != nil {
		return 0, err
	}

	// Report the caller's payload length, not the framed length, so callers
	// don't see a short write.
	return len(p), nil
}

func (c *Socks5UDPConn) Close() error {
	relayErr := c.relay.Close()
	controlErr := c.control.Close()
	if relayErr != nil {
		return relayErr
	}
	return controlErr
}

func (c *Socks5UDPConn) LocalAddr() net.Addr                { return c.relay.LocalAddr() }
func (c *Socks5UDPConn) SetDeadline(t time.Time) error      { return c.relay.SetDeadline(t) }
func (c *Socks5UDPConn) SetReadDeadline(t time.Time) error  { return c.relay.SetReadDeadline(t) }
func (c *Socks5UDPConn) SetWriteDeadline(t time.Time) error { return c.relay.SetWriteDeadline(t) }

var _ net.PacketConn = (*Socks5UDPConn)(nil)
