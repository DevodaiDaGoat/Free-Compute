package scheduler

import (
	"errors"
	"fmt"
)

// VMRequest describes the resources needed for a VM.
type VMRequest struct {
	UserID    string
	CPUCores  int
	RAMGB     float64
	StorageGB float64
	GPUVramGB float64
}

// Allocation represents a successful host assignment.
type Allocation struct {
	HostID    string
	HostName  string
	UserID    string
	CPUCores  int
	RAMGB     float64
	StorageGB float64
	GPUVramGB float64
}

var (
	ErrNoSuitableHost    = errors.New("no suitable host found")
	ErrInvalidRequest    = errors.New("invalid VM request")
	ErrInsufficientCPU   = errors.New("insufficient CPU on host")
	ErrInsufficientRAM   = errors.New("insufficient RAM on host")
	ErrInsufficientGPU   = errors.New("insufficient GPU VRAM on host")
)

// ValidateRequest checks that a VMRequest has valid resource values.
func ValidateRequest(req VMRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("%w: missing user ID", ErrInvalidRequest)
	}
	if req.CPUCores <= 0 {
		return fmt.Errorf("%w: CPU cores must be positive", ErrInvalidRequest)
	}
	if req.RAMGB <= 0 {
		return fmt.Errorf("%w: RAM must be positive", ErrInvalidRequest)
	}
	if req.StorageGB < 0 {
		return fmt.Errorf("%w: storage cannot be negative", ErrInvalidRequest)
	}
	if req.GPUVramGB < 0 {
		return fmt.Errorf("%w: GPU VRAM cannot be negative", ErrInvalidRequest)
	}
	if req.CPUCores > 64 {
		return fmt.Errorf("%w: CPU cores exceeds maximum (64)", ErrInvalidRequest)
	}
	if req.RAMGB > 512 {
		return fmt.Errorf("%w: RAM exceeds maximum (512 GB)", ErrInvalidRequest)
	}
	return nil
}

// CanFit checks whether a host has enough free resources for a request.
func CanFit(h Host, req VMRequest) error {
	freeCPU := float64(h.CPUCores) * (1.0 - h.CPUUsage)
	if freeCPU < float64(req.CPUCores) {
		return ErrInsufficientCPU
	}
	freeRAM := h.RAMGB * (1.0 - h.RAMUsage)
	if freeRAM < req.RAMGB {
		return ErrInsufficientRAM
	}
	if req.GPUVramGB > 0 {
		freeGPU := h.GPUVramGB * (1.0 - h.GPUUsage)
		if freeGPU < req.GPUVramGB {
			return ErrInsufficientGPU
		}
	}
	return nil
}

// Allocate picks the best host for a VM request from a ranked list.
func Allocate(ranked []HostScore, req VMRequest) (Allocation, error) {
	if err := ValidateRequest(req); err != nil {
		return Allocation{}, err
	}
	for _, hs := range ranked {
		if err := CanFit(hs.Host, req); err == nil {
			return Allocation{
				HostID:    hs.Host.ID,
				HostName:  hs.Host.Name,
				UserID:    req.UserID,
				CPUCores:  req.CPUCores,
				RAMGB:     req.RAMGB,
				StorageGB: req.StorageGB,
				GPUVramGB: req.GPUVramGB,
			}, nil
		}
	}
	return Allocation{}, ErrNoSuitableHost
}
