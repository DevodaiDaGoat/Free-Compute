package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"golang.org/x/net/http2"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/freecompute/free-compute/apps/gateway/internal/browsing"
	"github.com/freecompute/free-compute/apps/gateway/internal/webrtc"
	"github.com/freecompute/free-compute/apps/gateway/internal/session"
	"github.com/freecompute/free-compute/apps/gateway/internal/input"
	"github.com/freecompute/free-compute/apps/gateway/internal/audio"
	"github.com/freecompute/free-compute/apps/gateway/internal/transfer"
	"github.com/freecompute/free-compute/apps/gateway/internal/gaming"
	"github.com/freecompute/free-compute/apps/gateway/internal/auth"
	"github.com/freecompute/free-compute/apps/gateway/internal/database"
	"github.com/freecompute/free-compute/apps/gateway/internal/storage"
	"github.com/freecompute/free-compute/apps/gateway/internal/security"
	"github.com/freecompute/free-compute/apps/gateway/internal/admin"
	"github.com/freecompute/free-compute/apps/gateway/internal/monitoring"
	"github.com/freecompute/free-compute/apps/gateway/internal/moderation"
	"github.com/freecompute/free-compute/apps/gateway/internal/images"
	"github.com/freecompute/free-compute/apps/gateway/internal/keys"
	"github.com/freecompute/free-compute/apps/gateway/internal/firewall"
	"github.com/freecompute/free-compute/apps/gateway/internal/usage"
	"github.com/freecompute/free-compute/apps/gateway/internal/ratelimit"
)

type TailscaleHost struct {
	TailscaleIP string   `json:"tailscaleIp"`
	HostName    string   `json:"hostName"`
	UserID      string   `json:"userId,omitempty"`
	VMs         []TailscaleVM `json:"vms"`
	LastSeen    time.Time `json:"lastSeen"`
}

type TailscaleVM struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	TailIP string `json:"tailIp,omitempty"`
}

