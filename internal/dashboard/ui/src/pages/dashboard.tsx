import { useCallback, useEffect, useRef, useState } from "react"
import { useNavigate } from "react-router-dom"
import { toast } from "sonner"
import {
  Activity,
  Boxes,
  CircleAlert,
  Database,
  ExternalLink,
  Globe,
  LayoutDashboard,
  Play,
  RefreshCw,
  ScrollText,
  Square,
} from "lucide-react"
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts"

import api, {
  poll,
  type ServiceStatus,
  type SetupStatus,
  type TrafficPoint,
} from "@/lib/api"
import SetupPage from "@/pages/setup"
import { useLive } from "@/lib/live"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"

const chartConfig = {
  requests: {
    label: "Requests",
    color: "hsl(var(--chart-1))",
  },
} satisfies ChartConfig

function StatCard({
  icon: Icon,
  label,
  value,
  sub,
}: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  sub?: string
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-muted-foreground text-sm font-medium">{label}</CardTitle>
        <Icon className="text-muted-foreground size-4" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-bold">{value}</div>
        {sub && <p className="text-muted-foreground text-xs">{sub}</p>}
      </CardContent>
    </Card>
  )
}

function ServiceCard({
  svc,
  onToggle,
  onStart,
  onStop,
  onLogs,
  busy,
}: {
  svc: ServiceStatus
  onToggle: (enabled: boolean) => void
  onStart: () => void
  onStop: () => void
  onLogs: () => void
  busy: boolean
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-start justify-between gap-2">
          <div className="flex flex-col gap-1">
            <CardTitle className="text-base">{svc.label}</CardTitle>
            <CardDescription className="font-mono text-xs">
              {svc.ports?.join("  ") || "no ports"}
            </CardDescription>
          </div>
          <Badge
            variant={svc.running ? "default" : svc.installed ? "outline" : "secondary"}
            className={svc.running ? "" : "text-muted-foreground"}
          >
            {svc.running ? "running" : svc.installed ? "installed" : "not installed"}
          </Badge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex items-center justify-between rounded-lg border p-3">
          <span className="text-sm font-normal">
            {svc.enabled ? "Auto-start ON" : "Auto-start OFF"}
          </span>
          <Switch checked={!!svc.enabled} disabled={busy} onCheckedChange={onToggle} />
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {svc.running ? (
            <Button size="sm" variant="outline" disabled={busy} onClick={onStop}>
              <Square /> Stop
            </Button>
          ) : (
            <Button size="sm" disabled={busy || !svc.installed} onClick={onStart}>
              <Play /> Start
            </Button>
          )}
          <Button size="sm" variant="ghost" disabled={busy} onClick={onLogs}>
            <ScrollText /> Logs
          </Button>
          {svc.running && svc.ui && (
            <a
              href={svc.ui}
              target="_blank"
              rel="noreferrer"
              className="text-primary inline-flex items-center gap-1 text-sm hover:underline"
            >
              Open UI <ExternalLink className="size-3.5" />
            </a>
          )}
        </div>

        {!svc.installed && !svc.running && (
          <p className="text-muted-foreground text-xs">{svc.hint || "Install its package from Packages."}</p>
        )}
        {svc.last_error && !svc.running && (
          <p className="text-destructive flex items-center gap-1 text-xs">
            <CircleAlert className="size-3" /> {svc.last_error}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

function LogDialog({
  name,
  onClose,
}: {
  name: string | null
  onClose: () => void
}) {
  const [lines, setLines] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const preRef = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (!name) return
    const load = () => {
      api.logs(name).then((d) => {
        if ("error" in d && d.error) setError(d.error)
        else {
          setError(null)
          setLines(Array.isArray(d.lines) ? d.lines : [])
        }
      }).catch(() => setError("failed to load log"))
    }
    load()
    const t = setInterval(load, 5000)
    return () => clearInterval(t)
  }, [name])

  useEffect(() => {
    preRef.current?.scrollTo({ top: preRef.current.scrollHeight })
  }, [lines])

  return (
    <Dialog open={!!name} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-h-[80vh] sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Log — {name}</DialogTitle>
          <DialogDescription>Auto-refreshing (5s).</DialogDescription>
        </DialogHeader>
        {error ? (
          <p className="text-muted-foreground text-sm">{error}</p>
        ) : (
          <pre
            ref={preRef}
            className="bg-background text-muted-foreground max-h-[55vh] overflow-y-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap"
          >
            {lines.length ? lines.join("\n") : "No log entries yet."}
          </pre>
        )}
      </DialogContent>
    </Dialog>
  )
}

export default function DashboardPage() {
  const navigate = useNavigate()
  const { status } = useLive()
  const [setup, setSetup] = useState<SetupStatus | null>(null)
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [traffic, setTraffic] = useState<TrafficPoint[]>([])
  const [trafficTotal, setTrafficTotal] = useState(0)
  const [busy, setBusy] = useState<string | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [logFor, setLogFor] = useState<string | null>(null)
  const [serviceHistory, setServiceHistory] = useState<{ t: number; running: number; enabled: number }[]>([])
  const [setupDone, setSetupDone] = useState(false)

  // Mounted guard: prevents setState after unmount (nav away mid-fetch),
  // which was one cause of the dashboard→sites hang.
  const mountedRef = useRef(true)
  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
    }
  }, [])

  // Latest services snapshot for the history sampler (avoid stale closure).
  const servicesRef = useRef<ServiceStatus[]>([])
  useEffect(() => {
    servicesRef.current = services
  }, [services])

  // Wizard → dashboard transition: after setup finishes the server reloads
  // the SPA; if it doesn't, poll until bootstrapped flips.
  const refreshSetup = useCallback(() => {
    api.setupStatus().then((s) => {
      if (!mountedRef.current) return
      setSetup(s)
      if (s.bootstrapped) setSetupDone(true)
    }).catch(() => {})
  }, [])

  useEffect(() => {
    const t = setInterval(refreshSetup, 3000)
    refreshSetup()
    return () => clearInterval(t)
  }, [refreshSetup])

  // Stable polling callback — lives in a ref so the interval NEVER restarts
  // (the [services]-dependent callback previously reset the effect on every
  // fetch → fetch/setState loop that hung the UI on navigation).
  const loadAllRef = useRef<() => void>(() => {})
  loadAllRef.current = () => {
    api.services().then((d) => {
      if (mountedRef.current) setServices(d.services || [])
    }).catch(() => {})
    api.traffic().then((t) => {
      if (!mountedRef.current) return
      setTrafficTotal(t.total)
      setTraffic(t.per_minute || [])
    }).catch(() => {})
    // Service history sampler: one point per minute (epoch), deduped.
    const minute = Math.floor(Date.now() / 60000)
    setServiceHistory((h) => {
      if (h.length && h[h.length - 1].t === minute) return h
      const svcs = servicesRef.current
      const next = [
        ...h,
        {
          t: minute,
          running: svcs.filter((s) => s.running).length,
          enabled: svcs.filter((s) => s.enabled).length,
        },
      ]
      return next.length > 60 ? next.slice(next.length - 60) : next
    })
  }

  useEffect(() => {
    if (!setupDone) return
    const t = poll(() => loadAllRef.current(), 5000)
    return () => clearInterval(t)
  }, [setupDone])

  async function toggleService(svc: ServiceStatus, enabled: boolean) {
    setBusy(svc.name)
    try {
      const r = await api.toggleService(svc.name, enabled)
      if (r.error) {
        toast.error(r.error)
        setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
      } else {
        toast.success(r.message ?? `${svc.name}: ${enabled ? "on" : "off"}`)
        setErrors((e) => {
          const n = { ...e }
          delete n[svc.name]
          return n
        })
      }
      loadAllRef.current()
    } finally {
      setBusy(null)
    }
  }

  // Runtime start/stop (tidak mengubah config enable).
  async function startService(svc: ServiceStatus) {
    setBusy(svc.name)
    try {
      const r = await api.startService(svc.name)
      if (r.error) {
        toast.error(r.error)
        setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
      } else {
        toast.success(r.message ?? `${svc.name} started`)
        setErrors((e) => {
          const n = { ...e }
          delete n[svc.name]
          return n
        })
      }
      loadAllRef.current()
    } finally {
      setBusy(null)
    }
  }

  async function stopService(svc: ServiceStatus) {
    setBusy(svc.name)
    try {
      const r = await api.stopService(svc.name)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `${svc.name} stopped`)
      loadAllRef.current()
    } finally {
      setBusy(null)
    }
  }

  // Start All / Stop All: hanya service yang terinstall.
  async function startAll() {
    setBusy("all")
    try {
      for (const svc of services) {
        if (svc.running || !svc.installed) continue
        const r = await api.startService(svc.name)
        if (r.error) setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
      }
      toast.success("Start All selesai")
      loadAllRef.current()
    } finally {
      setBusy(null)
    }
  }

  async function stopAll() {
    setBusy("all")
    try {
      for (const svc of services) {
        if (!svc.running) continue
        const r = await api.stopService(svc.name)
        if (r.error) setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
      }
      toast.success("Stop All selesai")
      loadAllRef.current()
    } finally {
      setBusy(null)
    }
  }

  if (!setup) {
    return (
      <div className="flex flex-col gap-4 px-4 lg:px-6">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (!setup.bootstrapped) {
    return (
      <div className="flex flex-col gap-4 px-4 lg:px-6">
        <SetupPage />
      </div>
    )
  }

  // Lifecycle controls live on the Database page — this card links there.

  // Hanya tool yang sudah terinstall yang ditampilkan & bisa di-start.
  const installed = services.filter((s) => s.installed)
  const runningCount = installed.filter((s) => s.running).length
  const anyError = Object.values(errors).some(Boolean) || installed.some((s) => s.last_error)
  // DB engine selalu aktif selama server jalan (sqlite/mariadb dikelola otomatis).
  const dbRunning = status?.db_running ?? true
  const dbEngine = status?.database ?? "—"

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <div className="flex items-center justify-between gap-2">
        <p className="text-muted-foreground text-sm">
          Ringkasan server, layanan, dan lalu lintas — semua dalam satu layar.
        </p>
        <Button size="sm" variant="outline" onClick={() => loadAllRef.current()} disabled={busy === "all"}>
          <RefreshCw className={busy === "all" ? "animate-spin" : ""} /> Refresh
        </Button>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
        <StatCard icon={Globe} label="Sites" value={String(status?.sites_count ?? 0)} sub={`*.${status?.tld ?? "localhost"}`} />
        <StatCard icon={Boxes} label="Services" value={`${runningCount}/${installed.length}`} sub={`${installed.length} terinstall`} />
        <StatCard icon={Activity} label="Requests" value={String(trafficTotal)} sub={`HTTP :${status?.http_port ?? "?"} · HTTPS :${status?.https_port ?? "?"}`} />
        <div
          role="button"
          tabIndex={0}
          onClick={() => navigate("/database")}
          onKeyDown={(e) => e.key === "Enter" && navigate("/database")}
          className="cursor-pointer transition-shadow hover:shadow-md rounded-xl"
          title="Kelola database — start/stop/restart"
        >
          <StatCard
            icon={Database}
            label="Databases"
            value={Object.entries(status?.db_states ?? {}).filter(([, v]) => v).length + "/2"}
            sub={
              Object.keys(status?.db_states ?? {}).length > 0
                ? Object.entries(status?.db_states ?? {})
                    .map(([k, v]) => `${k} ${v ? "✓" : "✗"}`)
                    .join(" · ")
                : "mariadb · postgresql"
            }
          />
        </div>
      </div>

      {/* Error banner (port conflict etc.) */}
      {anyError && (
        <div className="bg-destructive/10 flex items-start gap-3 rounded-xl border border-destructive/30 p-4">
          <CircleAlert className="text-destructive mt-0.5 size-5 shrink-0" />
          <div className="flex flex-col gap-1">
            <p className="text-sm font-medium">Ada layanan yang gagal dijalankan</p>
            {Object.entries(errors).map(([name, msg]) => (
              <p key={name} className="text-muted-foreground text-xs">
                <code className="bg-muted rounded px-1 py-0.5">{name}</code>: {msg}
              </p>
            ))}
            {installed
              .filter((s) => s.last_error)
              .map((s) => (
                <p key={s.name} className="text-muted-foreground text-xs">
                  <code className="bg-muted rounded px-1 py-0.5">{s.name}</code>: {s.last_error}
                </p>
              ))}
          </div>
        </div>
      )}

      {/* Server control card */}
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex flex-col gap-1">
              <CardTitle className="flex items-center gap-2 text-base">
                <LayoutDashboard className="size-4" /> Server
              </CardTitle>
              <CardDescription>
                {runningCount > 0 ? `${runningCount} service berjalan` : "Tidak ada service berjalan"} · DB{" "}
                {Object.entries(status?.db_states ?? {})
                  .map(([k, v]) => `${k} ${v ? "✓" : "✗"}`)
                  .join(" · ") || `${dbEngine} ${dbRunning ? "✓" : "✗"}`}{" "}
                · Proxy :{status?.http_port ?? "?"}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" onClick={startAll} disabled={busy === "all" || installed.length === 0}>
                <Play /> Start All
              </Button>
              <Button size="sm" variant="outline" onClick={stopAll} disabled={busy === "all"}>
                <Square /> Stop All
              </Button>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Service grid — hanya tool yang terinstall */}
      {installed.length > 0 ? (
        <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-3">
          {installed.map((svc) => (
            <ServiceCard
              key={svc.name}
              svc={svc}
              busy={busy === svc.name || busy === "all"}
              onToggle={(v) => toggleService(svc, v)}
              onStart={() => startService(svc)}
              onStop={() => stopService(svc)}
              onLogs={() => setLogFor(svc.name)}
            />
          ))}
        </div>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Layanan tambahan</CardTitle>
            <CardDescription>
              Belum ada tool tambahan yang terinstall. Pasang lewat halaman{" "}
              <a href="/packages" className="text-primary hover:underline">Packages</a>{" "}
              (Mailpit, Redis, MinIO, Meilisearch, Adminer).
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      {/* Charts */}
      <div className="grid grid-cols-1 gap-4 @5xl/main:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Traffic (requests/menit)</CardTitle>
            <CardDescription>30 menit terakhir — melalui proxy</CardDescription>
          </CardHeader>
          <CardContent>
            <ChartContainer config={chartConfig} className="h-[220px] w-full">
              <AreaChart data={traffic} margin={{ left: -20, right: 8, top: 8 }}>
                <defs>
                  <linearGradient id="fillRequests" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--color-requests)" stopOpacity={0.4} />
                    <stop offset="95%" stopColor="var(--color-requests)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="t" tickLine={false} axisLine={false} tickMargin={8} minTickGap={24} />
                <YAxis tickLine={false} axisLine={false} allowDecimals={false} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Area
                  dataKey="requests"
                  type="monotone"
                  fill="url(#fillRequests)"
                  stroke="var(--color-requests)"
                  strokeWidth={2}
                />
              </AreaChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Service status</CardTitle>
            <CardDescription>Running vs enabled (30 sampel terakhir)</CardDescription>
          </CardHeader>
          <CardContent>
            <ChartContainer
              config={{
                running: { label: "Running", color: "hsl(var(--chart-2))" },
                enabled: { label: "Enabled", color: "hsl(var(--chart-3))" },
              }}
              className="h-[220px] w-full"
            >
              <AreaChart data={serviceHistory} margin={{ left: -20, right: 8, top: 8 }}>
                <defs>
                  <linearGradient id="fillRunning" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="var(--color-running)" stopOpacity={0.4} />
                    <stop offset="95%" stopColor="var(--color-running)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey="t"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  minTickGap={24}
                  // t = epoch menit → tampilkan jam:menit
                  tickFormatter={(v: number) =>
                    new Date(v * 60000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
                  }
                />
                <YAxis tickLine={false} axisLine={false} allowDecimals={false} />
                <ChartTooltip
                  content={<ChartTooltipContent />}
                  labelFormatter={(v) =>
                    new Date(Number(v) * 60000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
                  }
                />
                <Area dataKey="running" type="stepAfter" connectNulls fill="url(#fillRunning)" stroke="var(--color-running)" strokeWidth={2} />
                <Area dataKey="enabled" type="stepAfter" connectNulls fill="transparent" stroke="var(--color-enabled)" strokeWidth={2} strokeDasharray="4 4" />
              </AreaChart>
            </ChartContainer>
          </CardContent>
        </Card>
      </div>

      {/* Service table (compact overview) */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Layanan terinstall</CardTitle>
          <CardDescription>Status ringkas tool yang sudah terpasang.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Service</TableHead>
                  <TableHead>Ports</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {installed.map((svc) => (
                  <TableRow key={svc.name}>
                    <TableCell className="font-medium">{svc.label}</TableCell>
                    <TableCell className="font-mono text-xs">{svc.ports?.join("  ")}</TableCell>
                    <TableCell>
                      <Badge variant={svc.running ? "default" : "outline"}>
                        {svc.running ? "running" : svc.enabled ? "enabled" : "stopped"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="inline-flex items-center gap-1">
                        {svc.running ? (
                          <Button size="sm" variant="outline" onClick={() => stopService(svc)} disabled={busy === svc.name}>
                            <Square /> Stop
                          </Button>
                        ) : (
                          <Button size="sm" onClick={() => startService(svc)} disabled={busy === svc.name}>
                            <Play /> Start
                          </Button>
                        )}
                        <Button size="icon-sm" variant="ghost" onClick={() => setLogFor(svc.name)} title="Logs">
                          <ScrollText />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <LogDialog name={logFor} onClose={() => setLogFor(null)} />
    </div>
  )
}
