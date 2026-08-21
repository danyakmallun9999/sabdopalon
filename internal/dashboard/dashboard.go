// Package dashboard implements an interactive web dashboard that lets users
// manage sites, view logs, trigger backups, and monitor running services from
// the browser. It runs on a separate port (default :9900) and provides a JSON
// API + a single-page HTML UI.
package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/backup"
	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/proxy"
)

// Server is the dashboard HTTP server.
type Server struct {
	cfg     *config.Engine
	proxy   *proxy.Server
	backup  *backup.Manager
	mux     *http.ServeMux
	started time.Time
}

// New creates a dashboard Server.
func New(cfg *config.Engine, px *proxy.Server, bk *backup.Manager) *Server {
	s := &Server{
		cfg:     cfg,
		proxy:   px,
		backup:  bk,
		mux:     http.NewServeMux(),
		started: time.Now(),
	}
	s.routes()
	return s
}

// Start launches the dashboard. Blocks.
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.cfg.Dashboard.Port)
	fmt.Printf("  🖥  Dashboard UI: http://localhost:%d/\n", s.cfg.Dashboard.Port)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleUI)
	s.mux.HandleFunc("/api/status", s.handleAPIStatus)
	s.mux.HandleFunc("/api/sites", s.handleAPISites)
	s.mux.HandleFunc("/api/logs/", s.handleAPILogs)
	s.mux.HandleFunc("/api/backup", s.handleAPIBackup)
	s.mux.HandleFunc("/api/backups", s.handleAPIBackups)
}

// --- API: /api/status ---

func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	s.json(w, map[string]any{
		"version":     "0.3.0-dev",
		"uptime":      time.Since(s.started).Round(time.Second).String(),
		"proxy_port":  s.cfg.Proxy.HTTPPort,
		"https_port":  s.cfg.Proxy.HTTPSPort,
		"tld":         s.cfg.TLD,
		"php":         filepath.Base(s.cfg.PHP.Binary),
		"database":    s.cfg.Database.Engine,
		"sites_count": len(s.proxy.RunningSites()),
	})
}

// --- API: /api/sites ---

func (s *Server) handleAPISites(w http.ResponseWriter, r *http.Request) {
	running := s.proxy.RunningSites()
	runningMap := map[string]bool{}
	for _, ri := range running {
		runningMap[ri.Host] = true
	}

	sites, _ := discoverSites(s.cfg)
	result := []map[string]any{}
	for _, name := range sites {
		host := name + "." + s.cfg.TLD
		result = append(result, map[string]any{
			"name":    name,
			"url":     fmt.Sprintf("http://%s:%d/", host, s.cfg.Proxy.HTTPPort),
			"https":   fmt.Sprintf("https://%s:%d/", host, s.cfg.Proxy.HTTPSPort),
			"running": runningMap[host],
		})
	}
	s.json(w, result)
}

// --- API: /api/logs/<sitename> ---

func (s *Server) handleAPILogs(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if name == "" {
		s.json(w, map[string]string{"error": "missing site name"})
		return
	}
	// Try PHP log first, then DB log
	for _, suffix := range []string{name + ".php.log", s.cfg.Database.Engine + ".log"} {
		logPath := filepath.Join(s.cfg.Logs, suffix)
		if data, err := os.ReadFile(logPath); err == nil {
			// Return last 100 lines
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) > 100 {
				lines = lines[len(lines)-100:]
			}
			s.json(w, map[string]any{
				"file":  suffix,
				"lines": lines,
				"count": len(lines),
			})
			return
		}
	}
	s.json(w, map[string]string{"error": "no logs found"})
}

// --- API: /api/backup (POST) ---

func (s *Server) handleAPIBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, map[string]string{"error": "POST required"})
		return
	}
	if s.backup == nil {
		s.json(w, map[string]string{"error": "backup not configured"})
		return
	}
	path, err := s.backup.Backup()
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	pruned, _ := s.backup.Prune()
	s.json(w, map[string]any{
		"backup":  filepath.Base(path),
		"pruned":  pruned,
		"message": fmt.Sprintf("Backup created: %s (pruned %d old)", filepath.Base(path), pruned),
	})
}

// --- API: /api/backups (GET) ---

func (s *Server) handleAPIBackups(w http.ResponseWriter, r *http.Request) {
	if s.backup == nil {
		s.json(w, []any{})
		return
	}
	list, err := s.backup.List()
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	result := []map[string]any{}
	for _, b := range list {
		result = append(result, map[string]any{
			"name": b.Name,
			"size": b.Size,
			"time": b.ModTime.Format(time.RFC3339),
		})
	}
	s.json(w, result)
}

// --- UI: / ---

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

// --- helpers ---

