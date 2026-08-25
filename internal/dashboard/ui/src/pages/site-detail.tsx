import { useCallback, useEffect, useRef, useState } from "react"
import { useParams } from "react-router-dom"
import { toast } from "sonner"
import {
  Box,
  Clock,
  Database,
  ExternalLink,
  FileCode2,
  FolderTree,
  Globe,
  Lock,
  Play,
  RotateCw,
  ScrollText,
  Server,
  Settings2,
  Square,
  Terminal as TerminalIcon,
  Zap,
} from "lucide-react"

import api, {
  type DevToolStatus,
  type LogResponse,
  type SiteConfigPayload,
  type SiteDetail,
  poll,
} from "@/lib/api"
import TerminalPanel, { type TermStatus } from "@/components/terminal-panel"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { useLive } from "@/lib/live"

type Tab = "overview" | "config" | "logs" | "devtools" | "terminal"

// Persist the active tab per-site across route changes and reloads, so a
// refresh returns to the same view instead of snapping to "Overview". The
// key is per-site (namespaced by name) so each site remembers its own tab.
function tabKeyFor(name: string): string {
  return `sabdopalon.site.${name}.tab`
}
function loadTab(name: string): Tab {
  try {
    const v = localStorage.getItem(tabKeyFor(name))
    if (v === "overview" || v === "config" || v === "logs" || v === "devtools" || v === "terminal") return v
  } catch {
    /* fresh */
  }
  return "overview"
}

