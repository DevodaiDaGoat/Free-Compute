package agent

import (
	"context"
	"runtime"

	"github.com/rs/zerolog/log"
)

// Register authenticates with the gateway and registers this host.
// SECURITY: Registration requires:
// 1. Valid mTLS client certificate (issued during manual admin approval)
// 2. Host attestation data (hardware/software fingerprint)
// 3. The gateway must verify the certificate against the CA before accepting
func (a *Agent) Register(ctx context.Context) error {
	if a.cfg.HostID == "" {
		return &AgentError{Message: "HOST_ID is required — obtain one from admin during provisioning"}
	}

	log.Info().
		Str("host_id", a.cfg.HostID).
		Str("host_name", a.cfg.HostName).
		Str("region", a.cfg.Region).
		Int("num_cpu", runtime.NumCPU()).
		Msg("registering host with gateway")

	// TODO: Establish mTLS connection to gateway
	// TODO: Send registration payload:
	//   - host_id (pre-assigned during admin approval)
	//   - hostname, region
	//   - hardware specs (CPU, RAM, GPU, disk)
	//   - software attestation (OS version, QEMU version, agent version)
	// TODO: Gateway verifies mTLS cert and host_id match, returns session token

	_ = ctx
	return nil
}

type AgentError struct {
	Message string
}

func (e *AgentError) Error() string {
	return e.Message
}
