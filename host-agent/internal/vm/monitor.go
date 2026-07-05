package vm

import (
	"errors"
	"time"
)

// VMState represents the lifecycle state of a virtual machine.
type VMState string

const (
	StateRunning VMState = "running"
	StatePaused  VMState = "paused"
	StateStopped VMState = "stopped"
	StateError   VMState = "error"
)

// VMMetrics holds resource usage metrics for a VM.
type VMMetrics struct {
	VMID       string
	CPUPercent float64
	RAMUsedMB  float64
	RAMTotalMB float64
	DiskUsedGB float64
	DiskTotalGB float64
	NetworkRxMB float64
	NetworkTxMB float64
	Uptime     time.Duration
	State      VMState
}

var (
	ErrInvalidMetrics = errors.New("invalid metrics")
)

// Validate checks that metrics contain reasonable values.
func (m *VMMetrics) Validate() error {
	if m.VMID == "" {
		return errors.New("VM ID is required")
	}
	if m.CPUPercent < 0 || m.CPUPercent > 100 {
		return errors.New("CPU percent must be between 0 and 100")
	}
	if m.RAMUsedMB < 0 {
		return errors.New("RAM used cannot be negative")
	}
	if m.RAMTotalMB < 0 {
		return errors.New("RAM total cannot be negative")
	}
	if m.RAMUsedMB > m.RAMTotalMB {
		return errors.New("RAM used cannot exceed total")
	}
	if m.DiskUsedGB < 0 || m.DiskTotalGB < 0 {
		return errors.New("disk values cannot be negative")
	}
	if m.DiskUsedGB > m.DiskTotalGB {
		return errors.New("disk used cannot exceed total")
	}
	return nil
}

// RAMPercent returns RAM usage as a percentage.
func (m *VMMetrics) RAMPercent() float64 {
	if m.RAMTotalMB == 0 {
		return 0
	}
	return (m.RAMUsedMB / m.RAMTotalMB) * 100
}

// DiskPercent returns disk usage as a percentage.
func (m *VMMetrics) DiskPercent() float64 {
	if m.DiskTotalGB == 0 {
		return 0
	}
	return (m.DiskUsedGB / m.DiskTotalGB) * 100
}

// IsHealthy returns true if the VM is running and within resource thresholds.
func (m *VMMetrics) IsHealthy(cpuThreshold, ramThreshold, diskThreshold float64) bool {
	if m.State != StateRunning {
		return false
	}
	if m.CPUPercent > cpuThreshold {
		return false
	}
	if m.RAMPercent() > ramThreshold {
		return false
	}
	if m.DiskPercent() > diskThreshold {
		return false
	}
	return true
}

// HealthReport summarizes the health of a VM.
type HealthReport struct {
	VMID      string
	Healthy   bool
	Issues    []string
	Timestamp time.Time
}

// CheckHealth generates a detailed health report for a VM.
func CheckHealth(m VMMetrics, cpuThreshold, ramThreshold, diskThreshold float64) HealthReport {
	report := HealthReport{
		VMID:      m.VMID,
		Healthy:   true,
		Timestamp: time.Now(),
	}

	if m.State != StateRunning {
		report.Healthy = false
		report.Issues = append(report.Issues, "VM is not running (state: "+string(m.State)+")")
	}

	if m.CPUPercent > cpuThreshold {
		report.Healthy = false
		report.Issues = append(report.Issues, "CPU usage above threshold")
	}

	if m.RAMPercent() > ramThreshold {
		report.Healthy = false
		report.Issues = append(report.Issues, "RAM usage above threshold")
	}

	if m.DiskPercent() > diskThreshold {
		report.Healthy = false
		report.Issues = append(report.Issues, "disk usage above threshold")
	}

	return report
}

// ValidState checks if a state string is a valid VMState.
func ValidState(s string) bool {
	switch VMState(s) {
	case StateRunning, StatePaused, StateStopped, StateError:
		return true
	}
	return false
}

// CanTransition checks if transitioning from one state to another is allowed.
func CanTransition(from, to VMState) bool {
	switch from {
	case StateStopped:
		return to == StateRunning
	case StateRunning:
		return to == StatePaused || to == StateStopped || to == StateError
	case StatePaused:
		return to == StateRunning || to == StateStopped
	case StateError:
		return to == StateStopped
	}
	return false
}