type UserTailscaleIP struct {
	UserID      string    `json:"userId"`
	TailscaleIP string    `json:"tailscaleIp"`
	HostName    string    `json:"hostName"`
	ProxyMode   string    `json:"proxyMode"` // "direct", "relay", "disabled"
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type Server struct {
	cfg            Config
	logger         *log.Logger
	registry       *Registry
	agents         *agentPool
	httpServer     *http.Server
	proxyTransport *http.Transport
	webrtcServer   *webrtc.Server
	sessionManager *session.SessionManager
	inputManager   *input.InputManager
	audioStreamer  *audio.AudioStreamer
	transferManager *transfer.TransferManager
	clipboardManager *transfer.ClipboardManager
	gamingManager  *gaming.GamingManager
	authManager    *auth.AuthManager
	authHandler    *auth.AuthHandler
	storageManager *storage.StorageManager
	storageHandler *storage.StorageHandler
	securityDetector *security.SecurityDetector
	securityHandler *security.SecurityHandler
	adminManager    *admin.AdminManager
	adminHandler    *admin.AdminHandler
	tailscaleHosts  map[string]*TailscaleHost
	tailscaleMu     sync.RWMutex
	userTailscaleIPs map[string]*UserTailscaleIP
	userTailscaleMu  sync.RWMutex

	signalStore     *signalStore
	metrics         *monitoring.Metrics
	healthChecker   *monitoring.HealthChecker
	collector       *monitoring.Collector

	imageManager    *images.Manager
	keyManager      *keys.Manager
	firewallManager *firewall.Manager
	usageTracker    *usage.Tracker

	db             *database.DB
	reportHandler  *ReportHandler

	relayMesh           *webrtc.RelayMesh
	qualityTracker      *QualityTracker
	compressMiddleware  *compressionMiddleware
	proxyCache          *ProxyCache
	checkpointStore     *CheckpointStore
	sessionRecorder     *SessionRecorder

	rateLimiter *ratelimit.Limiter

	userConns   sync.Map // userID -> int32 (active connections)
	userConnsMu sync.Mutex
}

func NewServer(cfg Config, logger *log.Logger) (*Server, error) {
	proxyTransport := newProxyTransport(cfg, NewDNSCache(time.Duration(cfg.DNSTTLSeconds)*time.Second, cfg.DNSMaxEntries))
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

	// Initialize WebRTC server with codec support
	codecSupport := webrtc.CodecSupport{
		H264Enabled:   true,
		H265Enabled:   true,
		AV1Enabled:    false,
		VP8Enabled:    true,
		VP9Enabled:    true,
		OpusEnabled:   true,
		AACEnabled:    true,
		HardwareAccel: true,
	}

	webrtcServer, err := webrtc.NewServer(logger, codecSupport, "", "")
	if err != nil {
		return nil, err
	}

	var db *database.DB
	if cfg.DBPath != "" {
		var err error
		db, err = database.Open(cfg.DBPath)
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
	}

	var authMgr *auth.AuthManager
	if db != nil {
		authMgr = auth.NewAuthManagerWithDB(logger, db)
	} else {
		authMgr = auth.NewAuthManager(logger)
	}
	storeMgr := storage.NewStorageManager(logger, "/tmp/freecompute-storage")
	secDetector := security.NewSecurityDetector(logger)

	if db != nil {
		if err := secDetector.ReloadSignatures(db); err != nil {
			logger.Printf("reload security signatures: %v", err)
		}
	}

	admMgr := admin.NewAdminManager(logger, authMgr, secDetector)
	admMgr.SeedAdmin()

	metrics := monitoring.GetMetrics()
	healthChecker := monitoring.NewHealthChecker()
	collector := monitoring.NewCollector(metrics, healthChecker, logger, 15*time.Second)

	imgMgr := images.NewManager(logger)
	keyMgr := keys.NewManager(logger)
	fwMgr := firewall.NewManager(logger)
	usageTrk := usage.NewTracker(logger)

	healthChecker.RegisterComponent("gateway")
	healthChecker.RegisterComponent("http")
	healthChecker.RegisterComponent("webrtc")
	healthChecker.RegisterComponent("tunnel")
	healthChecker.RegisterComponent("images")
	healthChecker.RegisterComponent("keys")
	healthChecker.RegisterComponent("firewall")
	healthChecker.RegisterComponent("usage")

	hostAllocator := session.NewHostAllocator(logger)

	sigStore := &signalStore{rooms: map[string]*signalRoom{}}
	go sigStore.sweepLoop()

	SetTCPCCAlgo(cfg.TCPCCAlgo)
	SetTCPBufferSize(cfg.TCPBufferSize)
	SetUDPBufferSize(cfg.UDPBufferSize)

	relayMesh := webrtc.NewRelayMesh()
	for _, peer := range cfg.MeshPeers {
		relayMesh.AddPeer(&webrtc.MeshPeer{
			ID:   peer,
			Addr: peer,
		})
	}

	var aiMod moderation.ModerationAI = &moderation.HeuristicModerator{}
	if cfg.ModerationLLMURL != "" {
		aiMod = moderation.NewLLMModerator(cfg.ModerationLLMURL, cfg.ModerationLLMKey)
	}
	reportHandler := NewReportHandler(db, authMgr, aiMod, security.GetAIModerationActive)

	server := &Server{
		cfg:               cfg,
		logger:            logger,
		registry:          registry,
		agents:            newAgentPool(),
		proxyTransport:    proxyTransport,
		webrtcServer:      webrtcServer,
		sessionManager:    session.NewSessionManager(logger, hostAllocator),
		inputManager:      input.NewInputManager(logger),
		audioStreamer:     audio.NewAudioStreamer(logger),
		transferManager:   transfer.NewTransferManager(logger),
		clipboardManager:  transfer.NewClipboardManager(logger),
		gamingManager:     gaming.NewGamingManager(logger),
		authManager:       authMgr,
		authHandler:       auth.NewAuthHandler(authMgr, db),
		storageManager:    storeMgr,
		storageHandler:    storage.NewStorageHandler(storeMgr),
		securityDetector:  secDetector,
		securityHandler:   security.NewSecurityHandler(secDetector),
		adminManager:      admMgr,
		adminHandler:      admin.NewAdminHandler(admMgr, authMgr, secDetector, db),
		tailscaleHosts:    make(map[string]*TailscaleHost),
		userTailscaleIPs:  make(map[string]*UserTailscaleIP),
		signalStore:       sigStore,
		relayMesh:         relayMesh,
		metrics:           metrics,
		healthChecker:     healthChecker,
		collector:         collector,
		imageManager:      imgMgr,
		keyManager:        keyMgr,
		firewallManager:   fwMgr,
		usageTracker:      usageTrk,
		db:                db,
		reportHandler:     reportHandler,
		compressMiddleware: newCompressionMiddleware(cfg.MaxProxyCacheMB, logger),
		proxyCache:        NewProxyCache(cfg.MaxProxyCacheMB, logger),
		checkpointStore:   NewCheckpointStore(int64(cfg.MaxProxyCacheMB)*1024*1024, logger),
		sessionRecorder:   NewSessionRecorder(logger),
	}

	// Wire up storage quota check with auth
	storeMgr.SetQuotaCheck(func(userID string, additionalBytes int64) error {
		return authMgr.CheckStorageQuota(userID, additionalBytes)
	})
	storeMgr.SetUsageFunc(func(userID string, bytes int64) {
		authMgr.AddStorageUsed(userID, bytes)
	})

	server.rateLimiter = ratelimit.NewLimiter(cfg.RateLimitRPM/60, cfg.RateLimitRPM/4)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/capabilities", server.handleCapabilities)
	mux.HandleFunc("/routes", auth.RequireAuth(authMgr, server.handleRoutes))
	mux.Handle("/proxy/", server.rateLimitByIPAndUser(server.handleReverseProxy))
	mux.HandleFunc("/browsing/modes", server.handleBrowsingModes)
	mux.Handle("/connect/", server.rateLimitByIPAndUser(server.handleConnect))
	mux.HandleFunc("/ssh/", server.handleSSHTunnel)
	mux.Handle("/ws/", server.rateLimitByIPAndUser(server.handleWebSocketTunnel))
	mux.HandleFunc("/agent/", server.handleAgentTunnel)
	mux.HandleFunc("/signal/", server.handleSignal)
	mux.HandleFunc("/webrtc/", server.handleWebRTC)
	mux.HandleFunc("/sessions/", server.handleSessions)
	mux.HandleFunc("/input/", server.handleInput)
	mux.HandleFunc("/audio/", server.handleAudio)
	mux.HandleFunc("/transfer/", server.handleTransfer)
	mux.HandleFunc("/clipboard/", server.handleClipboard)
	mux.HandleFunc("/gaming/", server.handleGaming)
	mux.HandleFunc("/media/", server.handleMediaIngest)
	mux.HandleFunc("/data/", server.handleDataIngest)
	mux.HandleFunc("/keyframe/", server.handleRequestKeyframe)
	mux.HandleFunc("/tailscale/register", server.handleTailscaleRegister)
	mux.HandleFunc("/tailscale/hosts", server.handleTailscaleHosts)
	mux.HandleFunc("/tailscale/user", server.handleUserTailscale)
	mux.HandleFunc("/tailscale/proxy", server.handleTailscaleProxy)
	mux.HandleFunc("/hosts/", server.handleHosts)
	mux.HandleFunc("/hosts/register", auth.RequireAuth(authMgr, server.handleHostRegister))
	mux.HandleFunc("/hosts/metrics", auth.RequireAuth(authMgr, server.handleHostMetrics))

	// Admin routes
	adminWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(authMgr, auth.RequireRole(2, authMgr, h))
	}
	modWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(authMgr, auth.RequireRole(1, authMgr, h))
	}
	mux.HandleFunc("/admin/dashboard", modWrap(server.adminHandler.Dashboard))
	mux.HandleFunc("/admin/users", modWrap(server.adminHandler.ListUsers))
	mux.HandleFunc("/admin/users/delete", adminWrap(server.adminHandler.DeleteUser))
	mux.HandleFunc("/admin/threats", modWrap(server.adminHandler.ListThreats))
	mux.HandleFunc("/admin/threats/review", modWrap(server.adminHandler.ReviewThreat))
	mux.HandleFunc("/admin/vm/pause", modWrap(server.adminHandler.PauseVM))
	mux.HandleFunc("/admin/vm/resume", modWrap(server.adminHandler.ResumeVM))
	mux.HandleFunc("/admin/settings", adminWrap(server.adminHandler.Settings))
	mux.HandleFunc("/admin/auto-detect", adminWrap(server.adminHandler.AutoDetect))

	// Security routes
	mux.HandleFunc("/security/metrics", adminWrap(server.securityHandler.ReportMetrics))
	mux.HandleFunc("/security/process", adminWrap(server.securityHandler.ReportProcess))
	mux.HandleFunc("/security/stats", adminWrap(server.securityHandler.Stats))

	// Personalization routes
	mux.Handle("/auth/preferences", server.rateLimitByIP(auth.RequireAuth(authMgr, server.authHandler.Preferences)))
	mux.HandleFunc("/auth/personalization/sync-request", auth.RequireAuth(authMgr, server.authHandler.RequestPersonalizationSync))
	mux.HandleFunc("/admin/personalization/sync-approve", modWrap(server.adminHandler.ApprovePersonalizationSync))
	mux.HandleFunc("/admin/role", adminWrap(server.adminHandler.SetRole))

	// Reports routes
	mux.HandleFunc("/reports", auth.RequireAuth(authMgr, server.reportHandler.Create))
	mux.HandleFunc("/reports/list", auth.RequireRole(1, authMgr, server.reportHandler.List))
	mux.HandleFunc("/reports/action", auth.RequireRole(1, authMgr, server.reportHandler.Action))

	// Auth routes
	mux.Handle("/auth/register", server.rateLimitByIP(server.authHandler.Register))
	mux.Handle("/auth/login", server.rateLimitByIP(server.authHandler.Login))
	mux.HandleFunc("/auth/profile", auth.RequireAuth(authMgr, server.authHandler.Profile))
	mux.HandleFunc("/auth/tailscale-ip", auth.RequireAuth(authMgr, server.authHandler.AllocateIP))

	// Monitoring routes
	mux.HandleFunc("/metrics", server.metricsHandler)
	mux.HandleFunc("/health/detail", server.healthChecker.HandleHealthDetailed)

	// VM Image routes
	mux.HandleFunc("/images", server.imageManager.HandleImages)
	mux.HandleFunc("/images/", server.imageManager.HandleImageOps)
	mux.HandleFunc("/snapshots", server.imageManager.HandleSnapshots)
	mux.HandleFunc("/snapshots/", server.imageManager.HandleSnapshotOps)

	// SSH Key routes
	mux.HandleFunc("/keys", server.keyManager.HandleKeys)
	mux.HandleFunc("/keys/", server.keyManager.HandleKeyOps)

	// Prewarm / early connection establishment
	mux.HandleFunc("/prewarm", server.handlePrewarm)

	// Speed test / connection checker
	if server.cfg.EnableSpeedTest {
		mux.HandleFunc("/api/v1/speedtest", auth.RequireAuth(authMgr, server.handleSpeedTest))
	}

	// Session recording
	mux.HandleFunc("/sessions/recordings", adminWrap(server.handleListRecordings))
	mux.HandleFunc("/sessions/recordings/", adminWrap(server.handleRecordingOps))

	// Firewall routes
	mux.HandleFunc("/firewall/rules", modWrap(server.firewallManager.HandleRules))
	mux.HandleFunc("/firewall/rules/", modWrap(server.firewallManager.HandleRuleOps))
	mux.HandleFunc("/firewall/groups", modWrap(server.firewallManager.HandleGroups))
	mux.HandleFunc("/firewall/assign", modWrap(server.firewallManager.HandleGroupAssign))

	// Usage & quota routes
	mux.HandleFunc("/usage", auth.RequireAuth(authMgr, server.usageTracker.HandleUsage))
	mux.HandleFunc("/quota", auth.RequireAuth(authMgr, server.usageTracker.HandleQuota))
	mux.HandleFunc("/invoice", auth.RequireAuth(authMgr, server.usageTracker.HandleInvoice))

	// Storage routes (authenticated)
	storageAuth := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(authMgr, func(w http.ResponseWriter, r *http.Request) {
			user := auth.UserFromContext(r)
			if user == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			q := r.URL.Query()
			q.Set("userId", user.ID)
			r.URL.RawQuery = q.Encode()
			h(w, r)
		})
	}
	mux.HandleFunc("/storage/list", storageAuth(server.storageHandler.List))
	mux.HandleFunc("/storage/upload", storageAuth(server.storageHandler.Upload))
	mux.HandleFunc("/storage/download", storageAuth(server.storageHandler.Download))
	mux.HandleFunc("/storage/delete", storageAuth(server.storageHandler.Delete))

	server.healthChecker.ReportHealth("gateway", monitoring.HealthOK, "running", 0)
	server.healthChecker.ReportHealth("http", monitoring.HealthOK, fmt.Sprintf("listening on %s", cfg.Addr), 0)
	server.healthChecker.ReportHealth("tunnel", monitoring.HealthOK, fmt.Sprintf("%d routes loaded", len(cfg.Routes)), 0)

	handler := withCommonHeaders(server.compressMiddleware.Handler(mux), cfg)
	server.httpServer = &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadBufferSize:    256 * 1024,
		WriteBufferSize:   256 * 1024,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	if err := http2.ConfigureServer(server.httpServer, &http2.Server{
		MaxConcurrentStreams: 1000,
		MaxReadFrameSize:     1 << 20, // 1MB
	}); err != nil {
		logger.Printf("configure HTTP/2: %v", err)
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

	// Start metrics collector
	collectorCtx, collectorCancel := context.WithCancel(ctx)
	go s.collector.Start(collectorCtx)
	defer collectorCancel()

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

func (s *Server) rateLimitByIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !s.rateLimiter.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitByUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r)
		if user != nil {
			if !s.rateLimiter.AllowUser(user.ID) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitByIPAndUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" {
			ip = r.RemoteAddr
		}
		if !s.rateLimiter.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
			return
		}
		user := auth.UserFromContext(r)
		if user != nil {
			if !s.rateLimiter.AllowUser(user.ID) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return xrip
	}
	return ""
}

