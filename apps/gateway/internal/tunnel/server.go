package tunnel

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"

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
	sigStoreCancel  context.CancelFunc
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

	userConns   map[string]int32 // userID -> active connections, guarded by userConnsMu
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
	storeMgr := storage.NewStorageManager(logger, filepath.Join(os.TempDir(), "freecompute-storage"))
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
	// Sweeper is bound to the server's shutdown so it doesn't leak the
	// goroutine when the process is stopped in tests or graceful shutdown.
	sigStoreCtx, sigStoreCancel := context.WithCancel(context.Background())
	go sigStore.sweepLoop(sigStoreCtx)

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
		sigStoreCancel:    sigStoreCancel,
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
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/capabilities", http.StatusFound)
	})
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/capabilities", server.handleCapabilities)
	mux.HandleFunc("/routes", auth.RequireAuth(authMgr, server.handleRoutes))
	mux.Handle("/proxy/", server.rateLimitByIPAndUser(http.HandlerFunc(server.handleReverseProxy)))
	mux.HandleFunc("/browsing/modes", server.handleBrowsingModes)
	mux.Handle("/connect/", server.rateLimitByIPAndUser(http.HandlerFunc(server.handleConnect)))
	mux.HandleFunc("/ssh/", server.handleSSHTunnel)
	mux.Handle("/ws/", server.rateLimitByIPAndUser(http.HandlerFunc(server.handleWebSocketTunnel)))
	mux.HandleFunc("/agent/", server.handleAgentTunnel)
	mux.HandleFunc("/signal/", server.webrtcServer.HandleSignal)
	// /webrtc/ POST creates a peer connection + burns one of the 1000 concurrent
	// session slots. Left anonymous, a script can exhaust the pool and DoS
	// legitimate users. Use AuthMiddleware (not RequireAuth) so anon demos still
	// work but the handler can attribute anonymous sessions to a stable
	// per-caller ID (see handleCreateWebRTCSession).
	mux.Handle("/webrtc/", server.rateLimitByIPAndUser(auth.AuthMiddleware(authMgr, server.handleWebRTC)))
	// Accept both "/sessions" and "/sessions/" for GET (list) and POST (create)
	// so callers don't need to guess whether the trailing slash matters. GET
	// with a resource id (e.g. /sessions/{id}) is dispatched by handleSessions.
	sessionsRoot := func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			server.handleListSessions(w, r)
		case http.MethodPost:
			server.handleCreateSession(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
	// AuthMiddleware populates user context if a valid token is present but
	// still permits anonymous callers (needed for the demo-mode POST that
	// creates a session without a login). handleGetSession / handleEndSession
	// enforce ownership internally.
	mux.HandleFunc("/sessions", auth.AuthMiddleware(authMgr, sessionsRoot))
	mux.HandleFunc("/sessions/", auth.AuthMiddleware(authMgr, server.handleSessions))
	mux.HandleFunc("/input/", server.requireSessionOwner("/input/", server.handleInput))
	mux.HandleFunc("/audio/", server.requireSessionOwner("/audio/", server.handleAudio))
	mux.HandleFunc("/transfer/", auth.RequireAuth(authMgr, server.handleTransfer))
	mux.HandleFunc("/clipboard/", server.requireSessionOwner("/clipboard/", server.handleClipboard))
	mux.HandleFunc("/gaming/", server.requireSessionOwner("/gaming/", server.handleGaming))
	mux.HandleFunc("/media/", server.requireSessionIngest("/media/", server.handleMediaIngest))
	mux.HandleFunc("/data/", server.requireSessionIngest("/data/", server.handleDataIngest))
	mux.HandleFunc("/keyframe/", server.requireSessionOwner("/keyframe/", server.handleRequestKeyframe))
	// /tailscale/register was previously anonymous — any caller could inject a
	// fake TailscaleHost record that dialViaTailscale would then attempt to dial
	// (a targeted SSRF pointing at gateway-reachable networks). The endpoint is
	// intended for the host-agent to announce itself, so gate it behind the
	// shared tunnel token or admin auth. handleHostRegister uses the same
	// mechanism.
	mux.HandleFunc("/tailscale/register", server.requireTunnelToken(server.handleTailscaleRegister))
	// /tailscale/hosts leaks the full peer list to any anonymous caller (info
	// disclosure — every registered VM name, Tailscale IP, and VM inventory).
	// Require auth so only logged-in users see the mesh.
	mux.HandleFunc("/tailscale/hosts", auth.RequireAuth(authMgr, server.handleTailscaleHosts))
	mux.HandleFunc("/tailscale/user", auth.RequireAuth(authMgr, server.handleUserTailscale))
	// /tailscale/proxy composes proxy configuration for an arbitrary target
	// IP:port. Anonymous callers could probe internal ranges via the "wsUrl"
	// echo. Auth-gate it; the tunnel token is also honored so host-agents can
	// still request proxy metadata.
	mux.HandleFunc("/tailscale/proxy", server.requireTunnelToken(server.handleTailscaleProxy))
	mux.HandleFunc("/hosts/", server.handleHosts)
	mux.HandleFunc("/hosts/register", server.requireTunnelToken(server.handleHostRegister))
	mux.HandleFunc("/hosts/metrics", server.requireTunnelToken(server.handleHostMetrics))

	// Admin routes
	adminWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(authMgr, auth.RequireRole(2, authMgr, h))
	}
	modWrap := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.RequireAuth(authMgr, auth.RequireRole(1, authMgr, h))
	}
	mux.HandleFunc("/admin/dashboard", modWrap(server.adminHandler.Dashboard))
	mux.HandleFunc("/admin/health", modWrap(server.handleAdminHealth))
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
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		server.rateLimitByIP(http.HandlerFunc(server.authHandler.Register)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		server.rateLimitByIP(http.HandlerFunc(server.authHandler.Login)).ServeHTTP(w, r)
	})
	mux.HandleFunc("/auth/profile", auth.RequireAuth(authMgr, server.authHandler.Profile))
	mux.HandleFunc("/auth/tailscale-ip", auth.RequireAuth(authMgr, server.authHandler.AllocateIP))

	// Monitoring routes
	mux.HandleFunc("/metrics", server.metricsHandler)
	mux.HandleFunc("/health/detail", server.healthChecker.HandleHealthDetailed)

	// VM Image routes
	// /images was previously unauthenticated — any caller could DELETE via
	// ?userId=admin. HandleImageOps now derives caller identity from JWT.
	mux.HandleFunc("/images", auth.RequireAuth(authMgr, server.imageManager.HandleImages))
	mux.HandleFunc("/images/", auth.RequireAuth(authMgr, server.imageManager.HandleImageOps))
	// Snapshots were previously anonymous — any caller could list a VM's
	// snapshots by ID (info leak) or DELETE arbitrary snapshots. Require
	// auth; the handlers themselves still need to enforce ownership on
	// DELETE, but at minimum an unauthenticated caller can no longer poke
	// the endpoint.
	mux.HandleFunc("/snapshots", auth.RequireAuth(authMgr, server.imageManager.HandleSnapshots))
	mux.HandleFunc("/snapshots/", auth.RequireAuth(authMgr, server.imageManager.HandleSnapshotOps))

	// SSH Key routes — RequireAuth; handler enforces IDOR via JWT-derived userID.
	mux.HandleFunc("/keys", auth.RequireAuth(authMgr, server.keyManager.HandleKeys))
	mux.HandleFunc("/keys/", auth.RequireAuth(authMgr, server.keyManager.HandleKeyOps))

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

	// Usage & quota routes — inject authenticated user's own ID to prevent IDOR.
	usageAuth := func(h http.HandlerFunc) http.HandlerFunc {
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
	mux.HandleFunc("/usage", usageAuth(server.usageTracker.HandleUsage))
	mux.HandleFunc("/quota", usageAuth(server.usageTracker.HandleQuota))
	mux.HandleFunc("/invoice", usageAuth(server.usageTracker.HandleInvoice))

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
	server.healthChecker.ReportHealth("webrtc", monitoring.HealthOK, "initialized", 0)
	server.healthChecker.ReportHealth("images", monitoring.HealthOK, "initialized", 0)
	server.healthChecker.ReportHealth("keys", monitoring.HealthOK, "initialized", 0)
	server.healthChecker.ReportHealth("firewall", monitoring.HealthOK, "initialized", 0)
	server.healthChecker.ReportHealth("usage", monitoring.HealthOK, "initialized", 0)

	server.compressMiddleware.SkipPath("/capabilities")
	server.compressMiddleware.SkipPath("/healthz")
	// WebSocket / streaming paths must not be wrapped: even with a Hijack
	// forwarder, the compressed writer would compress the frame stream on
	// the way out. Skip the whole prefix to be safe.
	server.compressMiddleware.SkipPrefix("/signal/")
	server.compressMiddleware.SkipPrefix("/ws/")
	server.compressMiddleware.SkipPrefix("/agent/")
	server.compressMiddleware.SkipPrefix("/connect/")
	server.compressMiddleware.SkipPrefix("/ssh/")
	handler := withCommonHeaders(server.compressMiddleware.Handler(mux), cfg)
	server.httpServer = &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
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
	if s.sigStoreCancel != nil {
		s.sigStoreCancel()
	}
	if s.webrtcServer != nil {
		s.webrtcServer.Shutdown()
	}

	wg.Wait()
	if errors.Is(err, context.Canceled) {
		return nil
	}

	return err
}