func (s *Server) json(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func discoverSites(cfg *config.Engine) ([]string, error) {
	entries, err := os.ReadDir(cfg.Root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

var _ = io.Discard // keep io imported for future use

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sabdopalon Dashboard</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#c9d1d9;padding:0}
.header{background:#161b22;border-bottom:1px solid #30363d;padding:1rem 2rem;display:flex;align-items:center;justify-content:space-between}
.header h1{color:#58a6ff;font-size:1.3rem}
.header .status{display:flex;gap:1rem;font-size:.85rem;color:#8b949e}
.container{max-width:960px;margin:2rem auto;padding:0 1rem}
.card{background:#161b22;border:1px solid #30363d;border-radius:12px;padding:1.2rem;margin-bottom:1rem}
.card h2{color:#58a6ff;font-size:1rem;margin-bottom:.8rem;display:flex;align-items:center;gap:.5rem}
table{width:100%;border-collapse:collapse}
th,td{text-align:left;padding:.6rem .8rem;border-bottom:1px solid #21262d}
th{color:#8b949e;font-size:.8rem;text-transform:uppercase}
td a{color:#58a6ff;text-decoration:none}
td a:hover{text-decoration:underline}
.badge{font-size:.7rem;padding:.15rem .5rem;border-radius:99px;background:#238636;color:#fff}
.badge.off{background:#30363d;color:#8b949e}
.btn{background:#238636;color:#fff;border:none;padding:.5rem 1rem;border-radius:8px;cursor:pointer;font-size:.85rem}
.btn:hover{background:#2ea043}
.btn:disabled{opacity:.5;cursor:default}
.log-box{background:#010409;border:1px solid #21262d;border-radius:8px;padding:1rem;max-height:400px;overflow-y:auto;font-family:'SF Mono',Consolas,monospace;font-size:.8rem;line-height:1.6;color:#8b949e}
.log-line{white-space:pre-wrap;word-break:break-all}
.site-tabs{display:flex;gap:.5rem;flex-wrap:wrap;margin-bottom:.5rem}
.tab{padding:.3rem .8rem;border:1px solid #30363d;border-radius:99px;cursor:pointer;font-size:.8rem;color:#8b949e}
.tab.active{background:#1f6feb;color:#fff;border-color:#1f6feb}
.empty{color:#8b949e;text-align:center;padding:1rem}
#msg{position:fixed;bottom:2rem;right:2rem;background:#238636;color:#fff;padding:.6rem 1rem;border-radius:8px;display:none;font-size:.85rem}
</style>
</head>
<body>
<div class="header">
<h1>🐫 Sabdopalon</h1>
<div class="status" id="status-bar">Loading...</div>
</div>
<div class="container">

<div class="card">
<h2>📊 Status</h2>
<div id="status-detail">Loading...</div>
</div>

<div class="card">
<h2>🌐 Sites</h2>
<table>
<thead><tr><th>Site</th><th>URL (HTTP)</th><th>URL (HTTPS)</th><th>Status</th></tr></thead>
<tbody id="sites-table"></tbody>
</table>
</div>

<div class="card">
<h2>💾 Backups <button class="btn" onclick="doBackup()">Backup Now</button></h2>
<div id="backups-list"></div>
</div>

<div class="card">
<h2>📋 Logs</h2>
<div class="site-tabs" id="log-tabs"></div>
<div class="log-box" id="log-box">Select a site to view logs</div>
</div>

</div>
<div id="msg"></div>
<script>
async function api(path,opts){const r=await fetch(path,opts);return r.json();}
function showMsg(t){const m=document.getElementById('msg');m.textContent=t;m.style.display='block';setTimeout(()=>m.style.display='none',3000);}

async function loadStatus(){
  const s=await api('/api/status');
  const bar=document.getElementById('status-bar');
  bar.innerHTML='PHP '+s.php+' · DB '+s.database+' · Uptime '+s.uptime+' · '+s.sites_count+' site(s) running';
  const d=document.getElementById('status-detail');
  d.innerHTML='<table>'+
    '<tr><th>Proxy</th><td>http://localhost:'+s.proxy_port+' / https://localhost:'+s.https_port+'</td></tr>'+
    '<tr><th>TLD</th><td>.'+s.tld+'</td></tr>'+
    '<tr><th>PHP</th><td>'+s.php+'</td></tr>'+
    '<tr><th>Database</th><td>'+s.database+'</td></tr>'+
    '</table>';
}
async function loadSites(){
  const sites=await api('/api/sites');
  const tb=document.getElementById('sites-table');
  tb.innerHTML=sites.map(s=>'<tr><td>'+s.name+'</td>'+
    '<td><a href="'+s.url+'">'+s.url.replace('https://','')+'</a></td>'+
    '<td><a href="'+s.https+'">'+s.https.replace('https://','')+'</a></td>'+
    '<td><span class="badge '+(s.running?'':'off')+'">'+(s.running?'running':'stopped')+'</span></td></tr>').join('');
  // Build log tabs
  const tabs=document.getElementById('log-tabs');
  tabs.innerHTML=sites.map((s,i)=>'<span class="tab'+(i===0?' active':'')+'" onclick="loadLog(\''+s.name+'\')">'+s.name+'</span>').join('');
  if(sites.length>0) loadLog(sites[0].name);
}
async function loadLog(name){
  document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
  event&&event.target&&event.target.classList.add('active');
  const data=await api('/api/logs/'+name);
  const box=document.getElementById('log-box');
  if(data.error){box.textContent=data.error;return;}
  box.innerHTML=data.lines.map(l=>'<div class="log-line">'+l.replace(/</g,'&lt;')+'</div>').join('');
  box.scrollTop=box.scrollHeight;
}
async function doBackup(){
  const r=await api('/api/backup',{method:'POST'});
  showMsg(r.message||r.error);
  loadBackups();
}
async function loadBackups(){
  const list=await api('/api/backups');
  const div=document.getElementById('backups-list');
  if(!Array.isArray(list)||list.length===0){div.innerHTML='<p class="empty">No backups yet. Click "Backup Now".</p>';return;}
  div.innerHTML='<table><thead><tr><th>Name</th><th>Size</th><th>Time</th></tr></thead><tbody>'+
    list.map(b=>'<tr><td>'+b.name+'</td><td>'+(b.size/1024).toFixed(0)+' KB</td><td>'+new Date(b.time).toLocaleString()+'</td></tr>').join('')+
    '</tbody></table>';
}
loadStatus();loadSites();loadBackups();
setInterval(loadStatus,5000);setInterval(loadSites,5000);
</script>
</body>
</html>`
