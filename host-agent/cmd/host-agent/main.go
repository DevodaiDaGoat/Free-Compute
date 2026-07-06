package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultPoolSize         = 2
	defaultDialSeconds      = 10
	defaultReconnectSeconds = 1
)

type Config struct {
	GatewayURL      string
	Token           string
	Routes          []RouteConfig
	DialTimeout     time.Duration
	ReconnectDelay  time.Duration
	InsecureSkipTLS bool
}

type RouteConfig struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	PoolSize int    `json:"poolSize,omitempty"`
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func main() {
	logger := log.New(os.Stdout, "host-agent ", log.LstdFlags|log.LUTC|log.Lmicroseconds)

	cfg, err := loadConfig()
	if err != nil {
		logger.Fatalf("config error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup
	for _, route := range cfg.Routes {
		route := route
		for i := 0; i < route.poolSize(); i++ {
			wg.Add(1)
			go func(slot int) {
				defer wg.Done()
				runTunnelLoop(ctx, cfg, route, slot, logger)
			}(i)
		}
	}

	wg.Wait()
}

func loadConfig() (Config, error) {
	cfg := Config{
		GatewayURL:      strings.TrimRight(os.Getenv("FREECOMPUTE_AGENT_GATEWAY_URL"), "/"),
		Token:           os.Getenv("FREECOMPUTE_AGENT_TOKEN"),
		DialTimeout:     secondsFromEnv("FREECOMPUTE_AGENT_DIAL_SECONDS", defaultDialSeconds),
		ReconnectDelay:  secondsFromEnv("FREECOMPUTE_AGENT_RECONNECT_SECONDS", defaultReconnectSeconds),
		InsecureSkipTLS: os.Getenv("FREECOMPUTE_AGENT_INSECURE_SKIP_TLS") == "1",
	}
	if cfg.GatewayURL == "" {
		return cfg, errors.New("FREECOMPUTE_AGENT_GATEWAY_URL is required")
	}

	rawRoutes := strings.TrimSpace(os.Getenv("FREECOMPUTE_AGENT_ROUTES"))
	if rawRoutes == "" {
		return cfg, errors.New("FREECOMPUTE_AGENT_ROUTES is required")
	}
	if err := json.Unmarshal([]byte(rawRoutes), &cfg.Routes); err != nil {
		return cfg, fmt.Errorf("parse FREECOMPUTE_AGENT_ROUTES: %w", err)
	}
	for _, route := range cfg.Routes {
		if strings.TrimSpace(route.ID) == "" {
			return cfg, errors.New("agent route id is required")
		}
		if _, _, err := net.SplitHostPort(route.Target); err != nil {
			return cfg, fmt.Errorf("route %q target must be host:port: %w", route.ID, err)
		}
	}

	return cfg, nil
}

func (r RouteConfig) poolSize() int {
	if r.PoolSize <= 0 {
		return defaultPoolSize
	}

	return r.PoolSize
}

func runTunnelLoop(ctx context.Context, cfg Config, route RouteConfig, slot int, logger *log.Logger) {
	for {
		if err := runTunnelOnce(ctx, cfg, route, logger); err != nil && ctx.Err() == nil {
			logger.Printf("route=%s slot=%d disconnected: %v", route.ID, slot, err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.ReconnectDelay):
		}
	}
}

func runTunnelOnce(ctx context.Context, cfg Config, route RouteConfig, logger *log.Logger) error {
	localConn, err := dialLocalTarget(ctx, cfg, route)
	if err != nil {
		return err
	}
	defer localConn.Close()

	gatewayConn, buffered, err := connectToGateway(ctx, cfg, route)
	if err != nil {
		return err
	}
	defer gatewayConn.Close()

	logger.Printf("route=%s connected gateway to local target=%s", route.ID, route.Target)
	bridge(ctx, &bufferedConn{Conn: gatewayConn, reader: buffered}, localConn)
	return nil
}

func connectToGateway(ctx context.Context, cfg Config, route RouteConfig) (net.Conn, *bufio.Reader, error) {
	gatewayURL, err := url.Parse(cfg.GatewayURL)
	if err != nil {
		return nil, nil, err
	}
	if gatewayURL.Scheme != "http" && gatewayURL.Scheme != "https" {
		return nil, nil, fmt.Errorf("unsupported gateway scheme %q", gatewayURL.Scheme)
	}

	dialer := &net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 30 * time.Second}
	addr := hostWithDefaultPort(gatewayURL)

	var conn net.Conn
	if gatewayURL.Scheme == "https" {
		tlsDialer := tls.Dialer{NetDialer: dialer, Config: &tls.Config{
			ServerName:         gatewayURL.Hostname(),
			InsecureSkipVerify: cfg.InsecureSkipTLS,
		}}
		conn, err = tlsDialer.DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, nil, err
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
	}

	pathPrefix := strings.TrimRight(gatewayURL.EscapedPath(), "/")
	path := pathPrefix + "/agent/" + url.PathEscape(route.ID)
	requestLines := []string{
		fmt.Sprintf("CONNECT %s HTTP/1.1", path),
		"Host: " + gatewayURL.Host,
		"User-Agent: freecompute-host-agent",
	}
	if cfg.Token != "" {
		requestLines = append(requestLines, "Authorization: Bearer "+cfg.Token)
	}
	request := strings.Join(requestLines, "\r\n") + "\r\n\r\n"

	if _, err := conn.Write([]byte(request)); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	if response.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("gateway rejected tunnel route=%s status=%s", route.ID, response.Status)
	}

	return conn, reader, nil
}

func dialLocalTarget(ctx context.Context, cfg Config, route RouteConfig) (net.Conn, error) {
	dialer := net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", route.Target)
	if err != nil {
		return nil, err
	}
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		_ = tcpConn.SetNoDelay(true)
		_ = tcpConn.SetKeepAlive(true)
	}

	return conn, nil
}

func bridge(ctx context.Context, left net.Conn, right net.Conn) {
	errCh := make(chan error, 2)
	go copyConn(errCh, left, right)
	go copyConn(errCh, right, left)

	select {
	case <-ctx.Done():
	case <-errCh:
	}
}

func copyConn(errCh chan<- error, dst io.Writer, src io.Reader) {
	_, err := io.Copy(dst, src)
	if closeWriter, ok := dst.(interface{ CloseWrite() error }); ok {
		_ = closeWriter.CloseWrite()
	}
	errCh <- err
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	if c.reader != nil && c.reader.Buffered() > 0 {
		return c.reader.Read(p)
	}

	return c.Conn.Read(p)
}

func hostWithDefaultPort(gatewayURL *url.URL) string {
	if gatewayURL.Port() != "" {
		return gatewayURL.Host
	}

	switch gatewayURL.Scheme {
	case "https":
		return net.JoinHostPort(gatewayURL.Hostname(), "443")
	default:
		return net.JoinHostPort(gatewayURL.Hostname(), "80")
	}
}

func secondsFromEnv(name string, fallback int) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return time.Duration(fallback) * time.Second
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return time.Duration(fallback) * time.Second
	}

	return time.Duration(parsed) * time.Second
}
