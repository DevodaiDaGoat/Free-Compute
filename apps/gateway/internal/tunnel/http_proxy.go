package tunnel

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
	"github.com/freecompute/free-compute/apps/gateway/internal/browsing"
)

func newProxyTransport(cfg Config, dnsCache *DNSCache) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   cfg.DialTimeout,
		KeepAlive: 30 * time.Second,
	}

	if dnsCache != nil {
		dnsDialer := newDNSCacheDialer(dialer, dnsCache)
		return &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dnsDialer.DialContext,
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

type proxyTargetKey struct{}

type proxyTargetInfo struct {
	target       *url.URL
	upstreamPath string
	routeID      string
}

func withProxyTarget(r *http.Request, info *proxyTargetInfo) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), proxyTargetKey{}, info))
}

func proxyDirector(req *http.Request) {
	info, ok := req.Context().Value(proxyTargetKey{}).(*proxyTargetInfo)
	if !ok {
		return
	}
	rewriteProxyRequest(req, info.target, info.upstreamPath)
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

	if s.proxyCache.ServeCached(w, r, route) {
		return
	}

	if route.Protocol != ProtocolHTTP && route.Protocol != ProtocolHTTPS && route.Protocol != ProtocolWebSocket {
		http.Error(w, "route does not support HTTP proxying", http.StatusBadRequest)
		return
	}

	policy := s.effectiveBrowsingPolicy(route, r)

	// Speed mode skips all filtering and relies on aggressive caching only.
	if policy.Mode != browsing.ModeSpeed {
		if !policy.IsURLAllowed(r.URL.String()) {
			http.Error(w, "blocked by browsing policy", http.StatusForbidden)
			return
		}
		if modified := policy.ModifySearchURL(r.URL.String()); modified != r.URL.String() {
			if rewritten, err := url.Parse(modified); err == nil {
				r.URL = rewritten
			}
		}
	}
	policy.ApplyHeaders(r)

	proxy := *route.reverseProxy
	target := *route.targetURL
	proxy.Director = proxyDirector
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		s.logger.Printf("http proxy route=%s mode=%s error=%v", route.ID, policy.Mode, err)
		http.Error(rw, "upstream unavailable", http.StatusBadGateway)
	}
	if policy.AggressiveCache {
		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Set("Cache-Control", "public, max-age=3600, stale-while-revalidate=86400")
			return nil
		}
	}
	proxy.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), proxyTargetKey{}, &proxyTargetInfo{
		target:       &target,
		upstreamPath: upstreamPath,
		routeID:      routeID,
	})))
}

// effectiveBrowsingPolicy resolves the BrowsingPolicy for a proxy request using the
// precedence: route-level BrowsingMode > authenticated user preferences >
// gateway default (FREECOMPUTE_DEFAULT_BROWSING_MODE) > ModeCasual.
func (s *Server) effectiveBrowsingPolicy(route *Route, r *http.Request) browsing.BrowsingPolicy {
	var mode browsing.BrowsingMode

	if route.BrowsingMode != "" {
		mode = browsing.BrowsingMode(strings.ToLower(strings.TrimSpace(route.BrowsingMode)))
	}

	if mode == "" {
		if user := auth.UserFromContext(r); user != nil && s.authManager != nil {
			if prefs, err := s.authManager.GetPreferences(user.ID); err == nil {
				if prefMode := browsing.ModeFromPreferences(prefs); prefMode != "" {
					mode = prefMode
				}
			}
		}
	}

	if mode == "" && s.cfg.DefaultBrowsingMode != "" {
		mode = browsing.BrowsingMode(s.cfg.DefaultBrowsingMode)
	}

	if mode == "" {
		mode = browsing.ModeCasual
	}

	return browsing.DefaultPolicy(mode)
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
