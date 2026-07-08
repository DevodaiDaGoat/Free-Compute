package scheduler

import (
	"sync"
	"time"
)

type ResourceAllocator struct {
	mu       sync.Mutex
	capacity ResourceCapacity
	used     ResourceUsage
}

type ResourceCapacity struct {
	TotalCPUCores int
	TotalRAMGB    int
	TotalGPUVram  int
}

type ResourceUsage struct {
	UsedCPUCores int
	UsedRAMGB    int
	UsedGPUVram  int
}

func NewResourceAllocator(capacity ResourceCapacity) *ResourceAllocator {
	return &ResourceAllocator{
		capacity: capacity,
	}
}

func (a *ResourceAllocator) CanAllocate(cpu, ram, gpuVram int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.used.UsedCPUCores+cpu <= a.capacity.TotalCPUCores &&
		a.used.UsedRAMGB+ram <= a.capacity.TotalRAMGB &&
		a.used.UsedGPUVram+gpuVram <= a.capacity.TotalGPUVram
}

func (a *ResourceAllocator) Allocate(cpu, ram, gpuVram int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !(a.used.UsedCPUCores+cpu <= a.capacity.TotalCPUCores &&
		a.used.UsedRAMGB+ram <= a.capacity.TotalRAMGB &&
		a.used.UsedGPUVram+gpuVram <= a.capacity.TotalGPUVram) {
		return false
	}

	a.used.UsedCPUCores += cpu
	a.used.UsedRAMGB += ram
	a.used.UsedGPUVram += gpuVram
	return true
}

func (a *ResourceAllocator) Release(cpu, ram, gpuVram int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.used.UsedCPUCores = max(0, a.used.UsedCPUCores-cpu)
	a.used.UsedRAMGB = max(0, a.used.UsedRAMGB-ram)
	a.used.UsedGPUVram = max(0, a.used.UsedGPUVram-gpuVram)
}

func (a *ResourceAllocator) Usage() ResourceUsage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.used
}

func (a *ResourceAllocator) Available() (cpu, ram, gpuVram int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.capacity.TotalCPUCores - a.used.UsedCPUCores,
		a.capacity.TotalRAMGB - a.used.UsedRAMGB,
		a.capacity.TotalGPUVram - a.used.UsedGPUVram
}

type AllocationLease struct {
	ID        string
	UserID    string
	HostID    string
	CPUCores  int
	RAMGB     int
	GPUVramGB int
	CreatedAt time.Time
	ExpiresAt time.Time
}

type LeaseManager struct {
	mu     sync.Mutex
	leases map[string]*AllocationLease
}

func NewLeaseManager() *LeaseManager {
	return &LeaseManager{
		leases: make(map[string]*AllocationLease),
	}
}

func (lm *LeaseManager) Create(lease *AllocationLease) {
	lm.mu.Lock()
	lm.leases[lease.ID] = lease
	lm.mu.Unlock()
}

func (lm *LeaseManager) Revoke(id string) {
	lm.mu.Lock()
	delete(lm.leases, id)
	lm.mu.Unlock()
}

func (lm *LeaseManager) Get(id string) *AllocationLease {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	return lm.leases[id]
}

func (lm *LeaseManager) ListByHost(hostID string) []*AllocationLease {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	var result []*AllocationLease
	for _, l := range lm.leases {
		if l.HostID == hostID {
			result = append(result, l)
		}
	}
	return result
}

func (lm *LeaseManager) CleanupExpired() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	now := time.Now()
	for id, lease := range lm.leases {
		if now.After(lease.ExpiresAt) {
			delete(lm.leases, id)
		}
	}
}