// requireTunnelToken gates host-agent endpoints: accepts the tunnel token
// (X-FreeCompute-Tunnel-Token header or Bearer), or a logged-in admin user.
func (s *Server) requireTunnelToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			token = r.Header.Get("X-FreeCompute-Tunnel-Token")
		}
		if s.cfg.TunnelToken != "" {
			if subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.TunnelToken)) == 1 {
				next(w, r)
				return
			}
		}
		// Also accept a logged-in admin so the dashboard can query hosts.
		if user, err := s.authManager.ValidateToken(token); err == nil && auth.RoleLevelOf(user.Role) >= auth.RoleAdmin {
			next(w, r)
			return
		}
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}
}

// sessionOwnerFromPath extracts sessionID from `prefix` (e.g. "/input/"), looks
// the session up in both the sessionManager and webrtcServer, and returns
// (ownerUserID, ok). Returns ("", false) if the session doesn't exist.
func (s *Server) sessionOwnerFromPath(path, prefix string) (string, bool) {
	sessionID := extractResourceID(path, prefix)
	if sessionID == "" {
		return "", false
	}
	if sess, err := s.sessionManager.GetSession(sessionID); err == nil && sess != nil {
		return sess.UserID, true
	}
	if sess, err := s.webrtcServer.GetSession(sessionID); err == nil && sess != nil {
		return sess.ClientID, true
	}
	return "", false
}

