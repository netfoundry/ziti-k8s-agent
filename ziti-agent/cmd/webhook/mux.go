package webhook

import (
	"net/http"

	"golang.org/x/time/rate"
)

// Rate limit -  1 token to be consumed per second, with a maximum burst size of 3.
var rateLimiter = rate.NewLimiter(rate.Limit(1), 5)

// CustomServeMux implements a custom ServeMux with a rate limiter and channel queue
type customMux struct {
	*http.ServeMux
	// queue chan *http.Request
	// wg    sync.WaitGroup
}

func NewCustomMux() *customMux {
	return &customMux{
		http.NewServeMux(),
		// make(chan *http.Request, 10),
		// sync.WaitGroup{},
	}
}

func (m *customMux) Handle(pattern string, handler http.HandlerFunc) {
	m.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		// Check rate limit
		if rateLimiter.Allow() == false {
			http.Error(w, http.StatusText(429), http.StatusTooManyRequests)
			return
		}

		// m.queue <- r
		// m.wg.Add(1)

		// defer m.wg.Done()

		// for {
		// 	select {
		// 	case req := <-m.queue:
		// 		m.ServeMux.ServeHTTP(w, req)
		// 	case <-context.Background().Done():
		// 		return
		// 	}
		// }

		// Serve next request
		m.ServeMux.ServeHTTP(w, r)
	})
}

// func (m *customMux) Shutdown(ctx context.Context) error {
// 	close(m.queue)
// 	m.wg.Wait()
// 	return nil
// }