export default function SiteDetailPage() {
  const { name } = useParams<{ name: string }>()
  const [detail, setDetail] = useState<SiteDetail | null>(null)
  const [tab, setTab] = useState<Tab>(() => (name ? loadTab(name) : "overview"))
  const [busy, setBusy] = useState<string | null>(null)

  // Persist the active tab so a reload/route change returns to the same view.
  useEffect(() => {
    if (!name) return
    try {
      localStorage.setItem(tabKeyFor(name), tab)
    } catch {
      /* storage unavailable — non-fatal */
    }
  }, [tab, name])

  const load = useCallback(() => {
    if (!name) return
    api.siteDetail(name).then(setDetail).catch(() => setDetail(null))
  }, [name])

  useEffect(() => {
    const t = poll(load, 5000)
    return () => clearInterval(t)
  }, [load])

  if (!name) return null

  if (!detail) {
    return (
      <div className="flex h-full items-center justify-center text-muted-foreground">
        Loading…
      </div>
    )
  }

  async function act(action: "start" | "stop" | "restart") {
    if (!name) return
    setBusy(action)
    try {
      const r = await api.siteAction(name, action)
      if (r.error) toast.error(r.error)
      else if (action === "restart") toast.success(`${name}: restarted`)
      load()
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col px-4 lg:px-6">
      {/* Sticky header */}
      <div className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-1">
          <div className="flex items-center gap-2">
            {detail.framework !== "unknown" && (
              <Badge variant="secondary">{detail.framework}</Badge>
            )}
            <span className="inline-flex items-center gap-1.5 text-sm">
              <span
                className={`size-2 rounded-full ${detail.running ? "bg-emerald-500" : "bg-zinc-400 dark:bg-zinc-600"}`}
              />
              <span className="text-muted-foreground">{detail.running ? "running" : "stopped"}</span>
            </span>
          </div>
          <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
            <a href={detail.url} target="_blank" rel="noreferrer" className="text-primary inline-flex items-center gap-1 hover:underline">
              <Globe className="size-3" /> {detail.url.replace(/^https?:\/\//, "").replace(/\/$/, "")}
              <ExternalLink className="size-3 opacity-60" />
            </a>
            <span className="opacity-30">·</span>
            <a href={detail.https} target="_blank" rel="noreferrer" className="text-muted-foreground inline-flex items-center gap-1 hover:underline">
              <Lock className="size-3 text-emerald-500" /> {detail.https.replace(/^https?:\/\//, "").replace(/\/$/, "")}
            </a>
            {detail.port ? (
              <>
                <span className="opacity-30">·</span>
                <span className="font-mono">:{detail.port}</span>
              </>
            ) : null}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {detail.running ? (
            <>
              <Button size="sm" variant="outline" disabled={busy === "restart"} onClick={() => act("restart")}>
                <RotateCw /> Restart
              </Button>
              <Button size="sm" variant="outline" disabled={busy === "stop"} onClick={() => act("stop")}>
                <Square /> Stop
              </Button>
            </>
          ) : (
            <Button size="sm" disabled={busy === "start"} onClick={() => act("start")}>
              <Play /> Start
            </Button>
          )}
        </div>
      </div>

      {/* Tabs */}
      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)} className="flex min-h-0 flex-1 flex-col">
        <TabsList className="flex-wrap">
          <TabsTrigger value="overview"><Box className="size-3.5" /> Overview</TabsTrigger>
          <TabsTrigger value="config"><Settings2 className="size-3.5" /> Config</TabsTrigger>
          <TabsTrigger value="logs"><ScrollText className="size-3.5" /> Logs</TabsTrigger>
          <TabsTrigger value="devtools"><Zap className="size-3.5" /> Dev Tools</TabsTrigger>
          <TabsTrigger value="terminal"><TerminalIcon className="size-3.5" /> Terminal</TabsTrigger>
        </TabsList>

        <TabsContent value="overview" className="mt-4 min-h-0 flex-1 overflow-y-auto">
          <OverviewTab detail={detail} />
        </TabsContent>
        <TabsContent value="config" className="mt-4 min-h-0 flex-1 overflow-y-auto">
          <ConfigTab name={detail.name} config={detail.config} onSaved={load} />
        </TabsContent>
        <TabsContent value="logs" className="mt-4 min-h-0 flex-1 overflow-y-auto">
          <LogsTab name={detail.name} sources={detail.logs} />
        </TabsContent>
        <TabsContent value="devtools" className="mt-4 min-h-0 flex-1 overflow-y-auto">
          <DevToolsTab name={detail.name} tools={detail.devtools} onChanged={load} />
        </TabsContent>
        <TabsContent value="terminal" className="mt-4 min-h-0 flex-1 overflow-hidden">
          <TerminalTab name={detail.name} dir={detail.dir} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

/* --------------------------------- Overview -------------------------------- */

function OverviewTab({ detail }: { detail: SiteDetail }) {
  const { status } = useLive()

  const cards = [
    {
      icon: FileCode2,
      title: "Framework",
      value: detail.framework === "unknown" ? "Plain PHP" : detail.framework,
      sub: detail.framework === "laravel" ? "Front controller: public/index.php" : "Router: .sabdopalon-router.php",
    },
    {
      icon: Server,
      title: "PHP",
      value: detail.php.binary ? detail.php.binary.split("/").pop() ?? "PHP" : "default",
      sub: detail.php.version ?? "",
    },
    {
      icon: Database,
      title: "Database",
      value: status?.database ?? "—",
      sub: status?.db_running ? "running" : "stopped",
    },
    {
      icon: FolderTree,
      title: "Site directory",
      value: detail.dir,
      sub: detail.running ? `listening on :${detail.port}` : "not running",
    },
  ]

  return (
    <div className="flex flex-col gap-4 pb-6">
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
        {cards.map((c) => (
          <Card key={c.title}>
            <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
              <CardTitle className="text-muted-foreground text-xs font-medium uppercase tracking-wide">
                {c.title}
              </CardTitle>
              <c.icon className="text-muted-foreground size-4" />
            </CardHeader>
            <CardContent>
              <div className="truncate font-mono text-sm font-medium" title={c.value}>
                {c.value}
              </div>
              {c.sub && <p className="text-muted-foreground mt-1 truncate text-xs">{c.sub}</p>}
            </CardContent>
          </Card>
        ))}
      </div>

      {/* URLs card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-sm"><Globe className="size-4" /> URLs</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <a href={detail.url} target="_blank" rel="noreferrer" className="text-primary inline-flex items-center gap-2 text-sm hover:underline">
            <Globe className="size-3.5" /> {detail.url} <ExternalLink className="size-3 opacity-60" />
          </a>
          <a href={detail.https} target="_blank" rel="noreferrer" className="text-muted-foreground inline-flex items-center gap-2 text-sm hover:underline">
            <Lock className="size-3.5 text-emerald-500" /> {detail.https}
          </a>
          {detail.config.aliases?.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {detail.config.aliases.map((a) => (
                <Badge key={a} variant="secondary">{a}</Badge>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* DevTools summary */}
      {detail.devtools.filter((t) => t.running).length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-sm"><Zap className="size-4" /> Running Dev Tools</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {detail.devtools.filter((t) => t.running).map((t) => (
              <div key={t.tool} className="flex items-center gap-2 text-sm">
                <span className="size-2 rounded-full bg-emerald-500" />
                <span className="font-medium">{t.label}</span>
                {t.port ? <Badge variant="secondary" className="font-mono">:{t.port}</Badge> : null}
                <span className="text-muted-foreground text-xs">PID {t.pid}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

/* ---------------------------------- Config --------------------------------- */

function ConfigTab({ name, config, onSaved }: { name: string; config: SiteConfigPayload; onSaved: () => void }) {
  const { status } = useLive()
  const [cfg, setCfg] = useState<SiteConfigPayload>(config)
  const [phpOptions, setPhpOptions] = useState<{ value: string; label: string; group: string }[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    setCfg(config)
  }, [config])

  useEffect(() => {
    const opts: { value: string; label: string; group: string }[] = [
      { value: "", label: `Default (${status?.php_version ?? "system-first"})`, group: "General" },
    ]
    Promise.all([api.listPackages(), api.systemPHPs()])
      .then(([pkgs, sys]) => {
        pkgs.filter((p) => p.is_php).forEach((p) =>
          opts.push({ value: p.short, label: `${p.short}${p.installed ? "" : " (not downloaded)"}`, group: "Bundled" }),
        )
        sys.forEach((c) => opts.push({ value: c.path, label: `${c.version} — ${c.path}`, group: "System" }))
        setPhpOptions(opts)
      })
      .catch(() => {})
  }, [status?.php_version])

  async function save() {
    setSaving(true)
    try {
      const r = await api.saveSiteConfig(name, cfg)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? "Saved")
      onSaved()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-4 pb-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-sm">PHP Version</CardTitle>
        </CardHeader>
        <CardContent>
          <Select value={cfg.php} onValueChange={(v) => setCfg({ ...cfg, php: v ?? "" })}>
            <SelectTrigger className="w-full"><SelectValue placeholder="Default" /></SelectTrigger>
            <SelectContent>
              {["General", "Bundled", "System"].map((group) => (
                <div key={group}>
                  <div className="text-muted-foreground px-2 py-1 text-xs font-medium">{group}</div>
                  {phpOptions.filter((o) => o.group === group).map((o) => (
                    <SelectItem key={o.value || "__default"} value={o.value}>{o.label}</SelectItem>
                  ))}
                </div>
              ))}
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Custom php.ini</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <PhpIniEditor name={name} onSaved={onSaved} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Document Root</CardTitle>
        </CardHeader>
        <CardContent>
          <Input
            placeholder="public"
            value={cfg.docroot}
            onChange={(e) => setCfg({ ...cfg, docroot: e.target.value })}
          />
          <p className="text-muted-foreground mt-2 text-xs">Relative to the site folder. Laravel uses <code>public</code>.</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Aliases</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Textarea
            rows={3}
            placeholder={"www.myapp.localhost\napi.myapp.test"}
            value={(cfg.aliases ?? []).join("\n")}
            onChange={(e) =>
              setCfg({ ...cfg, aliases: e.target.value.split("\n").map((x) => x.trim()) })
            }
          />
          <p className="text-muted-foreground text-xs">One per line. Custom domains (not *.localhost) need a hosts entry.</p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">Environment Variables</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-2">
          <Textarea
            rows={4}
            placeholder={"APP_ENV=local\nAPP_DEBUG=true"}
            value={Object.entries(cfg.env ?? {}).map(([k, v]) => `${k}=${v}`).join("\n")}
            onChange={(e) => {
              const env: Record<string, string> = {}
              e.target.value.split("\n").forEach((line) => {
                const idx = line.indexOf("=")
                if (idx > 0) env[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
              })
              setCfg({ ...cfg, env })
            }}
          />
          <p className="text-muted-foreground text-xs">KEY=value per line. Injected into the PHP process.</p>
        </CardContent>
      </Card>

      <Button onClick={save} disabled={saving} className="w-fit">
        {saving ? "Saving…" : "Save & apply"}
      </Button>
    </div>
  )
}

/* ----------------------------------- Logs ---------------------------------- */

function LogsTab({ name, sources }: { name: string; sources: string[] }) {
  const [current, setCurrent] = useState<string>("")
  const [log, setLog] = useState<LogResponse | null>(null)
  const [auto, setAuto] = useState(true)
  const scrollRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (sources.length > 0 && !sources.includes(current)) {
      setCurrent(sources[0])
    } else if (sources.length === 0 && current === "") {
      setCurrent("php")
    }
  }, [sources, current])

  useEffect(() => {
    if (!current) return
    const tick = () => api.siteLogs(name, current).then(setLog).catch(() => setLog(null))
    tick()
    if (!auto) return
    const t = setInterval(tick, 2500)
    return () => clearInterval(t)
  }, [name, current, auto])

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [log])

  return (
    <div className="flex flex-col gap-3 pb-6">
      <div className="flex items-center justify-between gap-2">
        <div className="flex flex-wrap gap-1.5">
          {(sources.length > 0 ? sources : ["php"]).map((src) => (
            <Button
              key={src}
              size="sm"
              variant={src === current ? "default" : "outline"}
              onClick={() => setCurrent(src)}
            >
              {src}
            </Button>
          ))}
        </div>
        <label className="text-muted-foreground flex items-center gap-2 text-xs">
          auto-refresh
          <Switch checked={auto} onCheckedChange={setAuto} />
        </label>
      </div>
      <p className="text-muted-foreground font-mono text-xs">
        logs/{log?.file ?? `${name}.${current}.log`}
      </p>
      <pre
        ref={scrollRef}
        className="bg-background text-muted-foreground max-h-[55vh] min-h-48 overflow-y-auto rounded-lg border p-3 font-mono text-xs leading-relaxed"
      >
        {log?.error ?? (log?.lines.length ? log.lines.join("\n") : "No logs yet.")}
      </pre>
    </div>
  )
}

/* --------------------------------- DevTools -------------------------------- */

function DevToolsTab({ name, tools, onChanged }: { name: string; tools: DevToolStatus[]; onChanged: () => void }) {
  const [busyTool, setBusyTool] = useState<string | null>(null)

  async function toggleTool(tool: string, running: boolean) {
    setBusyTool(tool)
    try {
      const r = await api.siteDevToolAction(name, tool, running ? "stop" : "start")
      if (r.error) toast.error(r.error)
      else toast.success(`${tool}: ${running ? "stopped" : r.port ? `started on :${r.port}` : "started"}`)
      onChanged()
    } finally {
      setBusyTool(null)
    }
  }

  // Filter out tools that are "not applicable"
  const applicable = tools.filter((t) => t.last_error !== "not applicable for this project")

  return (
    <div className="flex flex-col gap-4 pb-6">
      <div className="flex items-center gap-2 text-sm">
        <Zap className="size-4 text-amber-500" />
        <p className="text-muted-foreground">
          Start Vite, Artisan, or npm scripts directly from here — no terminal needed.
          Vite is auto-proxied so <code className="bg-muted rounded px-1">{name}.localhost</code> serves HMR assets.
        </p>
      </div>
      {applicable.length === 0 && (
        <div className="text-muted-foreground flex h-32 items-center justify-center text-sm">
          No dev-tools detected for this project. Create a <code>vite.config.js</code>, <code>artisan</code>, or <code>package.json</code> to enable tools.
        </div>
      )}
      {applicable.map((t) => (
        <Card key={t.tool}>
          <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
            <CardTitle className="flex items-center gap-2 text-sm">
              <span className={`size-2.5 rounded-full ${t.running ? "bg-emerald-500" : "bg-zinc-400 dark:bg-zinc-600"}`} />
              {t.label}
            </CardTitle>
            <Button
              size="sm"
              variant={t.running ? "outline" : "default"}
              disabled={busyTool === t.tool}
              onClick={() => toggleTool(t.tool, t.running)}
            >
              {t.running ? <Square /> : <Play />}
              {t.running ? "Stop" : "Start"}
            </Button>
          </CardHeader>
          <CardContent>
            <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
              {t.running ? (
                <>
                  <span className="inline-flex items-center gap-1">
                    <Clock className="size-3" /> PID {t.pid}
                  </span>
                  {t.port ? <Badge variant="secondary" className="font-mono">:{t.port}</Badge> : null}
                </>
              ) : (
                <span>stopped</span>
              )}
              {t.last_error && t.running === false && (
                <span className="text-amber-500 dark:text-amber-400">{t.last_error}</span>
              )}
            </div>
            {t.running && (
              <ToolLog name={name} tool={t.tool} />
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  )
}

function ToolLog({ name, tool }: { name: string; tool: string }) {
  const [log, setLog] = useState<LogResponse | null>(null)
  const scrollRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    const tick = () => api.siteLogs(name, tool).then(setLog).catch(() => {})
    tick()
    const t = setInterval(tick, 2000)
    return () => clearInterval(t)
  }, [name, tool])

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [log])

  return (
    <pre
      ref={scrollRef}
      className="bg-background text-muted-foreground mt-3 max-h-48 min-h-24 overflow-y-auto rounded-lg border p-2.5 font-mono text-xs leading-relaxed"
    >
      {log?.error ?? (log?.lines.length ? log.lines.join("\n") : "No output yet.")}
    </pre>
  )
}

/* --------------------------------- Terminal -------------------------------- */

function TerminalTab({ name, dir }: { name: string; dir: string }) {
  const [termStatus, setTermStatus] = useState<TermStatus>("connecting")
  const panelRef = useRef<{ clear: () => void; restart: () => void }>(null)

  return (
    <div className="flex h-full min-h-0 flex-col overflow-hidden rounded-xl border bg-background">
      <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2">
        <TerminalIcon className="size-4 shrink-0" />
        <span className="text-sm font-medium">{name}</span>
        <span className={`size-2 rounded-full ${termStatus === "connected" ? "bg-emerald-500" : termStatus === "connecting" ? "bg-amber-500 animate-pulse" : "bg-red-400"}`} />
        <span className="text-muted-foreground text-xs">{termStatus}</span>
      </div>
      <div className="relative min-h-0 flex-1 overflow-hidden">
        <TerminalPanel
          ref={panelRef}
          dir={dir}
          sessionKey={`site-detail:${name}`}
          className="absolute inset-0 h-auto border-0 rounded-none"
          onStatus={setTermStatus}
        />
      </div>
    </div>
  )
}

/* ------------------------------ PhpIni Editor ------------------------------ */

function PhpIniEditor({ name, onSaved }: { name: string; onSaved: () => void }) {
  const [content, setContent] = useState("")
  const [exists, setExists] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let active = true
    api.getSitePhpIni(name).then((r) => {
      if (!active) return
      setContent(r.content)
      setExists(r.exists)
    }).catch(() => {})
    return () => { active = false }
  }, [name])

  async function save() {
    setSaving(true)
    try {
      const r = await api.saveSitePhpIni(name, content)
      if (r.error) toast.error(r.error)
      else { toast.success(r.message); setExists(true); onSaved() }
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <Textarea
        rows={12}
        spellCheck={false}
        className="resize-y font-mono text-xs"
        placeholder="; memory_limit = 256M&#10; display_errors = On"
        value={content}
        onChange={(e) => setContent(e.target.value)}
      />
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-xs">
          {exists
            ? "Editing sites/" + name + "/php.ini — overrides the global php.ini for this site."
            : "No per-site php.ini yet — this default will create sites/" + name + "/php.ini on save."}
        </p>
        <Button size="sm" onClick={save} disabled={saving}>
          {saving ? "Saving…" : "Save php.ini"}
        </Button>
      </div>
    </div>
  )
}
