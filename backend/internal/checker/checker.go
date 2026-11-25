package checker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rafael/monitor-de-rede-backend/internal/models"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Run executes a health check for the given monitor and returns the CheckResult.
func Run(ctx context.Context, m models.Monitor) models.CheckResult {
	timeout := time.Duration(m.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res := models.CheckResult{
		MonitorID: m.ID,
		CheckedAt: time.Now().UTC(),
	}

	switch m.Type {
	case models.MonitorHTTP:
		checkHTTP(ctx, m, timeout, &res)
	case models.MonitorTCP:
		checkTCP(ctx, m, timeout, &res)
	case models.MonitorSSL:
		checkSSL(ctx, m, timeout, &res)
	case models.MonitorICMP:
		checkICMP(ctx, m, timeout, &res)
	default:
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("unsupported monitor type: %s", m.Type)
	}

	return res
}

func checkHTTP(ctx context.Context, m models.Monitor, timeout time.Duration, res *models.CheckResult) {
	target := strings.TrimSpace(m.Target)
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "https://" + target
	}

	method := strings.ToUpper(strings.TrimSpace(m.HTTPMethod))
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("invalid URL: %v", err)
		return
	}
	req.Header.Set("User-Agent", "NetPulse-Monitor/1.0")

	if strings.TrimSpace(m.HTTPHeaders) != "" {
		var customHeaders map[string]string
		if err := json.Unmarshal([]byte(m.HTTPHeaders), &customHeaders); err == nil {
			for k, v := range customHeaders {
				if strings.TrimSpace(k) != "" {
					req.Header.Set(k, v)
				}
			}
		}
	}

	client := &http.Client{
		Timeout: timeout,
	}

	start := time.Now()
	resp, err := client.Do(req)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Status = models.StatusDown
		res.Error = err.Error()
		return
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode

	// Capture TLS certificate info when HTTPS is used
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		cert := resp.TLS.PeerCertificates[0]
		exp := cert.NotAfter.UTC()
		days := int(time.Until(exp).Hours() / 24)
		res.SSLExpiresAt = &exp
		res.SSLDaysRemaining = &days
	}

	expected := m.ExpectedStatus
	statusOK := false
	if expected > 0 {
		statusOK = resp.StatusCode == expected
	} else {
		statusOK = resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	if !statusOK {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
		return
	}

	if m.Keyword != "" {
		limited := io.LimitReader(resp.Body, 128*1024)
		bodyBytes, err := io.ReadAll(limited)
		if err != nil {
			res.Status = models.StatusDown
			res.Error = fmt.Sprintf("failed reading response body: %v", err)
			return
		}
		if !strings.Contains(string(bodyBytes), m.Keyword) {
			res.Status = models.StatusDown
			res.Error = fmt.Sprintf("keyword %q not found in response", m.Keyword)
			return
		}
	}

	// Check SSL expiry warning if applicable
	if res.SSLDaysRemaining != nil && m.SSLExpiryWarnDays > 0 && *res.SSLDaysRemaining <= m.SSLExpiryWarnDays {
		res.Status = models.StatusDegraded
		res.Error = fmt.Sprintf("SSL certificate expires in %d days", *res.SSLDaysRemaining)
		return
	}

	res.Status = models.StatusUp
}

func checkTCP(ctx context.Context, m models.Monitor, timeout time.Duration, res *models.CheckResult) {
	host := extractHost(m.Target)
	port := m.Port
	if port <= 0 {
		port = 80
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Status = models.StatusDown
		res.Error = err.Error()
		return
	}
	_ = conn.Close()
	res.Status = models.StatusUp
}

