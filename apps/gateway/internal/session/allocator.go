package session

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

type HostAllocator struct {
	logger     *log.Logger
	hosts      map[string]*Host
	byRegion   map[string]map[string]*Host
	byClass    map[ResourceClass]map[string]*Host
	byOnline   map[string]*Host
	hostsMutex sync.RWMutex
}

type Host struct {
	ID              string
	Name            string
	Region          string
	CPUcores        int
	RAMGB           int
	GPUVRAMGB       int
	StorageGB       int
	Online          bool
	LastHeartbeat   time.Time
	ResourceClasses []ResourceClass
	GPUs            []GPUInfo
	Network         NetworkInfo
	Capabilities    HostCapabilities
	CurrentLoad     HostLoad
	Mutex           sync.RWMutex
}

type GPUInfo struct {
	Model         string
	Vendor        string
	VRAMGB        float64
	DriverVersion string
	EncoderSupport map[string]string // codec -> 'hardware' | 'software' | 'unsupported'
	MaxConcurrentStreams int
	CurrentStreams int
}

type NetworkInfo struct {
	PublicIPv4      string
	PublicIPv6      string
	Region          string
	UplinkMbps      float64
	DownlinkMbps    float64
	P50LatencyMs    float64
	P95LatencyMs    float64
	SupportsUDP     bool
	SupportsP2P     bool
}

type HostCapabilities struct {
	ResourceClasses    []ResourceClass
	GPUScheduling     bool
	HardwareAcceleration bool
	ControllerPassthrough bool
	AudioForwarding   bool
	FileTransfer      bool
	RemoteSupport     bool
	WebRTC            bool
	TCPProxy          bool
	UDPProxy          bool
	SSHProxy          bool
}

type HostLoad struct {
	CPUUsagePercent    float64
	RAMUsagePercent    float64
	GPUUsagePercent    float64
	GPUVRAMUsedGB      float64
	StorageUsedGB      float64
	ActiveVMs          int
	ActiveStreams      int
	ActiveProxyRoutes  int
	EncoderUsagePercent float64
	NetworkTxMbps      float64
	NetworkRxMbps      float64
	P95LatencyMs       float64
	Timestamp          time.Time
}

type AllocationRequest struct {
	SessionType    SessionType
	ResourceClass  ResourceClass
	Region         string
	GPURequired    bool
	RequestedCPU   int
	RequestedRAM   int
	RequestedGPU   float64
}

type AllocationResponse struct {
	Host      *Host
	Estimated time.Duration
}

func NewHostAllocator(logger *log.Logger) *HostAllocator {
	if logger == nil {
		logger = log.Default()
	}

	return &HostAllocator{
		logger:   logger,
		hosts:    make(map[string]*Host),
		byRegion: make(map[string]map[string]*Host),
		byClass:  make(map[ResourceClass]map[string]*Host),
		byOnline: make(map[string]*Host),
	}
}

func (a *HostAllocator) RegisterHost(host *Host) error {
	a.hostsMutex.Lock()
	defer a.hostsMutex.Unlock()

	if _, exists := a.hosts[host.ID]; exists {
		return errors.New("host already registered")
	}

	a.hosts[host.ID] = host

	if host.Region != "" {
		if a.byRegion[host.Region] == nil {
			a.byRegion[host.Region] = make(map[string]*Host)
		}
		a.byRegion[host.Region][host.ID] = host
	}

	for _, rc := range host.ResourceClasses {
		if a.byClass[rc] == nil {
			a.byClass[rc] = make(map[string]*Host)
		}
		a.byClass[rc][host.ID] = host
	}

	if host.Online {
		a.byOnline[host.ID] = host
	}

	a.logger.Printf("registered host %s (%s, %s)", host.ID, host.Name, host.Region)

	return nil
}

func (a *HostAllocator) UnregisterHost(hostID string) error {
	a.hostsMutex.Lock()
	defer a.hostsMutex.Unlock()

	host, exists := a.hosts[hostID]
	if !exists {
		return errors.New("host not found")
	}

	delete(a.hosts, hostID)

	if host.Region != "" {
		delete(a.byRegion[host.Region], hostID)
		if len(a.byRegion[host.Region]) == 0 {
			delete(a.byRegion, host.Region)
		}
	}

	for _, rc := range host.ResourceClasses {
		delete(a.byClass[rc], hostID)
		if len(a.byClass[rc]) == 0 {
			delete(a.byClass, rc)
		}
	}

	delete(a.byOnline, hostID)

	a.logger.Printf("unregistered host %s", hostID)
	return nil
}

func (a *HostAllocator) UpdateHostLoad(hostID string, load HostLoad) error {
	a.hostsMutex.Lock()
	host, exists := a.hosts[hostID]
	if !exists {
		a.hostsMutex.Unlock()
		return errors.New("host not found")
	}

	wasOnline := host.Online
	host.CurrentLoad = load
	host.LastHeartbeat = time.Now()

	if load.CPUUsagePercent > 0 || load.RAMUsagePercent > 0 || load.ActiveVMs > 0 {
		host.Online = true
	} else {
		host.Online = false
	}

	if host.Online && !wasOnline {
		a.byOnline[host.ID] = host
	} else if !host.Online && wasOnline {
		delete(a.byOnline, host.ID)
	}

	a.hostsMutex.Unlock()
	return nil
}

