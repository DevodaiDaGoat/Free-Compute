package monitoring

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"
)

type SystemStats struct {
	Goroutines   int     `json:"goroutines"`
	GoVersion    string  `json:"goVersion"`
	AllocMB      float64 `json:"allocMb"`
	TotalAllocMB float64 `json:"totalAllocMb"`
	SysMB        float64 `json:"sysMb"`
	NumGC        uint32  `json:"numGc"`
	CPUSeconds   float64 `json:"cpuSeconds"`
}

type Collector struct {
	metrics       *Metrics
	healthChecker *HealthChecker
	logger        *log.Logger
	interval      time.Duration
}

func NewCollector(metrics *Metrics, healthChecker *HealthChecker, logger *log.Logger, interval time.Duration) *Collector {
	if logger == nil {
		logger = log.Default()
	}
	if interval < time.Second {
		interval = 10 * time.Second
	}
	return &Collector{
		metrics:       metrics,
		healthChecker: healthChecker,
		logger:        logger,
		interval:      interval,
	}
}

func (c *Collector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Printf("metrics collector started (interval: %v)", c.interval)

	for {
		select {
		case <-ctx.Done():
			c.logger.Print("metrics collector stopped")
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

func (c *Collector) collect() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	stats := &SystemStats{
		Goroutines:   runtime.NumGoroutine(),
		GoVersion:    runtime.Version(),
		AllocMB:      float64(mem.Alloc) / 1024 / 1024,
		TotalAllocMB: float64(mem.TotalAlloc) / 1024 / 1024,
		SysMB:        float64(mem.Sys) / 1024 / 1024,
		NumGC:        mem.NumGC,
	}

	c.healthChecker.ReportHealth("system", HealthOK,
		fmt.Sprintf("%d goroutines, %.1f MB allocated", stats.Goroutines, stats.AllocMB),
		time.Duration(0))

	c.healthChecker.ReportHealth("runtime", HealthOK,
		fmt.Sprintf("Go %s, %d GC cycles", stats.GoVersion, stats.NumGC),
		time.Duration(0))

	_ = stats
}

func (c *Collector) CollectSystemStats() *SystemStats {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return &SystemStats{
		Goroutines:   runtime.NumGoroutine(),
		GoVersion:    runtime.Version(),
		AllocMB:      float64(mem.Alloc) / 1024 / 1024,
		TotalAllocMB: float64(mem.TotalAlloc) / 1024 / 1024,
		SysMB:        float64(mem.Sys) / 1024 / 1024,
		NumGC:        mem.NumGC,
	}
}