func (s *Server) incrementUserConns(userID string) bool {
	if userID == "" {
		return true
	}
	if s.cfg.MaxConnsPerUser <= 0 {
		return true
	}
	s.userConnsMu.Lock()
	defer s.userConnsMu.Unlock()
	current, _ := s.userConns.LoadOrStore(userID, int32(0))
	if current.(int32) >= int32(s.cfg.MaxConnsPerUser) {
		return false
	}
	s.userConns.Store(userID, current.(int32)+1)
	return true
}

func (s *Server) decrementUserConns(userID string) {
	if userID == "" {
		return
	}
	s.userConnsMu.Lock()
	defer s.userConnsMu.Unlock()
	if current, ok := s.userConns.Load(userID); ok {
		newVal := current.(int32) - 1
		if newVal <= 0 {
			s.userConns.Delete(userID)
		} else {
			s.userConns.Store(userID, newVal)
		}
	}
}

func (s *Server) writeConnLimitReached(w http.ResponseWriter, routeID string) {
	s.logger.Printf("connection limit reached route=%s user", routeID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many connections for user"})
}

func withCommonHeaders(next http.Handler, cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if cfg.CDNHostname != "" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FreeCompute-Tunnel-Token")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(s.metrics.PrometheusText()))
}

func (s *Server) handlePrewarm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Keep-Alive", "timeout=15")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"warm","ttl":15}`))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleBrowsingModes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"default": string(browsing.ModeCasual),
		"modes": []map[string]any{
			{
				"mode":        string(browsing.ModeSpeed),
				"label":       "Speed",
				"description": "Max speed, minimal overhead, no filtering, aggressive caching",
			},
			{
				"mode":        string(browsing.ModePrivacy),
				"label":       "Privacy",
				"description": "Header stripping, tracker/ads blocking, DNT/Sec-GPC, minimal logging",
			},
			{
				"mode":        string(browsing.ModeCasual),
				"label":       "Casual",
				"description": "Chrome-like default, safe search, blocks trackers/ads/malware/miners/illegal",
			},
		},
	})
}

