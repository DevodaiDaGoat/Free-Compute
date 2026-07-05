package scheduler

import (
	"context"
	"fmt"
	"log"

	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/database"
	"github.com/DevodaiDaGoat/Free-Compute/apps/scheduler/internal/host"
)

// Galaxy implements the proprietary host ranking and resource allocation algorithm.
// It coordinates the Ranker and Allocator to find the best host for a given request.
type Galaxy struct {
	db        *database.DB
	hostMgr   *host.Manager
	ranker    *Ranker
	allocator *Allocator
}

// NewGalaxy creates a Galaxy scheduler.
func NewGalaxy(db *database.DB, hostMgr *host.Manager) *Galaxy {
	return &Galaxy{
		db:        db,
		hostMgr:   hostMgr,
		ranker:    NewRanker(),
		allocator: NewAllocator(db),
	}
}

// Schedule finds the best available host for the resource request and allocates a VM.
func (g *Galaxy) Schedule(ctx context.Context, req ResourceRequest, userID string) (*Allocation, error) {
	hosts, err := g.hostMgr.OnlineHosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("galaxy: failed to list online hosts: %w", err)
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("galaxy: no online hosts available")
	}

	candidates := g.buildCandidates(ctx, hosts)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("galaxy: no candidates with valid metrics")
	}

	ranked := g.ranker.Rank(candidates, req)

	allocation, err := g.allocator.Allocate(ctx, ranked, req, userID)
	if err != nil {
		return nil, fmt.Errorf("galaxy: %w", err)
	}

	log.Printf("galaxy: scheduled user %s on host %s (score %.3f)",
		userID, allocation.HostID, ranked[0].Score)

	return allocation, nil
}

// buildCandidates pairs each host with its latest metrics.
func (g *Galaxy) buildCandidates(ctx context.Context, hosts []database.Host) []HostScore {
	candidates := make([]HostScore, 0, len(hosts))
	for _, h := range hosts {
		// TODO: fetch real metrics from Redis or the metrics store.
		m := host.Metrics{
			HostID:       h.ID,
			RAMTotalGB:   h.RAMGB,
			GPUVramTotal: h.GPUVramGB,
		}
		candidates = append(candidates, HostScore{Host: h, Metrics: m})
	}
	_ = ctx
	return candidates
}
