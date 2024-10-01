package webhook

import (
	"net/http"

	"golang.org/x/time/rate"
)

// Rate limit -  5 token to be consumed per second, with a maximum burst size of 15.
var rateLimiter = rate.NewLimiter(rate.Limit(15), 1)

// CustomServeMux implements a custom ServeMux with a rate limiter and channel queue
type customMux struct {
	*http.ServeMux
}

func NewCustomMux() *customMux {
	return &customMux{
		http.NewServeMux(),
	}
}

func (m *customMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check rate limit
	if rateLimiter.Allow() == false {
		http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
		return
	}

	// Serve next request
	m.ServeMux.ServeHTTP(w, r)
}
