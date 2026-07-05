package scheduler

import (
	"errors"
	"testing"
	"time"
)

func validRequest() VMRequest {
	return VMRequest{
		UserID:    "user-1",
		CPUCores:  4,
		RAMGB:     8,
		StorageGB: 50,
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     VMRequest
		wantErr bool
		errType error
	}{
		{"valid basic", validRequest(), false, nil},
		{"valid with GPU", VMRequest{UserID: "u1", CPUCores: 4, RAMGB: 16, GPUVramGB: 8}, false, nil},
		{"missing user ID", VMRequest{CPUCores: 4, RAMGB: 8}, true, ErrInvalidRequest},
		{"zero CPU", VMRequest{UserID: "u1", CPUCores: 0, RAMGB: 8}, true, ErrInvalidRequest},
		{"negative CPU", VMRequest{UserID: "u1", CPUCores: -1, RAMGB: 8}, true, ErrInvalidRequest},
		{"zero RAM", VMRequest{UserID: "u1", CPUCores: 4, RAMGB: 0}, true, ErrInvalidRequest},
		{"negative storage", VMRequest{UserID: "u1", CPUCores: 4, RAMGB: 8, StorageGB: -10}, true, ErrInvalidRequest},
		{"negative GPU", VMRequest{UserID: "u1", CPUCores: 4, RAMGB: 8, GPUVramGB: -1}, true, ErrInvalidRequest},
		{"excessive CPU", VMRequest{UserID: "u1", CPUCores: 128, RAMGB: 8}, true, ErrInvalidRequest},
		{"excessive RAM", VMRequest{UserID: "u1", CPUCores: 4, RAMGB: 1024}, true, ErrInvalidRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("expected error wrapping %v, got %v", tt.errType, err)
			}
		})
	}
}

func TestCanFit(t *testing.T) {
	bigHost := Host{
		CPUCores: 32, RAMGB: 128, GPUVramGB: 24,
		CPUUsage: 0.1, RAMUsage: 0.1, GPUUsage: 0.1,
	}
	tinyHost := Host{
		CPUCores: 2, RAMGB: 4,
		CPUUsage: 0.5, RAMUsage: 0.5,
	}

	tests := []struct {
		name    string
		host    Host
		req     VMRequest
		wantErr error
	}{
		{"fits on big host", bigHost, VMRequest{CPUCores: 4, RAMGB: 8}, nil},
		{"fits GPU request", bigHost, VMRequest{CPUCores: 4, RAMGB: 8, GPUVramGB: 8}, nil},
		{"CPU too high", tinyHost, VMRequest{CPUCores: 4, RAMGB: 1}, ErrInsufficientCPU},
		{"RAM too high", tinyHost, VMRequest{CPUCores: 1, RAMGB: 4}, ErrInsufficientRAM},
		{"GPU not available", tinyHost, VMRequest{CPUCores: 1, RAMGB: 1, GPUVramGB: 4}, ErrInsufficientGPU},
		{"zero GPU request on non-GPU host", tinyHost, VMRequest{CPUCores: 1, RAMGB: 1, GPUVramGB: 0}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CanFit(tt.host, tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CanFit() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAllocate_Success(t *testing.T) {
	now := time.Now()
	ranked := []HostScore{
		{Host: Host{ID: "h1", Name: "host-1", CPUCores: 32, RAMGB: 128, Online: true, LastHeartbeat: now}, Score: 0.9},
		{Host: Host{ID: "h2", Name: "host-2", CPUCores: 16, RAMGB: 64, Online: true, LastHeartbeat: now}, Score: 0.7},
	}
	req := validRequest()
	alloc, err := Allocate(ranked, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alloc.HostID != "h1" {
		t.Errorf("allocated to %s, want h1", alloc.HostID)
	}
	if alloc.UserID != req.UserID {
		t.Errorf("user = %s, want %s", alloc.UserID, req.UserID)
	}
	if alloc.CPUCores != req.CPUCores {
		t.Errorf("CPU = %d, want %d", alloc.CPUCores, req.CPUCores)
	}
}

func TestAllocate_FallsBackToSecondHost(t *testing.T) {
	ranked := []HostScore{
		{Host: Host{ID: "h1", CPUCores: 2, RAMGB: 4, CPUUsage: 0.9, RAMUsage: 0.9}, Score: 0.9},
		{Host: Host{ID: "h2", CPUCores: 32, RAMGB: 128}, Score: 0.5},
	}
	req := VMRequest{UserID: "u1", CPUCores: 8, RAMGB: 32}
	alloc, err := Allocate(ranked, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alloc.HostID != "h2" {
		t.Errorf("should fall back to h2, got %s", alloc.HostID)
	}
}

func TestAllocate_NoSuitableHost(t *testing.T) {
	ranked := []HostScore{
		{Host: Host{ID: "h1", CPUCores: 2, RAMGB: 4}, Score: 0.5},
	}
	req := VMRequest{UserID: "u1", CPUCores: 16, RAMGB: 64}
	_, err := Allocate(ranked, req)
	if !errors.Is(err, ErrNoSuitableHost) {
		t.Errorf("expected ErrNoSuitableHost, got %v", err)
	}
}

func TestAllocate_EmptyRanked(t *testing.T) {
	req := validRequest()
	_, err := Allocate(nil, req)
	if !errors.Is(err, ErrNoSuitableHost) {
		t.Errorf("expected ErrNoSuitableHost for empty list, got %v", err)
	}
}

func TestAllocate_InvalidRequest(t *testing.T) {
	ranked := []HostScore{
		{Host: Host{ID: "h1", CPUCores: 32, RAMGB: 128}, Score: 0.9},
	}
	req := VMRequest{CPUCores: 4, RAMGB: 8} // missing UserID
	_, err := Allocate(ranked, req)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("expected ErrInvalidRequest, got %v", err)
	}
}
