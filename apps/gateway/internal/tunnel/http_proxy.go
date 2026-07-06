package tunnel

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func newProxyTransport(cfg Config) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		IdleConnTimeout:       cfg.UpstreamIdleTimeout,
		TLSHandshakeTimeout:   cfg.DialTimeout,
		ResponseHeaderTimeout: cfg.ResponseHeaderTimeout,
		ExpectContinueTimeout: cfg.ExpectContinueTimeout,
		DisableCompression:    true,
	}
}

func (s *Server) handleReverseProxy(w http.ResponseWriter, r *http.Request) {
	routeID, upstreamPath := routeIDFromPath("/proxy/", r.URL.Path)
	route, ok := s.registry.Get(routeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(route, w, r) {
		return
	}

	if route.Protocol != ProtocolHTTP && route.Protocol != ProtocolHTTPS && route.Protocol != ProtocolWebSocket {
		http.Error(w, "route does not support HTTP proxying", http.StatusBadRequest)
		return
	}

	proxy := *route.reverseProxy
	target := *route.targetURL
	proxy.Director = func(req *http.Request) {
		rewriteProxyRequest(req, &target, upstreamPath)
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		s.logger.Printf("http proxy route=%s error=%v", route.ID, err)
		http.Error(rw, "upstream unavailable", http.StatusBadGateway)
	}
	proxy.ServeHTTP(w, r)
}

func rewriteProxyRequest(req *http.Request, target *url.URL, upstreamPath string) {
	originalHost := req.Host
	originalScheme := schemeFromRequest(req)

	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.URL.Path = singleJoiningSlash(target.Path, upstreamPath)
	req.Host = target.Host

	if target.RawQuery == "" || req.URL.RawQuery == "" {
		req.URL.RawQuery = target.RawQuery + req.URL.RawQuery
	} else {
		req.URL.RawQuery = target.RawQuery + "&" + req.URL.RawQuery
	}

	appendForwardedHeader(req, "X-Forwarded-Host", originalHost)
	appendForwardedHeader(req, "X-Forwarded-Proto", originalScheme)
	if clientIP := clientIPFromRemoteAddr(req.RemoteAddr); clientIP != "" {
		appendForwardedHeader(req, "X-Forwarded-For", clientIP)
		req.Header.Set("X-Real-IP", clientIP)
	}
}

func appendForwardedHeader(req *http.Request, key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	if existing := req.Header.Get(key); existing != "" {
		req.Header.Set(key, existing+", "+value)
		return
	}

	req.Header.Set(key, value)
}

func schemeFromRequest(req *http.Request) string {
	if req.TLS != nil {
		return "https"
	}

	if forwarded := req.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		return forwarded
	}

	return "http"
}

func clientIPFromRemoteAddr(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	return strings.TrimSpace(remoteAddr)
}

func singleJoiningSlash(a string, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	default:
		return a + b
	}
}
