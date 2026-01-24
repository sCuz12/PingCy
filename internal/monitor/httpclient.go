package monitor

import (
	"net"
	"net/http"
	"time"
)

type HTTPClientConfig struct {
	Timeout         time.Duration
	UserAgent       string
	MaxIdleConns    int
	IdleConnTimeout time.Duration
}

// NewHTTPClient returns a reusable HTTP client for all checks.
func NewHTTPClient(cfg HTTPClientConfig) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,

		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second, // TCP connect timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,

		ForceAttemptHTTP2:     true,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: roundTripperWithUA{
			rt:        transport,
			userAgent: cfg.UserAgent,
		},
		Timeout: cfg.Timeout, // hard safety net (per request ctx should still be used)
	}
}

// roundTripperWithUA injects a User-Agent into every request.
type roundTripperWithUA struct {
	rt        http.RoundTripper
	userAgent string
}

func (r roundTripperWithUA) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" && r.userAgent != "" {
		req.Header.Set("User-Agent", r.userAgent)
	}
	return r.rt.RoundTrip(req)
}
