package scheduler

import (
	"context"
	"fmt"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
)

// Allocation is the result of a successful resource allocation.
type Allocation struct {
	HostID string
	VM     database.VM
}

// Allocator assigns VMs to hosts based on ranked candidates.
type Allocator struct {
	db *database.DB
}

// NewAllocator creates an Allocator.
func NewAllocator(db *database.DB) *Allocator {
	return &Allocator{db: db}
}

// Allocate picks the best host from ranked candidates and creates a VM placement.
func (a *Allocator) Allocate(ctx context.Context, ranked []HostScore, req ResourceRequest, userID string) (*Allocation, error) {
	if len(ranked) == 0 {
		return nil, fmt.Errorf("allocator: no candidate hosts available")
	}

	best := ranked[0]
	if best.Score <= 0 {
		return nil, fmt.Errorf("allocator: no host satisfies the resource request")
	}

	vm := database.VM{
		UserID:    userID,
		HostID:    best.Host.ID,
		State:     "running",
		CPUCores:  req.CPUCores,
		RAMGB:     float64(req.RAMGB),
		StorageGB: float64(req.StorageGB),
	}

	// TODO: persist the VM record to the database.
	_ = ctx

	return &Allocation{
		HostID: best.Host.ID,
		VM:     vm,
	}, nil
}