// requireSessionOwner enforces JWT auth + verifies the caller owns the session
// referenced by URL path prefix (e.g. "/input/"). Admins bypass ownership.
// If sessionID is missing or the session doesn't exist we return 404.
func (s *Server) requireSessionOwner(prefix string, next http.HandlerFunc) http.HandlerFunc {
	return auth.RequireAuth(s.authManager, func(w http.ResponseWriter, r *http.Request) {
		user := auth.UserFromContext(r)
		if user == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ownerID, ok := s.sessionOwnerFromPath(r.URL.Path, prefix)
		if !ok {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}
		if ownerID != user.ID && auth.RoleLevelOf(user.Role) < auth.RoleAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	})
}

// requireSessionIngest gates the host-agent → gateway ingest endpoints
// (/media/, /data/, /keyframe/). Accepts either the tunnel token (host agents)
// or a caller who owns the referenced session (admin bypass ownership).
func (s *Server) requireSessionIngest(prefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			token = r.Header.Get("X-FreeCompute-Tunnel-Token")
		}
		if s.cfg.TunnelToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.TunnelToken)) == 1 {
			next(w, r)
			return
		}
		user, err := s.authManager.ValidateToken(token)
		if err != nil || user == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ownerID, ok := s.sessionOwnerFromPath(r.URL.Path, prefix)
		if !ok {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}
		if ownerID != user.ID && auth.RoleLevelOf(user.Role) < auth.RoleAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		// Stash the user so handlers can pick up identity when needed.
		next(w, r.WithContext(auth.WithUser(r.Context(), user)))
	}
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
	if s.userConns == nil {
		s.userConns = make(map[string]int32)
	}
	current := s.userConns[userID]
	if current >= int32(s.cfg.MaxConnsPerUser) {
		return false
	}
	s.userConns[userID] = current + 1
	return true
}

