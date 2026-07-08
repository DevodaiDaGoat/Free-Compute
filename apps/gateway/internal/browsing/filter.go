package browsing

import (
	"net/url"
	"strings"
)

var trackerKeywords = []string{
	"google-analytics", "googletagmanager", "facebook.net", "twitter.com",
	"doubleclick", "amazon-adsystem", "adsystem", "adservice", "analytics",
	"tracker", "telemetry", "pixel", "beacon",
}

var adKeywords = []string{
	"doubleclick.net", "googleads", "googlesyndication", "facebook.com/tr",
	"amazon-ads", "adsystem", "adservice", "adclick", "banner",
}

func IsTrackerURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, kw := range trackerKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func IsAdURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, kw := range adKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func SanitizeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Remove common tracking query parameters
	trackingParams := []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content", "gclid", "fbclid"}
	q := parsed.Query()
	for _, param := range trackingParams {
		q.Del(param)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
