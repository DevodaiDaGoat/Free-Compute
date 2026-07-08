package tunnel

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
)

type Region string

const (
	RegionUSEast    Region = "us-east"
	RegionUSWest    Region = "us-west"
	RegionEUWest    Region = "eu-west"
	RegionEUCentral Region = "eu-central"
	RegionAPSE      Region = "ap-southeast"
	RegionAPNE      Region = "ap-northeast"
	RegionDefault   Region = "default"
)

type geoMapping struct {
	cidr   *net.IPNet
	region Region
}

type geoConfigJSON struct {
	Mappings []struct {
		CIDR   string `json:"cidr"`
		Region string `json:"region"`
	} `json:"mappings"`
}

type GeoRouter struct {
	mu          sync.RWMutex
	mappings    []geoMapping
	poolByRoute map[string]Region
}

func NewGeoRouter() *GeoRouter {
	gr := &GeoRouter{
		poolByRoute: make(map[string]Region),
	}
	gr.loadMappings()
	return gr
}

func (g *GeoRouter) loadMappings() {
	raw := strings.TrimSpace(os.Getenv("FREECOMPUTE_GEO_REGIONS"))
	if raw == "" {
		g.mappings = nil
		return
	}

	var cfg geoConfigJSON
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.mappings = make([]geoMapping, 0, len(cfg.Mappings))
	for _, m := range cfg.Mappings {
		if _, cidr, err := net.ParseCIDR(m.CIDR); err == nil {
			g.mappings = append(g.mappings, geoMapping{
				cidr:   cidr,
				region: Region(m.Region),
			})
		}
	}

	sort.Slice(g.mappings, func(i, j int) bool {
		onesI, _ := g.mappings[i].cidr.Mask.Size()
		onesJ, _ := g.mappings[j].cidr.Mask.Size()
		return onesI > onesJ
	})
}

func (g *GeoRouter) RegionForIP(ipStr string) Region {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return RegionDefault
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, m := range g.mappings {
		if m.cidr.Contains(ip) {
			return m.region
		}
	}

	return RegionDefault
}

func (g *GeoRouter) RegionForRequest(r *http.Request) Region {
	ip := clientIPFromRemoteAddr(r.RemoteAddr)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		ip = strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		ip = realIP
	}
	return g.RegionForIP(ip)
}

func (g *GeoRouter) SetRouteRegion(routeID string, region Region) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.poolByRoute[routeID] = region
}

func (g *GeoRouter) RegionForRoute(routeID string) Region {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if region, ok := g.poolByRoute[routeID]; ok {
		return region
	}
	return RegionDefault
}

func (g *GeoRouter) RouteCountsByRegion() map[Region]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	counts := make(map[Region]int)
	for _, region := range g.poolByRoute {
		counts[region]++
	}
	return counts
}

func (g *GeoRouter) Reload() {
	g.loadMappings()
}
