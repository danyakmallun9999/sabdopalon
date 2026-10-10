package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/tunnel"
)

// share is the CLI half of the dashboard's "share publicly" action.
//
// It talks to the running instance over its loopback API instead of launching a
// tunnel of its own. That is not a convenience: the tunnel has to live inside the
// process that owns the proxy it points at, because the proxy is what learns the
// public hostname at runtime. A cloudflared started by the CLI would connect to a
// proxy that has never heard of that hostname, and every visitor would get a 404.
func (a *App) share(args []string) int {
	if !a.Cfg.Dashboard.Enabled {
		fmt.Println("  ✗ the dashboard is disabled in config/engine.toml — there is no running instance to share through")
		return 1
	}
	if len(args) == 0 {
		fmt.Println("usage: sabdopalon share <site> [--email a@b.com] [--email c@d.com]")
		fmt.Println("       sabdopalon share stop <site>")
		fmt.Println("       sabdopalon share list")
		return 2
	}

	switch args[0] {
	case "list":
		return a.shareList()
	case "stop":
		if len(args) < 2 {
			fmt.Println("  ✗ usage: sabdopalon share stop <site>")
			return 2
		}
		return a.shareStop(args[1])
	}

	site := args[0]
	var mail []string
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--email" || args[i] == "-e":
			if i+1 >= len(args) {
				fmt.Println("  ✗ --email needs an address")
				return 2
			}
			i++
			mail = append(mail, splitMail(args[i])...)
		case strings.Contains(args[i], "@"):
			mail = append(mail, splitMail(args[i])...)
		default:
			fmt.Printf("  ✗ unknown argument %q\n", args[i])
			return 2
		}
	}
	return a.shareStart(site, mail)
}

func (a *App) shareStart(site string, mail []string) int {
	data, err := a.dashDo(http.MethodPost, "/api/sites/"+site+"/tunnel", map[string]any{"allowed_mail": mail})
	if err != nil {
		fmt.Printf("  ✗ %v\n", err)
		return 1
	}
	var apiErr struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	_ = json.Unmarshal(data, &apiErr)
	if apiErr.Error != "" {
		fmt.Printf("  ✗ %s\n", apiErr.Error)
		if apiErr.Hint != "" {
			fmt.Printf("    install it with: %s\n", apiErr.Hint)
		}
		return 1
	}

	fmt.Printf("  … asking Cloudflare for a public URL for %s\n", site)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		st, err := a.tunnelState(site)
		if err == nil {
			switch st.Status {
			case tunnel.StatusOnline:
				fmt.Printf("\n  🔗 %s\n\n", st.URL)
				if len(st.AllowedMail) > 0 {
					fmt.Printf("     only these emails can open it: %s\n", strings.Join(st.AllowedMail, ", "))
				}
				fmt.Println("     Anyone with the link can reach this copy of the site.")
				fmt.Println("     The URL is new on every share, and it stops working when Sabdopalon stops.")
				return 0
			case tunnel.StatusError:
				fmt.Printf("  ✗ %s\n", st.Error)
				fmt.Printf("    log: logs/tunnel-%s.log\n", site)
				return 1
			}
		}
		time.Sleep(700 * time.Millisecond)
	}
	fmt.Printf("  ✗ timed out waiting for the public URL — see logs/tunnel-%s.log\n", site)
	return 1
}

func (a *App) shareStop(site string) int {
	data, err := a.dashDo(http.MethodDelete, "/api/sites/"+site+"/tunnel", nil)
	if err != nil {
		fmt.Printf("  ✗ %v\n", err)
		return 1
	}
	var apiErr struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(data, &apiErr)
	if apiErr.Error != "" {
		fmt.Printf("  ✗ %s\n", apiErr.Error)
		return 1
	}
	fmt.Printf("  ✓ stopped sharing %s\n", site)
	return 0
}

func (a *App) shareList() int {
	data, err := a.dashDo(http.MethodGet, "/api/status", nil)
	if err != nil {
		fmt.Printf("  ✗ %v\n", err)
		return 1
	}
	var snap struct {
		Tunnels map[string]tunnel.State `json:"tunnels"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		fmt.Printf("  ✗ unexpected response from %s\n", a.dashBase())
		return 1
	}
	names := make([]string, 0, len(snap.Tunnels))
	for name, st := range snap.Tunnels {
		if st.Status == tunnel.StatusOff {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		fmt.Println("  no site is shared right now")
		return 0
	}
	sort.Strings(names)
	for _, name := range names {
		st := snap.Tunnels[name]
		url := st.URL
		if url == "" {
			url = "(" + string(st.Status) + ")"
		}
		fmt.Printf("  %-24s %s\n", name, url)
		if st.Error != "" {
			fmt.Printf("  %-24s %s\n", "", st.Error)
		}
	}
	return 0
}

func (a *App) tunnelState(site string) (tunnel.State, error) {
	data, err := a.dashDo(http.MethodGet, "/api/sites/"+site+"/tunnel", nil)
	if err != nil {
		return tunnel.State{}, err
	}
	var st tunnel.State
	if err := json.Unmarshal(data, &st); err != nil {
		return tunnel.State{}, err
	}
	return st, nil
}

func (a *App) dashBase() string {
	return fmt.Sprintf("http://127.0.0.1:%d", a.Cfg.Dashboard.Port)
}

// dashDo calls the local dashboard API.
//
// A refused connection is reported as "is it running?" rather than as a raw dial
// error: talking to a stopped instance is the most likely way this command fails,
// and the fix is a command the user knows.
func (a *App) dashDo(method, path string, body any) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, a.dashBase()+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Sabdopalon on %s — is it running?", a.dashBase())
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// splitMail accepts "a@b.com" or "a@b.com,c@d.com" so both --email a,b and a
// single quoted list work.
func splitMail(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