func (s *Server) decrementUserConns(userID string) {
	if userID == "" {
		return
	}
	s.userConnsMu.Lock()
	defer s.userConnsMu.Unlock()
	if s.userConns == nil {
		return
	}
	current, ok := s.userConns[userID]
	if !ok {
		return
	}
	newVal := current - 1
	if newVal <= 0 {
		delete(s.userConns, userID)
	} else {
		s.userConns[userID] = newVal
	}
}

func (s *Server) writeConnLimitReached(w http.ResponseWriter, routeID string) {
	s.logger.Printf("connection limit reached route=%s user", routeID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many connections for user"})
}

// resolveAllowedOrigin checks the request origin against a safe allowlist.
// Returns the origin to echo back, or empty string to deny.
func resolveAllowedOrigin(origin string, cfg Config) string {
	if origin == "" {
		return ""
	}
	// Localhost always allowed (dev).
	if strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "https://localhost:") {
		return origin
	}
	// Allow the configured CDN / edge / API hostnames.
	allowedHosts := []string{cfg.CDNHostname, cfg.EdgeHostname, cfg.APIHostname}
	for _, host := range allowedHosts {
		if host == "" {
			continue
		}
		if origin == "https://"+host || origin == "http://"+host {
			return origin
		}
		// Subdomain match.
		if strings.HasSuffix(origin, "."+host) {
			return origin
		}
	}
	return ""
}

