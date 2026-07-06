package tunnel

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Route struct {
	RouteConfig
	targetURL    *url.URL
	reverseProxy *httputil.ReverseProxy
}

type Registry struct {
	mu     sync.RWMutex
	routes map[string]*Route
}

type RegistryOptions struct {
	Transport     http.RoundTripper
	BufferPool    httputil.BufferPool
	FlushInterval time.Duration
}

type PublicRoute struct {
	ID       string   `json:"id"`
	Protocol Protocol `json:"protocol"`
	Listen   string   `json:"listen,omitempty"`
	Target   string   `json:"target,omitempty"`
}

func NewRegistry(configs []RouteConfig, options RegistryOptions) (*Registry, error) {
	registry := &Registry{routes: make(map[string]*Route, len(configs))}

	for _, cfg := range configs {
		if err := cfg.Validate(); err != nil {
			return nil, err
		}
		if _, exists := registry.routes[cfg.ID]; exists {
			return nil, fmt.Errorf("duplicate route id %q", cfg.ID)
		}

		route := &Route{RouteConfig: cfg}
		if cfg.Protocol == ProtocolHTTP || cfg.Protocol == ProtocolHTTPS || cfg.Protocol == ProtocolWebSocket {
			targetURL, err := parseProxyTarget(cfg.Target)
			if err != nil {
				return nil, fmt.Errorf("parse route %q target URL: %w", cfg.ID, err)
			}
			route.targetURL = targetURL
			route.reverseProxy = &httputil.ReverseProxy{
				Transport:     options.Transport,
				BufferPool:    options.BufferPool,
				FlushInterval: options.FlushInterval,
			}
		}

		registry.routes[cfg.ID] = route
	}

	return registry, nil
}

func (r *Registry) Get(id string) (*Route, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	route, ok := r.routes[id]
	return route, ok
}

func (r *Registry) All() []*Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*Route, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, route)
	}

	return routes
}

func (r *Registry) PublicRoutes() []PublicRoute {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]PublicRoute, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, PublicRoute{
			ID:       route.ID,
			Protocol: route.Protocol,
			Listen:   route.Listen,
			Target:   route.Target,
		})
	}

	return routes
}
