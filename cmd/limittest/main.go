// Fires a burst of concurrent requests at the API, to show that the rate
// limiter's map has no lock. Start the server first, then run this:
//
//	go run -race ./cmd/api   (terminal 1)
//	go run ./cmd/limittest   (terminal 2)
//
// The server dies with "fatal error: concurrent map read and map write" from
// limiter.allow. That is not a 500 you can recover from -- the whole API is gone.
package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	baseURL  = "http://localhost:8088"
	path     = "/listings"
	body     = `{"email":"demo@example.com","password":"wrong-on-purpose"}`
	requests = 1000
)

func main() {
	fmt.Printf("POST %s%s  x%d, all at once\n\n", baseURL, path, requests)

	var through, limited, failed atomic.Int64
	var wg sync.WaitGroup

	for i := 1; i <= requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			switch status := send(i); {
			case status == 0:
				failed.Add(1)
			case status == http.StatusTooManyRequests:
				limited.Add(1)
			default:
				through.Add(1)
			}
		}()
	}

	wg.Wait()

	fmt.Printf("  %d through, %d rate limited, %d failed\n", through.Load(), limited.Load(), failed.Load())
	if failed.Load() > 0 {
		fmt.Println("\n  Requests stopped landing -- check the server terminal.")
	}
}

// send fires one request and returns its status, or 0 if it never got a reply.
func send(n int) int {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	// a different IP every time, so the limiter has to insert a new bucket into
	// its map on every single request. Inserts are what the runtime catches.
	req.Header.Set("X-Real-IP", "10:01:01:01")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)

	return res.StatusCode
}
