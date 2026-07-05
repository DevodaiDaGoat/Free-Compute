package vm

import (
	"testing"
	"time"
)

func validMetrics() VMMetrics {
	return VMMetrics{
		VMID:        "vm-1",
		CPUPercent:  45.0,
		RAMUsedMB:   4096,
		RAMTotalMB:  8192,
		DiskUsedGB:  25,
		DiskTotalGB: 50,
		NetworkRxMB: 100,
		NetworkTxMB: 50,
		Uptime:      2 * time.Hour,
		State:       StateRunning,
	}
}

func TestVMMetrics_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*VMMetrics)
		wantErr bool
	}{
		{"valid", func(m *VMMetrics) {}, false},
		{"empty VMID", func(m *VMMetrics) { m.VMID = "" }, true},
		{"CPU negative", func(m *VMMetrics) { m.CPUPercent = -1 }, true},
		{"CPU over 100", func(m *VMMetrics) { m.CPUPercent = 101 }, true},
		{"CPU at 0", func(m *VMMetrics) { m.CPUPercent = 0 }, false},
		{"CPU at 100", func(m *VMMetrics) { m.CPUPercent = 100 }, false},
		{"RAM used negative", func(m *VMMetrics) { m.RAMUsedMB = -1 }, true},
		{"RAM total negative", func(m *VMMetrics) { m.RAMTotalMB = -1 }, true},
		{"RAM used > total", func(m *VMMetrics) { m.RAMUsedMB = 9000; m.RAMTotalMB = 8000 }, true},
		{"disk negative", func(m *VMMetrics) { m.DiskUsedGB = -1 }, true},
		{"disk used > total", func(m *VMMetrics) { m.DiskUsedGB = 100; m.DiskTotalGB = 50 }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMetrics()
			tt.modify(&m)
			err := m.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVMMetrics_RAMPercent(t *testing.T) {
	tests := []struct {
		name  string
		used  float64
		total float64
		want  float64
	}{
		{"half used", 4096, 8192, 50.0},
		{"empty", 0, 8192, 0.0},
		{"full", 8192, 8192, 100.0},
		{"zero total", 0, 0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := VMMetrics{RAMUsedMB: tt.used, RAMTotalMB: tt.total}
			got := m.RAMPercent()
			if got != tt.want {
				t.Errorf("RAMPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVMMetrics_DiskPercent(t *testing.T) {
	tests := []struct {
		name  string
		used  float64
		total float64
		want  float64
	}{
		{"half used", 25, 50, 50.0},
		{"empty", 0, 100, 0.0},
		{"zero total", 0, 0, 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := VMMetrics{DiskUsedGB: tt.used, DiskTotalGB: tt.total}
			got := m.DiskPercent()
			if got != tt.want {
				t.Errorf("DiskPercent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVMMetrics_IsHealthy(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*VMMetrics)
		healthy bool
	}{
		{"healthy VM", func(m *VMMetrics) {}, true},
		{"not running", func(m *VMMetrics) { m.State = StatePaused }, false},
		{"high CPU", func(m *VMMetrics) { m.CPUPercent = 95 }, false},
		{"high RAM", func(m *VMMetrics) { m.RAMUsedMB = 7800; m.RAMTotalMB = 8192 }, false},
		{"high disk", func(m *VMMetrics) { m.DiskUsedGB = 48; m.DiskTotalGB = 50 }, false},
		{"above thresholds", func(m *VMMetrics) { m.CPUPercent = 91; m.RAMUsedMB = 7500; m.RAMTotalMB = 8192 }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validMetrics()
			tt.modify(&m)
			got := m.IsHealthy(90, 90, 90)
			if got != tt.healthy {
				t.Errorf("IsHealthy() = %v, want %v", got, tt.healthy)
			}
		})
	}
}

func TestCheckHealth(t *testing.T) {
	m := validMetrics()
	report := CheckHealth(m, 90, 90, 90)

	if !report.Healthy {
		t.Error("expected healthy report")
	}
	if report.VMID != "vm-1" {
		t.Errorf("VMID = %s, want vm-1", report.VMID)
	}
	if len(report.Issues) != 0 {
		t.Errorf("expected no issues, got %v", report.Issues)
	}
}

func TestCheckHealth_MultipleIssues(t *testing.T) {
	m := VMMetrics{
		VMID:        "vm-bad",
		CPUPercent:  95,
		RAMUsedMB:   7500,
		RAMTotalMB:  8000,
		DiskUsedGB:  95,
		DiskTotalGB: 100,
		State:       StateStopped,
	}
	report := CheckHealth(m, 90, 90, 90)

	if report.Healthy {
		t.Error("expected unhealthy report")
	}
	if len(report.Issues) != 4 {
		t.Errorf("expected 4 issues, got %d: %v", len(report.Issues), report.Issues)
	}
}

func TestCheckHealth_Timestamp(t *testing.T) {
	m := validMetrics()
	before := time.Now()
	report := CheckHealth(m, 90, 90, 90)
	after := time.Now()

	if report.Timestamp.Before(before) || report.Timestamp.After(after) {
		t.Error("timestamp should be approximately now")
	}
}

func TestValidState(t *testing.T) {
	tests := []struct {
		state string
		valid bool
	}{
		{"running", true},
		{"paused", true},
		{"stopped", true},
		{"error", true},
		{"unknown", false},
		{"", false},
		{"RUNNING", false},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			if got := ValidState(tt.state); got != tt.valid {
				t.Errorf("ValidState(%q) = %v, want %v", tt.state, got, tt.valid)
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	tests := []struct {
		from, to VMState
		allowed  bool
	}{
		{StateStopped, StateRunning, true},
		{StateStopped, StatePaused, false},
		{StateRunning, StatePaused, true},
		{StateRunning, StateStopped, true},
		{StateRunning, StateError, true},
		{StateRunning, StateRunning, false},
		{StatePaused, StateRunning, true},
		{StatePaused, StateStopped, true},
		{StatePaused, StateError, false},
		{StateError, StateStopped, true},
		{StateError, StateRunning, false},
	}
	for _, tt := range tests {
		name := string(tt.from) + " -> " + string(tt.to)
		t.Run(name, func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.allowed {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.allowed)
			}
		})
	}
}
