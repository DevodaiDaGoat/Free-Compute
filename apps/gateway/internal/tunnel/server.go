package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
)

type Server struct {
	cfg            Config
	logger         *log.Logger
	registry       *Registry
	agents         *agentPool
	httpServer     *http.Server
	proxyTransport *http.Transport
}

func NewServer(cfg Config, logger *log.Logger) (*Server, error) {
	proxyTransport := newProxyTransport(cfg)
	registry, err := NewRegistry(cfg.Routes, RegistryOptions{
		Transport:     proxyTransport,
		BufferPool:    newByteBufferPool(proxyBufferSize),
		FlushInterval: cfg.ProxyFlushInterval,
	})
	if err != nil {
		return nil, err
	}

	if logger == nil {
		logger = log.Default()
	}

	server := &Server{
		cfg:            cfg,
		logger:         logger,
		registry:       registry,
		agents:         newAgentPool(),
		proxyTransport: proxyTransport,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/capabilities", server.handleCapabilities)
	mux.HandleFunc("/routes", server.handleRoutes)
	mux.HandleFunc("/proxy/", server.handleReverseProxy)
	mux.HandleFunc("/connect/", server.handleConnect)
	mux.HandleFunc("/ws/", server.handleWebSocketTunnel)
	mux.HandleFunc("/agent/", server.handleAgentTunnel)
	mux.HandleFunc("/signal/", server.handleSignal)

	server.httpServer = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return server, nil
}

func (s *Server) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(s.registry.All())+1)
	var wg sync.WaitGroup

	for _, route := range s.registry.All() {
		switch route.Protocol {
		case ProtocolTCP, ProtocolSSH:
			if route.Listen != "" {
				wg.Add(1)
				go func(route *Route) {
					defer wg.Done()
					if err := s.serveTCP(ctx, route); err != nil && !errors.Is(err, http.ErrServerClosed) {
						errCh <- err
					}
				}(route)
			}
		case ProtocolUDP:
			if route.Listen != "" {
				wg.Add(1)
				go func(route *Route) {
					defer wg.Done()
					if err := s.serveUDP(ctx, route); err != nil && !errors.Is(err, http.ErrServerClosed) {
						errCh <- err
					}
				}(route)
			}
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Printf("http gateway listening on %s", s.cfg.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var err error
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case err = <-errCh:
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
	defer shutdownCancel()
	if shutdownErr := s.httpServer.Shutdown(shutdownCtx); shutdownErr != nil && err == nil {
		err = shutdownErr
	}
	s.proxyTransport.CloseIdleConnections()

	wg.Wait()
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(nil, w, r) {
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"routes": s.registry.PublicRoutes(),
	})
}

func (s *Server) handleCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"gateway": "freecompute-universal-proxy",
		"protocols": []Protocol{
			ProtocolHTTP,
			ProtocolHTTPS,
			ProtocolWebSocket,
			ProtocolTCP,
			ProtocolUDP,
			ProtocolSSH,
			ProtocolWebRTC,
			ProtocolP2P,
		},
		"transports": []string{
			"http-connect",
			"websocket",
			"webrtc-data-channel",
			"tcp",
			"udp",
		},
		"clientPaths": map[string]map[string]any{
			"browser": {
				"proxyPath":           "/proxy/{routeID}/{path}",
				"websocketTunnelPath": "/ws/{routeID}",
				"signalingPath":       "/signal/{routeID}/rooms/{roomID}",
			},
			"webos-app": {
				"proxyPath":           "/proxy/{routeID}/{path}",
				"connectPath":         "/connect/{routeID}",
				"websocketTunnelPath": "/ws/{routeID}",
				"signalingPath":       "/signal/{routeID}/rooms/{roomID}",
				"rawTcpListener":      true,
				"rawUdpListener":      true,
			},
			"native-client": {
				"connectPath":    "/connect/{routeID}",
				"signalingPath":  "/signal/{routeID}/rooms/{roomID}",
				"rawTcpListener": true,
				"rawUdpListener": true,
			},
			"host-agent": {
				"connectPath": "/agent/{routeID}",
			},
			"edge-worker": {
				"proxyPath":     "/proxy/{routeID}/{path}",
				"signalingPath": "/signal/{routeID}/rooms/{roomID}",
			},
		},
		"routeModes": []string{
			"edge-relay",
			"direct-p2p",
			"host-tunnel",
		},
		"bunnyCdn": map[string]any{
			"cacheable": []string{
				"static frontend assets",
				"immutable WebOS assets",
				"relay discovery documents",
			},
			"bypassCache": []string{
				"/proxy/*",
				"/ws/*",
				"/connect/*",
				"/agent/*",
				"/signal/*",
			},
			"supportsAcceleration": true,
		},
	})
}

func routeIDFromPath(prefix string, path string) (string, string) {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return "", ""
	}

	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}

	return parts[0], "/" + parts[1]
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
