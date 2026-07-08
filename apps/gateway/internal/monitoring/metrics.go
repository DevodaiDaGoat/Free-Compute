package monitoring

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	mu sync.RWMutex

	RequestsTotal       atomic.Int64
	RequestsActive      atomic.Int64
	Requests2xx         atomic.Int64
	Requests4xx         atomic.Int64
	Requests5xx         atomic.Int64

	SessionsTotal       atomic.Int64
	SessionsActive      atomic.Int64
	SessionsDesktop     atomic.Int64
	SessionsGaming      atomic.Int64
	SessionsRemote      atomic.Int64

	WebRTCConnections    atomic.Int64
	WebSocketConnections atomic.Int64
	TCPConnections       atomic.Int64
	UDPConnections       atomic.Int64

	AgentConnections    atomic.Int64
	AgentPoolSize       atomic.Int64

	BytesUploaded       atomic.Int64
	BytesDownloaded     atomic.Int64
	TransferOperations  atomic.Int64

	ProxyRequests       atomic.Int64
	ProxyBytesProxied   atomic.Int64

	startTime time.Time
	labels    map[string]string
}

var instance *Metrics
var once sync.Once

func GetMetrics() *Metrics {
	once.Do(func() {
		instance = &Metrics{
			startTime: time.Now(),
			labels:    make(map[string]string),
		}
	})
	return instance
}

func (m *Metrics) RecordRequest(statusCode int) {
	m.RequestsTotal.Add(1)
	m.RequestsActive.Add(1)

	switch {
	case statusCode >= 200 && statusCode < 300:
		m.Requests2xx.Add(1)
	case statusCode >= 400 && statusCode < 500:
		m.Requests4xx.Add(1)
	case statusCode >= 500:
		m.Requests5xx.Add(1)
	}
}

func (m *Metrics) CompleteRequest() {
	m.RequestsActive.Add(-1)
}

func (m *Metrics) Snapshot() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.startTime).Seconds()

	return map[string]any{
		"uptime_seconds":      uptime,
		"requests_total":      m.RequestsTotal.Load(),
		"requests_active":     m.RequestsActive.Load(),
		"requests_2xx":        m.Requests2xx.Load(),
		"requests_4xx":        m.Requests4xx.Load(),
		"requests_5xx":        m.Requests5xx.Load(),
		"sessions_total":      m.SessionsTotal.Load(),
		"sessions_active":     m.SessionsActive.Load(),
		"sessions_desktop":    m.SessionsDesktop.Load(),
		"sessions_gaming":     m.SessionsGaming.Load(),
		"sessions_remote":     m.SessionsRemote.Load(),
		"webrtc_connections":  m.WebRTCConnections.Load(),
		"websocket_connections": m.WebSocketConnections.Load(),
		"tcp_connections":     m.TCPConnections.Load(),
		"udp_connections":     m.UDPConnections.Load(),
		"agent_connections":   m.AgentConnections.Load(),
		"agent_pool_size":     m.AgentPoolSize.Load(),
		"bytes_uploaded":      m.BytesUploaded.Load(),
		"bytes_downloaded":    m.BytesDownloaded.Load(),
		"transfer_operations": m.TransferOperations.Load(),
		"proxy_requests":      m.ProxyRequests.Load(),
		"proxy_bytes_proxied": m.ProxyBytesProxied.Load(),
		"started_at":          m.startTime.Format(time.RFC3339),
	}
}

func (m *Metrics) PrometheusText() string {
	snap := m.Snapshot()
	var out string

	out += fmt.Sprintf("# HELP freecompute_uptime_seconds Uptime in seconds\n")
	out += fmt.Sprintf("# TYPE freecompute_uptime_seconds gauge\n")
	out += fmt.Sprintf("freecompute_uptime_seconds %v\n", snap["uptime_seconds"])

	out += fmt.Sprintf("# HELP freecompute_requests_total Total requests\n")
	out += fmt.Sprintf("# TYPE freecompute_requests_total counter\n")
	out += fmt.Sprintf("freecompute_requests_total %d\n", snap["requests_total"])

	out += fmt.Sprintf("# HELP freecompute_requests_active Active requests\n")
	out += fmt.Sprintf("# TYPE freecompute_requests_active gauge\n")
	out += fmt.Sprintf("freecompute_requests_active %d\n", snap["requests_active"])

	out += fmt.Sprintf("# HELP freecompute_requests_2xx 2xx responses\n")
	out += fmt.Sprintf("# TYPE freecompute_requests_2xx counter\n")
	out += fmt.Sprintf("freecompute_requests_2xx %d\n", snap["requests_2xx"])

	out += fmt.Sprintf("# HELP freecompute_requests_4xx 4xx responses\n")
	out += fmt.Sprintf("# TYPE freecompute_requests_4xx counter\n")
	out += fmt.Sprintf("freecompute_requests_4xx %d\n", snap["requests_4xx"])

	out += fmt.Sprintf("# HELP freecompute_requests_5xx 5xx responses\n")
	out += fmt.Sprintf("# TYPE freecompute_requests_5xx counter\n")
	out += fmt.Sprintf("freecompute_requests_5xx %d\n", snap["requests_5xx"])

	out += fmt.Sprintf("# HELP freecompute_sessions_active Active sessions\n")
	out += fmt.Sprintf("# TYPE freecompute_sessions_active gauge\n")
	out += fmt.Sprintf("freecompute_sessions_active %d\n", snap["sessions_active"])

	out += fmt.Sprintf("# HELP freecompute_webrtc_connections WebRTC connections\n")
	out += fmt.Sprintf("# TYPE freecompute_webrtc_connections gauge\n")
	out += fmt.Sprintf("freecompute_webrtc_connections %d\n", snap["webrtc_connections"])

	out += fmt.Sprintf("# HELP freecompute_agent_connections Agent connections\n")
	out += fmt.Sprintf("# TYPE freecompute_agent_connections gauge\n")
	out += fmt.Sprintf("freecompute_agent_connections %d\n", snap["agent_connections"])

	return out
}