func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r)
	if user == nil && !s.authorize(nil, w, r) {
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
			"tailscale",
		},
		"transportEndpoints": map[string]string{
			"websocket":    "/ws/",
			"webrtc":       "/signal/",
			},
		"preferredTransports": map[string][]string{
			"browser-modern":  {"webtransport", "webrtc-data-channel", "websocket"},
			"browser-legacy":  {"websocket", "http-connect", "tcp"},
			"webos":           {"websocket", "http-connect", "tcp"},
			"native":          {"tcp", "quic", "websocket", "http-connect"},
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
			"hostname":   s.cfg.CDNHostname,
			"edgeHost":   s.cfg.EdgeHostname,
			"apiHost":    s.cfg.APIHostname,
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

func (s *Server) handleWebRTC(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleCreateWebRTCSession(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateWebRTCSession(w http.ResponseWriter, r *http.Request) {
	var req webrtc.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.Preset == "" {
		req.Preset = "safe"
	}
	if len(req.VideoCodecs) == 0 {
		req.VideoCodecs = []string{"h264", "vp8", "vp9"}
	}
	if len(req.AudioCodecs) == 0 {
		req.AudioCodecs = []string{"opus", "aac"}
	}
	if req.Resolution.Width == 0 {
		req.Resolution.Width = 1920
	}
	if req.Resolution.Height == 0 {
		req.Resolution.Height = 1080
	}
	if req.Resolution.RefreshRate == 0 {
		req.Resolution.RefreshRate = 60
	}
	if req.RequestedFPS == 0 {
		req.RequestedFPS = 60
	}
	if req.LatencyTarget == 0 {
		req.LatencyTarget = 50 // 50ms default latency target
	}

	resp, err := s.webrtcServer.CreateSession(&req)
	if err != nil {
		s.logger.Printf("failed to create WebRTC session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleCreateSession(w, r)
	} else if r.Method == http.MethodGet {
		s.handleGetSession(w, r)
	} else if r.Method == http.MethodDelete {
		s.handleEndSession(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req session.CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := s.sessionManager.CreateSession(r.Context(), &req)
	if err != nil {
		s.logger.Printf("failed to create session: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/sessions/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	sess, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleEndSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/sessions/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Reason == "" {
		req.Reason = "user-requested"
	}

	if err := s.sessionManager.EndSession(sessionID, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ended"})
}

func (s *Server) handleInput(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleInputEvent(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleInputEvent(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/input/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	var event input.InputEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.inputManager.HandleInputEvent(sessionID, &event); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "processed"})
}

func (s *Server) handleAudio(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleAudioFrame(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAudioFrame(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/audio/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	var frame audio.AudioFrame
	if err := json.NewDecoder(r.Body).Decode(&frame); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	output, err := s.audioStreamer.ProcessAudioData(sessionID, frame.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessionId": sessionID,
		"data":      output,
	})
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleCreateTransfer(w, r)
	} else if r.Method == http.MethodPut {
		s.handleTransferChunk(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SessionID   string `json:"sessionId"`
		Direction   string `json:"direction"`
		Filename    string `json:"filename"`
		Size        int64  `json:"size"`
		ContentType string `json:"contentType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	transfer, err := s.transferManager.CreateTransfer(
		req.SessionID,
		transfer.TransferDirection(req.Direction),
		req.Filename,
		req.Size,
		req.ContentType,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, transfer)
}

func (s *Server) handleTransferChunk(w http.ResponseWriter, r *http.Request) {
	var chunk transfer.ChunkRequest
	if err := json.NewDecoder(r.Body).Decode(&chunk); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := s.transferManager.ProcessChunk(&chunk)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleClipboard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleClipboardWrite(w, r)
	} else if r.Method == http.MethodGet {
		s.handleClipboardRead(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleClipboardWrite(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/clipboard/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := s.clipboardManager.Write(sessionID, req.MimeType, req.Data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "written"})
}

func (s *Server) handleClipboardRead(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/clipboard/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	data, err := s.clipboardManager.Read(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func extractResourceID(path string, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	if trimmed == "" {
		return ""
	}
	
	// Extract just the ID (everything before /)
	parts := strings.Split(trimmed, "/")
	return parts[0]
}

func (s *Server) handleGaming(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.handleCreateGamingSession(w, r)
	} else if r.Method == http.MethodPut {
		s.handleUpdateGamingState(w, r)
	} else if r.Method == http.MethodGet {
		s.handleGetGamingState(w, r)
	} else {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCreateGamingSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/gaming/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	var config gaming.GamingConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Set defaults
	if config.TargetFPS == 0 {
		config.TargetFPS = 60
	}
	if config.TargetLatency == 0 {
		config.TargetLatency = 20
	}
	if config.Mode == "" {
		config.Mode = gaming.GamingModeStandard
	}

	session, err := s.gamingManager.CreateGamingSession(sessionID, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (s *Server) handleUpdateGamingState(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/gaming/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	var req struct {
		Action string `json:"action"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case "register-controller":
		var data struct {
			ControllerID string `json:"controllerId"`
			Vendor       string `json:"vendor"`
		}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			http.Error(w, "invalid controller data", http.StatusBadRequest)
			return
		}
		if err := s.gamingManager.RegisterController(sessionID, data.ControllerID, data.Vendor); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "update-controller":
		var data struct {
			ControllerID string            `json:"controllerId"`
			Buttons      []gaming.ButtonState `json:"buttons"`
			Axes         []float64          `json:"axes"`
		}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			http.Error(w, "invalid controller data", http.StatusBadRequest)
			return
		}
		if err := s.gamingManager.UpdateControllerState(sessionID, data.ControllerID, data.Buttons, data.Axes); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "set-rumble":
		var data struct {
			ControllerID string  `json:"controllerId"`
			Enabled      bool    `json:"enabled"`
			Intensity    float64 `json:"intensity"`
		}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			http.Error(w, "invalid rumble data", http.StatusBadRequest)
			return
		}
		if err := s.gamingManager.SetControllerRumble(sessionID, data.ControllerID, data.Enabled, data.Intensity); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "update-metrics":
		var data struct {
			FPS       float64 `json:"fps"`
			FrameTime float64 `json:"frameTime"`
			Bitrate   float64 `json:"bitrate"`
			Latency   float64 `json:"latency"`
		}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			http.Error(w, "invalid metrics data", http.StatusBadRequest)
			return
		}
		if err := s.gamingManager.UpdatePerformanceMetrics(sessionID, data.FPS, data.FrameTime, data.Bitrate, data.Latency); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case "optimize-mode":
		var data struct {
			Mode gaming.GamingMode `json:"mode"`
		}
		if err := json.Unmarshal(req.Data, &data); err != nil {
			http.Error(w, "invalid mode data", http.StatusBadRequest)
			return
		}
		if err := s.gamingManager.OptimizeForMode(sessionID, data.Mode); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleGetGamingState(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/gaming/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}

	session, err := s.gamingManager.GetGamingSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleMediaIngest(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/media/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}
	s.webrtcServer.HandleMediaIngest(w, r)
}

func (s *Server) handleDataIngest(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/data/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}
	s.webrtcServer.HandleDataIngest(w, r)
}

func (s *Server) handleRequestKeyframe(w http.ResponseWriter, r *http.Request) {
	sessionID := extractResourceID(r.URL.Path, "/keyframe/")
	if sessionID == "" {
		http.Error(w, "session ID required", http.StatusBadRequest)
		return
	}
	s.webrtcServer.HandleRequestKeyframe(w, r)
}

func (s *Server) handleTailscaleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var info struct {
		TailscaleIP string       `json:"tailscaleIp"`
		HostName    string       `json:"hostName"`
		VMs         []TailscaleVM `json:"vms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if info.TailscaleIP == "" {
		http.Error(w, "tailscaleIp required", http.StatusBadRequest)
		return
	}
	s.tailscaleMu.Lock()
	s.tailscaleHosts[info.TailscaleIP] = &TailscaleHost{
		TailscaleIP: info.TailscaleIP,
		HostName:    info.HostName,
		VMs:         info.VMs,
		LastSeen:    time.Now(),
	}
	s.tailscaleMu.Unlock()
	s.logger.Printf("tailscale host registered: IP=%s name=%s vms=%d", info.TailscaleIP, info.HostName, len(info.VMs))
	writeJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (s *Server) handleTailscaleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.tailscaleMu.RLock()
	hosts := make([]*TailscaleHost, 0, len(s.tailscaleHosts))
	for _, h := range s.tailscaleHosts {
		if time.Since(h.LastSeen) < 5*time.Minute {
			hosts = append(hosts, h)
		}
	}
	s.tailscaleMu.RUnlock()
	writeJSON(w, http.StatusOK, map[string]any{"hosts": hosts})
}

// Per-user Tailscale IP allocation
func (s *Server) handleUserTailscale(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			UserID      string `json:"userId"`
			TailscaleIP string `json:"tailscaleIp"`
			HostName    string `json:"hostName"`
			ProxyMode   string `json:"proxyMode"` // "direct", "relay", "disabled"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if req.UserID == "" || req.TailscaleIP == "" {
			http.Error(w, "userId and tailscaleIp required", http.StatusBadRequest)
			return
		}
		if req.ProxyMode == "" {
			req.ProxyMode = "direct"
		}
		s.userTailscaleMu.Lock()
		s.userTailscaleIPs[req.UserID] = &UserTailscaleIP{
			UserID:      req.UserID,
			TailscaleIP: req.TailscaleIP,
			HostName:    req.HostName,
			ProxyMode:   req.ProxyMode,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(24 * time.Hour),
		}
		s.userTailscaleMu.Unlock()
		s.logger.Printf("user tailscale: user=%s ip=%s mode=%s", req.UserID, req.TailscaleIP, req.ProxyMode)
		writeJSON(w, http.StatusOK, map[string]string{"status": "mapped"})

	case http.MethodGet:
		userID := r.URL.Query().Get("userId")
		s.userTailscaleMu.RLock()
		if userID != "" {
			if entry, ok := s.userTailscaleIPs[userID]; ok {
				s.userTailscaleMu.RUnlock()
				writeJSON(w, http.StatusOK, entry)
				return
			}
			s.userTailscaleMu.RUnlock()
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		entries := make([]*UserTailscaleIP, 0, len(s.userTailscaleIPs))
		s.userTailscaleMu.RUnlock()
		s.userTailscaleMu.Lock()
		for key, e := range s.userTailscaleIPs {
			if time.Since(e.ExpiresAt) < 0 {
				entries = append(entries, e)
			} else {
				delete(s.userTailscaleIPs, key)
			}
		}
		s.userTailscaleMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"users": entries})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// Tailscale proxy fallback - returns proxy connection info for Tailscale IPs
func (s *Server) handleTailscaleProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TargetIP    string `json:"targetIp"`
		TargetPort  int    `json:"targetPort"`
		Protocol    string `json:"protocol"` // "tcp", "udp"
		UserID      string `json:"userId,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Build proxy route for this Tailscale target
	routeID := "ts-proxy-" + strings.ReplaceAll(req.TargetIP, ".", "-")
	proxyURL := fmt.Sprintf("http://%s:%d", req.TargetIP, req.TargetPort)

	s.logger.Printf("tailscale proxy: target=%s:%d protocol=%s route=%s", req.TargetIP, req.TargetPort, req.Protocol, routeID)

	writeJSON(w, http.StatusOK, map[string]any{
		"routeId":  routeID,
		"proxyUrl": proxyURL,
		"wsUrl":    fmt.Sprintf("/ws/%s", routeID),
		"connectUrl": fmt.Sprintf("/connect/%s", routeID),
		"fallback": true,
	})
}

// SSH tunnel handler - SSH-over-WebSocket for browser clients
func (s *Server) handleSSHTunnel(w http.ResponseWriter, r *http.Request) {
	routeID, _ := routeIDFromPath("/ssh/", r.URL.Path)
	route, ok := s.registry.Get(routeID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if !s.authorize(route, w, r) {
		return
	}
	if route.Protocol != ProtocolSSH && route.Protocol != ProtocolTCP {
		http.Error(w, "route does not support SSH tunneling", http.StatusBadRequest)
		return
	}

	if isWebSocketUpgrade(r) {
		s.handleWebSocketTunnel(w, r)
		return
	}

	// HTTP CONNECT for SSH
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}

	http.Error(w, "use WebSocket upgrade or HTTP CONNECT for SSH", http.StatusBadRequest)
}

// Host registration API (for vm-setup agent)
func (s *Server) handleHostRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var info map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	s.logger.Printf("host registered: %v", info)
	writeJSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (s *Server) handleHostMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var metrics map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	vmID, _ := metrics["vmId"].(string)
	cpuUsage, _ := metrics["cpuUsage"].(float64)
	gpuUsage, _ := metrics["gpuUsage"].(float64)
	networkTx, _ := metrics["networkTx"].(float64)
	networkRx, _ := metrics["networkRx"].(float64)
	processes := 0
	if p, ok := metrics["processes"].(float64); ok {
		processes = int(p)
	} else if p, ok := metrics["processes"].(int); ok {
		processes = p
	}

	network := networkTx + networkRx

	if s.securityDetector != nil {
		s.securityDetector.AnalyzeMetrics(vmID, cpuUsage, gpuUsage, network, processes)
	}

	if s.db != nil && vmID != "" {
		if vm, err := s.db.GetVM(vmID); err == nil && vm != nil {
			bytesOut := int64(networkTx * 125_000)
			bytesIn := int64(networkRx * 125_000)
			if s.securityDetector != nil {
				s.securityDetector.AnalyzeTraffic(vm.UserID, bytesIn, bytesOut, 1, 5*time.Second)
			}
			if s.usageTracker != nil {
				s.usageTracker.Track(vm.UserID, usage.ResourceNetwork, float64(bytesOut))
			}
		}
	}

	s.logger.Printf("host metrics: vmid=%v cpu=%v ram=%v", metrics["vmId"], metrics["cpuUsage"], metrics["ramUsage"])
	writeJSON(w, http.StatusOK, map[string]string{"status": "received"})
}

func (s *Server) handleHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]interface{}{"hosts": []interface{}{}})
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) dialViaTailscale(ctx context.Context, route *Route) (net.Conn, error) {
	if route.UsesAgentTunnel() {
		return nil, errors.New("route uses agent tunnel; cannot dial directly via tailscale")
	}
	s.tailscaleMu.RLock()
	hosts := make([]*TailscaleHost, 0, len(s.tailscaleHosts))
	for _, host := range s.tailscaleHosts {
		if time.Since(host.LastSeen) <= 5*time.Minute {
			hosts = append(hosts, host)
		}
	}
	s.tailscaleMu.RUnlock()

	if len(hosts) == 0 {
		return nil, errors.New("no tailscale host available")
	}

	targetPort := ""
	if _, port, err := net.SplitHostPort(route.Target); err == nil {
		targetPort = port
	}
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, len(hosts))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, host := range hosts {
		host := host
		go func() {
			dialer := net.Dialer{Timeout: 5 * time.Second}
			conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host.TailscaleIP, targetPort))
			if err == nil {
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					_ = tcpConn.SetNoDelay(true)
					_ = setTCPKeepaliveAggressive(tcpConn)
				}
			}
			ch <- result{conn, err}
		}()
	}

	var firstErr error
	for i := 0; i < len(hosts); i++ {
		select {
		case r := <-ch:
			if r.err == nil {
				s.logger.Printf("tailscale direct route=%s via %s", route.ID, r.conn.RemoteAddr())
				return r.conn, nil
			}
			if firstErr == nil {
				firstErr = r.err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, firstErr
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

func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.sessionRecorder == nil {
		writeJSON(w, http.StatusOK, map[string]any{"recordings": []any{}})
		return
	}
	recordings := s.sessionRecorder.ListRecordings()
	writeJSON(w, http.StatusOK, map[string]any{"recordings": recordings})
}

func (s *Server) handleRecordingOps(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/sessions/recordings/") {
		sessionID := extractResourceID(r.URL.Path, "/sessions/recordings/")
		if sessionID == "" {
			http.Error(w, "session ID required", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var req RecordingConfig
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if s.sessionRecorder == nil {
				http.Error(w, "recording not available", http.StatusNotImplemented)
				return
			}
			_, err := s.sessionRecorder.StartRecording(sessionID, req)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]string{"status": "recording-started", "sessionId": sessionID})
		case http.MethodDelete:
			if s.sessionRecorder == nil {
				writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
				return
			}
			if err := s.sessionRecorder.StopRecording(sessionID); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "recording-stopped", "sessionId": sessionID})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}



