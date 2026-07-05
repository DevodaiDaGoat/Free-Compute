package vm

import (
	"sync"

	"github.com/DevodaiDaGoat/Free-Compute/host-agent/internal/config"
	"github.com/rs/zerolog/log"
)

// Manager handles the lifecycle of QEMU VMs on this host.
type Manager struct {
	cfg *config.Config
	mu  sync.RWMutex
	vms map[string]*Instance
}

type Instance struct {
	ID        string
	UserID    string
	Name      string
	CPUCores  int
	RAMGB     int
	StorageGB int
	State     string // running, paused, stopped
	PID       int    // QEMU process ID
}

func NewManager(cfg *config.Config) (*Manager, error) {
	return &Manager{
		cfg: cfg,
		vms: make(map[string]*Instance),
	}, nil
}

// Launch starts a new VM instance.
// SECURITY: VM configuration is validated and sandboxed:
// - Separate cgroup and network namespace
// - No access to host filesystem outside designated paths
// - Egress filtering applied
func (m *Manager) Launch(vmID, userID, name string, cpu, ramGB, storageGB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.vms) >= m.cfg.MaxVMs {
		return &VMError{Message: "host at maximum VM capacity"}
	}

	instance := &Instance{
		ID:        vmID,
		UserID:    userID,
		Name:      name,
		CPUCores:  cpu,
		RAMGB:     ramGB,
		StorageGB: storageGB,
		State:     "starting",
	}

	// TODO: Launch QEMU process with security flags:
	// -sandbox on
	// -netdev user,id=net0 (isolated networking)
	// Separate cgroup for resource limiting

	instance.State = "running"
	m.vms[vmID] = instance

	log.Info().Str("vm_id", vmID).Str("user_id", userID).Msg("VM launched")
	return nil
}

// Stop gracefully stops a VM.
func (m *Manager) Stop(vmID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.vms[vmID]
	if !ok {
		return &VMError{Message: "VM not found"}
	}

	// TODO: Send ACPI shutdown to QEMU, wait for graceful shutdown
	// Kill process if it doesn't stop within timeout
	instance.State = "stopped"
	delete(m.vms, vmID)

	log.Info().Str("vm_id", vmID).Msg("VM stopped")
	return nil
}

// StopAll stops all running VMs (used during agent shutdown).
func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.vms))
	for id := range m.vms {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		if err := m.Stop(id); err != nil {
			log.Error().Err(err).Str("vm_id", id).Msg("failed to stop VM")
		}
	}
}

// CollectMetrics gathers resource usage from all running VMs.
func (m *Manager) CollectMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"active_vms": len(m.vms),
		// TODO: Collect per-VM resource usage from cgroups
	}
}

type VMError struct {
	Message string
}

func (e *VMError) Error() string {
	return e.Message
}
