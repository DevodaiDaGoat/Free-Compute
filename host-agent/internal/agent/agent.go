package agent

import (
	"context"
	"time"

	"github.com/DevodaiDaGoat/Free-Compute/host-agent/internal/config"
	"github.com/DevodaiDaGoat/Free-Compute/host-agent/internal/vm"
	"github.com/rs/zerolog/log"
)

// Agent is the main host agent that manages VMs and communicates with the scheduler.
type Agent struct {
	cfg       *config.Config
	vmManager *vm.Manager
}

func New(cfg *config.Config) (*Agent, error) {
	vmMgr, err := vm.NewManager(cfg)
	if err != nil {
		return nil, err
	}

	return &Agent{
		cfg:       cfg,
		vmManager: vmMgr,
	}, nil
}

// Run starts the main agent loop (heartbeat, command polling).
func (a *Agent) Run(ctx context.Context) {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	log.Info().Str("host_id", a.cfg.HostID).Msg("agent running")

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			a.sendHeartbeat(ctx)
		}
	}
}

// Shutdown gracefully stops all VMs and cleans up resources.
func (a *Agent) Shutdown() {
	log.Info().Msg("stopping all VMs")
	a.vmManager.StopAll()
}

func (a *Agent) sendHeartbeat(ctx context.Context) {
	metrics := a.vmManager.CollectMetrics()

	// TODO: Send heartbeat to scheduler via mTLS
	// POST /heartbeat { host_id, metrics, active_vms }
	_ = ctx
	_ = metrics
	log.Debug().Str("host_id", a.cfg.HostID).Msg("heartbeat sent")
}