func checkSSL(ctx context.Context, m models.Monitor, timeout time.Duration, res *models.CheckResult) {
	host := extractHost(m.Target)
	port := m.Port
	if port <= 0 {
		port = 443
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: timeout},
		Config: &tls.Config{
			ServerName: host,
		},
	}

	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	res.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		res.Status = models.StatusDown
		res.Error = err.Error()
		return
	}
	defer conn.Close()

	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		res.Status = models.StatusDown
		res.Error = "failed to inspect TLS connection state"
		return
	}

	certs := tlsConn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		res.Status = models.StatusDown
		res.Error = "no peer SSL certificates presented"
		return
	}

	cert := certs[0]
	exp := cert.NotAfter.UTC()
	days := int(time.Until(exp).Hours() / 24)
	res.SSLExpiresAt = &exp
	res.SSLDaysRemaining = &days

	if time.Now().UTC().After(exp) {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("SSL certificate expired on %s", exp.Format("2006-01-02"))
		return
	}

	warnDays := m.SSLExpiryWarnDays
	if warnDays <= 0 {
		warnDays = 14
	}
	if days <= warnDays {
		res.Status = models.StatusDegraded
		res.Error = fmt.Sprintf("SSL certificate expires in %d days (%s)", days, exp.Format("2006-01-02"))
		return
	}

	res.Status = models.StatusUp
}

func checkICMP(ctx context.Context, m models.Monitor, timeout time.Duration, res *models.CheckResult) {
	host := extractHost(m.Target)
	ipAddr, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("DNS resolution failed: %v", err)
		return
	}

	isV6 := ipAddr.IP.To4() == nil

	var networks []string
	var proto int
	var msgType icmp.Type
	if isV6 {
		networks = []string{"ip6:ipv6-icmp", "udp6"}
		proto = 58
		msgType = ipv6.ICMPTypeEchoRequest
	} else {
		networks = []string{"ip4:icmp", "udp4"}
		proto = 1
		msgType = ipv4.ICMPTypeEcho
	}

	var conn *icmp.PacketConn
	var listenErr error
	var usedNetwork string
	for _, network := range networks {
		listenAddr := "0.0.0.0"
		if isV6 {
			listenAddr = "::"
		}
		conn, listenErr = icmp.ListenPacket(network, listenAddr)
		if listenErr == nil {
			usedNetwork = network
			break
		}
	}
	if conn == nil {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("ICMP socket unavailable (try CAP_NET_RAW or admin): %v", listenErr)
		return
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	echoID := (os.Getpid() ^ rand.Intn(0xffff)) & 0xffff
	seq := 1
	msg := icmp.Message{
		Type: msgType,
		Code: 0,
		Body: &icmp.Echo{
			ID:   echoID,
			Seq:  seq,
			Data: []byte("NETPULSE-PING"),
		},
	}

	wb, err := msg.Marshal(nil)
	if err != nil {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("failed marshaling ICMP packet: %v", err)
		return
	}

	var dst net.Addr = ipAddr
	if strings.HasPrefix(usedNetwork, "udp") {
		dst = &net.UDPAddr{IP: ipAddr.IP, Zone: ipAddr.Zone}
	}

	start := time.Now()
	if _, err := conn.WriteTo(wb, dst); err != nil {
		res.Status = models.StatusDown
		res.Error = fmt.Sprintf("ICMP write failed: %v", err)
		return
	}

	rb := make([]byte, 1500)
	for {
		if ctx.Err() != nil {
			res.Status = models.StatusDown
			res.Error = "ICMP ping timed out"
			return
		}
		n, _, err := conn.ReadFrom(rb)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				res.Status = models.StatusDown
				res.Error = "ICMP ping timed out"
				return
			}
			res.Status = models.StatusDown
			res.Error = fmt.Sprintf("ICMP read failed: %v", err)
			return
		}

		rm, err := icmp.ParseMessage(proto, rb[:n])
		if err != nil {
			continue
		}

		if (!isV6 && rm.Type == ipv4.ICMPTypeEchoReply) || (isV6 && rm.Type == ipv6.ICMPTypeEchoReply) {
			if echo, ok := rm.Body.(*icmp.Echo); ok {
				// In unprivileged UDP mode, kernel may rewrite ID; accept matching seq when using udp
				if echo.ID == echoID || strings.HasPrefix(usedNetwork, "udp") {
					res.LatencyMs = time.Since(start).Milliseconds()
					res.Status = models.StatusUp
					return
				}
			}
		}
	}
}

func extractHost(target string) string {
	t := strings.TrimSpace(target)
	if strings.Contains(t, "://") {
		if u, err := url.Parse(t); err == nil && u.Hostname() != "" {
			return u.Hostname()
		}
	}
	if h, _, err := net.SplitHostPort(t); err == nil {
		return h
	}
	return strings.TrimSuffix(t, "/")
}
