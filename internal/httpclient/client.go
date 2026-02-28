package httpclient

import (
	"net/http"
	"time"
)

// Shared HTTP client with timeout and connection reuse.
var Default = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}
