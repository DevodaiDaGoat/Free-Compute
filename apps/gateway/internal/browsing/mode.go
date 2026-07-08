package browsing

import (
	"encoding/json"
	"net/http"
	"strings"
)

type BrowsingMode string

const (
	ModeSpeed    BrowsingMode = "speed"     // Max speed, minimal overhead, no filtering
	ModePrivacy  BrowsingMode = "privacy"   // Header stripping, tracker blocking, no logging
	ModeCasual   BrowsingMode = "casual"    // Chrome-like default, safe search, moderate filtering
)

type BrowsingPolicy struct {
	Mode BrowsingMode

	// Performance
	DisableCompression bool
	DisableCache       bool
	AggressiveCache    bool
	MaxHeaderBytes     int

	// Privacy
	StripForwardedHeaders bool
	StripUserAgent        bool
	BlockTrackers         bool
	BlockAds              bool
	EnableDNT             bool
	MinimalLogging        bool

	// Security
	EnforceSafeSearch     bool
	BlockMalware          bool
	BlockCryptoMiners     bool
	BlockIllegalActivity  bool
	MaxConnsPerUser       int
	RateLimitRPM          int
}

func DefaultPolicy(mode BrowsingMode) BrowsingPolicy {
	switch mode {
	case ModeSpeed:
		return BrowsingPolicy{
			Mode:             ModeSpeed,
			DisableCache:     false,
			AggressiveCache:  true,
			MaxHeaderBytes:   1 << 20,
			StripForwardedHeaders: false,
			BlockTrackers:    false,
			BlockAds:         false,
			EnforceSafeSearch: false,
			BlockMalware:     true,
			BlockCryptoMiners: true,
			MaxConnsPerUser:  500,
			RateLimitRPM:     5000,
		}
	case ModePrivacy:
		return BrowsingPolicy{
			Mode:                  ModePrivacy,
			DisableCompression:    false,
			AggressiveCache:       false,
			MaxHeaderBytes:        1 << 20,
			StripForwardedHeaders: true,
			StripUserAgent:        true,
			BlockTrackers:         true,
			BlockAds:              true,
			EnableDNT:             true,
			MinimalLogging:        true,
			EnforceSafeSearch:     false,
			BlockMalware:          true,
			BlockCryptoMiners:     true,
			BlockIllegalActivity:  true,
			MaxConnsPerUser:       200,
			RateLimitRPM:          2000,
		}
	case ModeCasual:
		return BrowsingPolicy{
			Mode:                  ModeCasual,
			AggressiveCache:       false,
			MaxHeaderBytes:        1 << 20,
			StripForwardedHeaders: false,
			BlockTrackers:         true,
			BlockAds:              true,
			EnableDNT:             false,
			EnforceSafeSearch:     true,
			BlockMalware:          true,
			BlockCryptoMiners:     true,
			BlockIllegalActivity:  true,
			MaxConnsPerUser:       100,
			RateLimitRPM:          2000,
		}
	default:
		return DefaultPolicy(ModeCasual)
	}
}

// ModeFromPreferences extracts the browsing mode from a user's preferences blob.
// The blob may contain {"browsingMode":"speed|privacy|casual"}. It returns the
// normalized BrowsingMode, or an empty string when the key is absent/invalid so
// callers can fall back to their own defaults.
func ModeFromPreferences(prefs json.RawMessage) BrowsingMode {
	if len(prefs) == 0 {
		return ""
	}
	var parsed struct {
		BrowsingMode string `json:"browsingMode"`
	}
	if err := json.Unmarshal(prefs, &parsed); err != nil {
		return ""
	}
	mode := BrowsingMode(strings.ToLower(strings.TrimSpace(parsed.BrowsingMode)))
	switch mode {
	case ModeSpeed, ModePrivacy, ModeCasual:
		return mode
	default:
		return ""
	}
}

func (p *BrowsingPolicy) ApplyHeaders(req *http.Request) {
	if p == nil {
		return
	}
	if p.StripForwardedHeaders {
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Forwarded-Proto")
		req.Header.Del("X-Real-IP")
	}
	if p.StripUserAgent && req.Header.Get("User-Agent") != "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36")
	}
	if p.EnableDNT {
		req.Header.Set("DNT", "1")
		req.Header.Set("Sec-GPC", "1")
	}
}

func (p *BrowsingPolicy) IsURLAllowed(url string) bool {
	if p == nil {
		return true
	}
	lower := strings.ToLower(url)
	if p.BlockCryptoMiners {
		if strings.Contains(lower, "coinhive") || strings.Contains(lower, "cryptonight") || strings.Contains(lower, "webminer") {
			return false
		}
	}
	if p.BlockMalware {
		if strings.Contains(lower, "malware") || strings.Contains(lower, "exploit") || strings.Contains(lower, "payload") {
			return false
		}
	}
	if p.BlockIllegalActivity {
		if strings.Contains(lower, "darknet") || strings.Contains(lower, "torrent") && strings.Contains(lower, "copyright") {
			return false
		}
	}
	if p.BlockTrackers || p.BlockAds {
		for _, indicator := range trackerAdIndicators {
			if strings.Contains(lower, indicator) {
				return false
			}
		}
	}
	return true
}

// trackerAdIndicators are substrings present in common tracker/advertising
// request URLs. Matching is conservative and host/query based.
var trackerAdIndicators = []string{
	"doubleclick.net",
	"googlesyndication.com",
	"googleadservices.com",
	"adservice.google",
	"pubads.g.doubleclick.net",
	"googletagmanager.com",
	"googletagservices.com",
	"analytics.twitter.com",
	"facebook.com/tr",
	"connect.facebook.net",
	"scorecardresearch.com",
	"hotjar.com",
	"mixpanel.com",
	"segment.io",
	"segment.com",
	"criteo.com",
	"adnxs.com",
	"rubiconproject.com",
	"pubmatic.com",
	"openx.net",
	"taboola.com",
	"outbrain.com",
	"adsystem",
	"/ads/",
	"adsense",
	"tracker",
	"beacon",
	"telemetry",
}

func (p *BrowsingPolicy) ModifySearchURL(url string) string {
	if p == nil || !p.EnforceSafeSearch {
		return url
	}
	lower := strings.ToLower(url)
	if strings.Contains(lower, "q=") {
		if strings.Contains(url, "?") {
			return url + "&safe=active"
		}
		return url + "?safe=active"
	}
	return url
}