func withCommonHeaders(next http.Handler, cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")

		origin := r.Header.Get("Origin")
		allowedOrigin := resolveAllowedOrigin(origin, cfg)
		if allowedOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FreeCompute-Tunnel-Token, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

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

func (s *Server) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
	})
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

	// Attribute the session to the JWT-derived user ID so it appears in the
	// caller's /sessions listing and is subject to per-user rate limits.
	// Anonymous callers fall through to a stable per-request identifier so
	// they can't spoof ClientID via the request body.
	if user := auth.UserFromContext(r); user != nil {
		req.ClientID = user.ID
	} else if req.ClientID == "" {
		req.ClientID = fmt.Sprintf("anon-%x", time.Now().UnixNano())
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

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Initialize to an empty slice so the JSON always serializes as [],
	// never null. The frontend does Array.isArray checks on data.sessions —
	// null would be treated as "no update" and mask real empty states.
	sessionList := []map[string]interface{}{}

	// Enforce per-user filtering: unauthenticated callers see nothing;
	// non-admin users see only sessions they own. Admins see everything.
	user := auth.UserFromContext(r)
	isAdmin := user != nil && auth.RoleLevelOf(user.Role) >= auth.RoleAdmin

	for _, sess := range s.sessionManager.ListSessions() {
		if !isAdmin {
			if user == nil || sess.UserID != user.ID {
				continue
			}
		}
		sessionList = append(sessionList, map[string]interface{}{
			"id":             sess.ID,
			"state":          sess.State,
			"mode":           sess.Mode,
			"type":           sess.Type,
			"resourceClass":  sess.ResourceClass,
			"streamPreset":   sess.StreamProfile.Preset,
			"connectionToken": sess.ConnectionToken,
		})
	}

	for _, entry := range s.webrtcServer.ListSessions() {
		if !isAdmin {
			if user == nil {
				continue
			}
			ownerID, _ := entry["clientId"].(string)
			if ownerID == "" {
				// Fall back to a direct GetSession lookup for older entries.
				if id, ok := entry["id"].(string); ok {
					if sess, err := s.webrtcServer.GetSession(id); err == nil && sess != nil {
						ownerID = sess.ClientID
					}
				}
			}
			if ownerID != user.ID {
				continue
			}
		}
		sessionList = append(sessionList, entry)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": sessionList})
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

	// Derive UserID from the JWT context if the caller is authenticated.
	// Otherwise stamp an anonymous ID so single-user demo / unauthenticated
	// flows don't blow up with "user ID is required" from the session manager.
	if req.UserID == "" {
		if u := auth.UserFromContext(r); u != nil {
			req.UserID = u.ID
		} else {
			req.UserID = fmt.Sprintf("anon-%x", time.Now().UnixNano())
		}
	}

	resp, err := s.sessionManager.CreateSession(r.Context(), &req)
	if err != nil {
		// If host allocation failed (single-node dev or all hosts busy) fall
		// back to a pure WebRTC session — the browser-facing signaling path
		// still works without a full host. This keeps the /sessions endpoint
		// usable in demo installs and matches the "try /sessions then
		// /webrtc" pattern from the connect page.
		if strings.Contains(err.Error(), "no available hosts") ||
			strings.Contains(err.Error(), "no available hosts in region") {
			s.logger.Printf("session create fell back to webrtc: %v", err)
			wrtcReq := &webrtc.CreateSessionRequest{
				ClientID:    req.UserID,
				VideoCodecs: []string{"h264", "vp8", "vp9"},
				AudioCodecs: []string{"opus", "aac"},
				Preset:      string(req.StreamPreset),
				Resolution: webrtc.Resolution{
					Width:       uint32(req.RequestedResolution.Width),
					Height:      uint32(req.RequestedResolution.Height),
					RefreshRate: uint32(req.RequestedResolution.RefreshRate),
				},
				GPURequired: req.GPURequired,
			}
			if wrtcReq.Resolution.Width == 0 {
				wrtcReq.Resolution.Width = 1920
			}
			if wrtcReq.Resolution.Height == 0 {
				wrtcReq.Resolution.Height = 1080
			}
			wResp, wErr := s.webrtcServer.CreateSession(wrtcReq)
			if wErr != nil {
				s.logger.Printf("webrtc fallback also failed: %v", wErr)
				http.Error(w, wErr.Error(), http.StatusInternalServerError)
				return
			}
			// Adapt the webrtc response to the shape the frontend expects
			// (either `session.id` or `sessionId` — RemoteDesktop.tsx handles
			// both — plus `signalingUrl`).
			writeJSON(w, http.StatusCreated, map[string]any{
				"session": map[string]any{
					"id":    wResp.SessionID,
					"state": "connecting",
				},
				"sessionId":    wResp.SessionID,
				"signalingUrl": wResp.SignalingURL,
				"turnServers":  wResp.TURNServers,
				"stunServers":  wResp.STUNServers,
				"videoCodec":   wResp.VideoCodec,
				"audioCodec":   wResp.AudioCodec,
				"expiresAt":    wResp.ExpiresAt,
			})
			return
		}
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

	// Ownership: non-admin callers only see their own sessions.
	user := auth.UserFromContext(r)
	if user == nil || (sess.UserID != user.ID && auth.RoleLevelOf(user.Role) < auth.RoleAdmin) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
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
	// Body is optional on DELETE — treat empty/no-body as "user-requested".
	// Reject only malformed non-empty bodies.
	if r.ContentLength > 0 || r.Header.Get("Content-Type") != "" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	if req.Reason == "" {
		req.Reason = "user-requested"
	}

	// Ownership: only the session's owner (or admin) may end it. Non-owners
	// used to be able to DELETE any session by ID.
	user := auth.UserFromContext(r)
	ownerID, found := s.sessionOwnerFromPath(r.URL.Path, "/sessions/")
	if !found {
		http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
		return
	}
	if user == nil || (ownerID != user.ID && auth.RoleLevelOf(user.Role) < auth.RoleAdmin) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	// Try the session manager first, then fall through to the webrtc-only
	// path — a session created via the /sessions webrtc-fallback lives in
	// the webrtc server, not the session manager. Either succeeding means
	// the client's disconnect landed.
	sessionErr := s.sessionManager.EndSession(sessionID, req.Reason)
	webrtcErr := s.webrtcServer.EndSession(sessionID, req.Reason)
	if sessionErr != nil && webrtcErr != nil {
		http.Error(w, sessionErr.Error(), http.StatusNotFound)
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

	// Ownership check on the referenced session (body-supplied SessionID).
	user := auth.UserFromContext(r)
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if req.SessionID != "" {
		var ownerID string
		if sess, err := s.sessionManager.GetSession(req.SessionID); err == nil && sess != nil {
			ownerID = sess.UserID
		} else if wsess, err := s.webrtcServer.GetSession(req.SessionID); err == nil && wsess != nil {
			ownerID = wsess.ClientID
		} else {
			http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
			return
		}
		if ownerID != user.ID && auth.RoleLevelOf(user.Role) < auth.RoleAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
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

// Per-user Tailscale IP allocation.
//
// This endpoint is wrapped in RequireAuth (see mux registration) so the
// authenticated caller's identity is always available via UserFromContext.
// The GET path returns only the caller's own entry — it never lists other
// users' Tailscale mappings, and admin listing is not exposed here.
// The POST path forces UserID from the JWT to prevent IDOR.
func (s *Server) handleUserTailscale(w http.ResponseWriter, r *http.Request) {
	callerUser := auth.UserFromContext(r)
	if callerUser == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	callerRole := auth.RoleLevelOf(callerUser.Role)

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
		// Force UserID to caller unless admin is explicitly setting for someone else.
		if callerRole < auth.RoleAdmin || req.UserID == "" {
			req.UserID = callerUser.ID
		}
		if req.TailscaleIP == "" {
			http.Error(w, "tailscaleIp required", http.StatusBadRequest)
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
		// Non-admin: always return caller's own entry regardless of query params.
		// Admin: may lookup a specific userId (still per-user, not a list).
		targetID := callerUser.ID
		if callerRole >= auth.RoleAdmin {
			if q := r.URL.Query().Get("userId"); q != "" {
				targetID = q
			}
		}

		s.userTailscaleMu.RLock()
		entry, ok := s.userTailscaleIPs[targetID]
		s.userTailscaleMu.RUnlock()
		if !ok {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		// Filter out expired entries.
		if time.Now().After(entry.ExpiresAt) {
			s.userTailscaleMu.Lock()
			delete(s.userTailscaleIPs, targetID)
			s.userTailscaleMu.Unlock()
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, entry)

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
	vmID, _ := metrics["hostId"].(string)
	cpuUsage, _ := metrics["cpuUsagePercent"].(float64)
	ramUsage, _ := metrics["ramUsagePercent"].(float64)
	gpuUsage, _ := metrics["gpuUsagePercent"].(float64)
	networkTx, _ := metrics["networkTxMbps"].(float64)
	networkRx, _ := metrics["networkRxMbps"].(float64)
	activeStreams := 0
	if p, ok := metrics["activeStreams"].(float64); ok {
		activeStreams = int(p)
	}

	network := networkTx + networkRx

	if s.securityDetector != nil {
		s.securityDetector.AnalyzeMetrics(vmID, cpuUsage, gpuUsage, network, activeStreams)
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

	s.logger.Printf("host metrics: vmid=%s cpu=%g ram=%g gpu=%g tx=%g rx=%g streams=%d", vmID, cpuUsage, ramUsage, gpuUsage, networkTx, networkRx, activeStreams)
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
	drainRemaining := func(remaining int) {
		if remaining <= 0 {
			return
		}
		go func() {
			for j := 0; j < remaining; j++ {
				r := <-ch
				if r.conn != nil {
					_ = r.conn.Close()
				}
			}
		}()
	}
	for i := 0; i < len(hosts); i++ {
		select {
		case r := <-ch:
			if r.err == nil {
				s.logger.Printf("tailscale direct route=%s via %s", route.ID, r.conn.RemoteAddr())
				drainRemaining(len(hosts) - i - 1)
				return r.conn, nil
			}
			if firstErr == nil {
				firstErr = r.err
			}
		case <-ctx.Done():
			drainRemaining(len(hosts) - i)
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