func (a *HostAllocator) AllocateHost(ctx context.Context, sessionType SessionType, resourceClass ResourceClass, region string, gpuRequired bool) (*Host, error) {
	a.hostsMutex.RLock()
	defer a.hostsMutex.RUnlock()

	regionHosts := a.byOnline
	if region != "" {
		if hosts, ok := a.byRegion[region]; ok {
			regionHosts = hosts
		} else {
			return nil, errors.New("no available hosts in region")
		}
	}

	classHosts := a.byClass[resourceClass]

	candidates := make([]*Host, 0)
	for _, host := range regionHosts {
		if classHosts != nil {
			if _, ok := classHosts[host.ID]; !ok {
				continue
			}
		}

		host.Mutex.RLock()
		if gpuRequired && len(host.GPUs) == 0 {
			host.Mutex.RUnlock()
			continue
		}
		if !a.hasCapacity(host, sessionType) {
			host.Mutex.RUnlock()
			continue
		}
		candidates = append(candidates, host)
		host.Mutex.RUnlock()
	}

	if len(candidates) == 0 {
		return nil, errors.New("no available hosts")
	}

	bestHost := a.selectBestHost(candidates, sessionType, resourceClass, gpuRequired)

	a.logger.Printf("allocated host %s for session type %s", bestHost.ID, sessionType)

	return bestHost, nil
}

func (a *HostAllocator) supportsResourceClass(host *Host, resourceClass ResourceClass) bool {
	for _, rc := range host.ResourceClasses {
		if rc == resourceClass {
			return true
		}
	}
	return false
}

func (a *HostAllocator) hasCapacity(host *Host, sessionType SessionType) bool {
	load := host.CurrentLoad

	// Check CPU capacity
	if load.CPUUsagePercent > 80 {
		return false
	}

	// Check RAM capacity
	if load.RAMUsagePercent > 80 {
		return false
	}

	// Check GPU capacity for gaming sessions
	if sessionType == SessionTypeGaming {
		if load.GPUUsagePercent > 70 {
			return false
		}
		if load.EncoderUsagePercent > 80 {
			return false
		}
	}

	// Check active streams limit
	if load.ActiveStreams >= 10 {
		return false
	}

	return true
}

func (a *HostAllocator) selectBestHost(candidates []*Host, sessionType SessionType, resourceClass ResourceClass, gpuRequired bool) *Host {
	bestHost := candidates[0]
	bestScore := a.scoreHost(bestHost, sessionType, resourceClass, gpuRequired)

	for _, host := range candidates[1:] {
		score := a.scoreHost(host, sessionType, resourceClass, gpuRequired)
		if score > bestScore {
			bestHost = host
			bestScore = score
		}
	}

	return bestHost
}

func (a *HostAllocator) scoreHost(host *Host, sessionType SessionType, resourceClass ResourceClass, gpuRequired bool) float64 {
	score := 0.0
	load := host.CurrentLoad

	// Score based on available resources (inverse of load)
	score += (100 - load.CPUUsagePercent) * 0.3
	score += (100 - load.RAMUsagePercent) * 0.2

	// GPU score for gaming or GPU-required sessions
	if sessionType == SessionTypeGaming || gpuRequired {
		if len(host.GPUs) > 0 {
			score += (100 - load.GPUUsagePercent) * 0.3
			score += (100 - load.EncoderUsagePercent) * 0.2
		}
	}

	// Network quality score
	if host.Network.P95LatencyMs > 0 {
		score += (100 - host.Network.P95LatencyMs) * 0.1
	}

	// Resource class match bonus
	for _, rc := range host.ResourceClasses {
		if rc == resourceClass {
			score += 10
			break
		}
	}

	return score
}

func (a *HostAllocator) GetHost(hostID string) (*Host, error) {
	a.hostsMutex.RLock()
	defer a.hostsMutex.RUnlock()

	host, exists := a.hosts[hostID]
	if !exists {
		return nil, errors.New("host not found")
	}

	return host, nil
}

func (a *HostAllocator) ListHosts() []*Host {
	a.hostsMutex.RLock()
	defer a.hostsMutex.RUnlock()

	hosts := make([]*Host, 0, len(a.hosts))
	for _, host := range a.hosts {
		hosts = append(hosts, host)
	}

	return hosts
}

func (a *HostAllocator) GetAvailableHosts(region string, resourceClass ResourceClass) []*Host {
	a.hostsMutex.RLock()
	defer a.hostsMutex.RUnlock()

	hosts := make([]*Host, 0)
	for _, host := range a.hosts {
		host.Mutex.RLock()

		if !host.Online {
			host.Mutex.RUnlock()
			continue
		}

		if region != "" && host.Region != region {
			host.Mutex.RUnlock()
			continue
		}

		if !a.supportsResourceClass(host, resourceClass) {
			host.Mutex.RUnlock()
			continue
		}

		hosts = append(hosts, host)
		host.Mutex.RUnlock()
	}

	return hosts
}