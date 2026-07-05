package host

import "time"

// HealthStatus represents the overall health of a host.
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthOffline   HealthStatus = "offline"
)

// DetermineHealth evaluates a host's health based on its last heartbeat and metrics.
func DetermineHealth(lastHeartbeat time.Time, metrics *Metrics) HealthStatus {
	elapsed := time.Since(lastHeartbeat)

	if elapsed > 5*time.Minute {
		return HealthOffline
	}

	if elapsed > 2*time.Minute {
		return HealthUnhealthy
	}

	if metrics != nil {
		if metrics.CPUUsage > 0.95 || metrics.RAMUsedGB/metrics.RAMTotalGB > 0.95 {
			return HealthDegraded
		}
	}

	return HealthHealthy
}
