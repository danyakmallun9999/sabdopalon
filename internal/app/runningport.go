package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// runningHTTPPort returns the port the running proxy actually listens on.
//
// The configured port is a preference, not a fact: the proxy moves on when
// something already holds it, which is how `sabdopalon link` came to report
// "served at http://site.localhost:8080/" while the browser was answering on port
// 80. Only the running dashboard knows the real answer, so ask it, and fall back
// to the configuration when nothing is running.
func runningHTTPPort(dashboardPort, configured int) int {
	if dashboardPort <= 0 {
		return configured
	}
	client := &http.Client{Timeout: 1200 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/status", dashboardPort))
	if err != nil {
		return configured
	}
	defer func() { _ = resp.Body.Close() }()

	var st struct {
		HTTPPort int `json:"http_port"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil || st.HTTPPort <= 0 {
		return configured
	}
	return st.HTTPPort
}
