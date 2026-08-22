import { useCallback, useEffect, useRef, useState } from "react"
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
  type Status,
  type TrafficPoint,
} from "@/lib/api"
import SetupPage from "@/pages/setup"
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
          <span className="text-sm font-normal">{svc.enabled ? "Enabled (auto-start)" : "Disabled"}</span>
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
  const [setup, setSetup] = useState<SetupStatus | null>(null)
  const [status, setStatus] = useState<Status | null>(null)
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [traffic, setTraffic] = useState<TrafficPoint[]>([])
  const [trafficTotal, setTrafficTotal] = useState(0)
  const [busy, setBusy] = useState<string | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [logFor, setLogFor] = useState<string | null>(null)
  const [serviceHistory, setServiceHistory] = useState<{ t: string; running: number; enabled: number }[]>([])
  const [setupDone, setSetupDone] = useState(false)

  // Wizard → dashboard transition: after setup finishes the server reloads
  // the SPA; if it doesn't, poll until bootstrapped flips.
  const refreshSetup = useCallback(() => {
    api.setupStatus().then((s) => {
      setSetup(s)
      if (s.bootstrapped) setSetupDone(true)
    }).catch(() => {})
  }, [])

  useEffect(() => {
    const t = setInterval(refreshSetup, 3000)
    refreshSetup()
    return () => clearInterval(t)
  }, [refreshSetup])

  const loadAll = useCallback(async () => {
    api.status().then(setStatus).catch(() => {})
    api.services().then((d) => setServices(d.services || [])).catch(() => {})
    api.traffic().then((t) => {
      setTrafficTotal(t.total)
      setTraffic(t.per_minute || [])
    }).catch(() => {})
    // Client-side service history for the mini chart.
    setServiceHistory((h) => {
      const now = new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
      const next = [...h, { t: now, running: services.filter((s) => s.running).length, enabled: services.filter((s) => s.enabled).length }]
      return next.length > 30 ? next.slice(next.length - 30) : next
    })
  }, [services])

  useEffect(() => {
    if (!setupDone) return
    const t = poll(loadAll, 5000)
    return () => clearInterval(t)
  }, [setupDone, loadAll])

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
      loadAll()
    } finally {
      setBusy(null)
    }
  }

  // Start All / Stop All: start/stop each installed service individually
  // (toggle would flip the persisted enable flag, which we don't want here).
  async function startAll() {
    setBusy("all")
    try {
      for (const svc of services) {
        if (svc.running) continue
        if (!svc.installed) continue
        const r = await api.toggleService(svc.name, true)
        if (r.error) setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
      }
      toast.success("Start All selesai")
      loadAll()
    } finally {
      setBusy(null)
    }
  }

  async function stopAll() {
    setBusy("all")
    try {
      for (const svc of services) {
        if (!svc.running) continue
        const r = await api.toggleService(svc.name, false)
        if (r.error) setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
      }
      toast.success("Stop All selesai")
      loadAll()
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

  const runningCount = services.filter((s) => s.running).length
  const enabledCount = services.filter((s) => s.enabled).length
  const anyError = Object.values(errors).some(Boolean)

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <div className="flex items-center justify-between gap-2">
        <p className="text-muted-foreground text-sm">
          Ringkasan server, layanan, dan lalu lintas — semua dalam satu layar.
        </p>
        <Button size="sm" variant="outline" onClick={loadAll} disabled={busy === "all"}>
          <RefreshCw className={busy === "all" ? "animate-spin" : ""} /> Refresh
        </Button>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-4">
        <StatCard icon={Globe} label="Sites" value={String(status?.sites_count ?? 0)} sub={`*.${status?.tld ?? "localhost"}`} />
        <StatCard icon={Boxes} label="Services" value={`${runningCount}/${services.length}`} sub={`${enabledCount} enabled`} />
        <StatCard icon={Database} label="Database" value={status?.database ?? "—"} sub="engine" />
        <StatCard icon={Activity} label="Requests" value={String(trafficTotal)} sub={`HTTP :${status?.http_port ?? "?"} · HTTPS :${status?.https_port ?? "?"}`} />
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
                {status?.database ?? "—"} · Proxy :{status?.http_port ?? "?"}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Button size="sm" onClick={startAll} disabled={busy === "all"}>
                <Play /> Start All
              </Button>
              <Button size="sm" variant="outline" onClick={stopAll} disabled={busy === "all"}>
                <Square /> Stop All
              </Button>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Service grid */}
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-3">
        {services.map((svc) => (
          <ServiceCard
            key={svc.name}
            svc={svc}
            busy={busy === svc.name || busy === "all"}
            onToggle={(v) => toggleService(svc, v)}
            onStart={() => toggleService(svc, true)}
            onStop={() => toggleService(svc, false)}
            onLogs={() => setLogFor(svc.name)}
          />
        ))}
      </div>

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
                <XAxis dataKey="t" tickLine={false} axisLine={false} tickMargin={8} minTickGap={24} />
                <YAxis tickLine={false} axisLine={false} allowDecimals={false} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Area dataKey="running" type="monotone" fill="url(#fillRunning)" stroke="var(--color-running)" strokeWidth={2} />
                <Area dataKey="enabled" type="monotone" fill="transparent" stroke="var(--color-enabled)" strokeWidth={2} strokeDasharray="4 4" />
              </AreaChart>
            </ChartContainer>
          </CardContent>
        </Card>
      </div>

      {/* Service table (compact overview) */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Semua service</CardTitle>
          <CardDescription>Status ringkas semua layanan terdaftar.</CardDescription>
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
                {services.map((svc) => (
                  <TableRow key={svc.name}>
                    <TableCell className="font-medium">{svc.label}</TableCell>
                    <TableCell className="font-mono text-xs">{svc.ports?.join("  ")}</TableCell>
                    <TableCell>
                      <Badge variant={svc.running ? "default" : "outline"}>
                        {svc.running ? "running" : svc.enabled ? "enabled" : "off"}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="inline-flex items-center gap-1">
                        {svc.running ? (
                          <Button size="sm" variant="outline" onClick={() => toggleService(svc, false)} disabled={busy === svc.name}>
                            <Square /> Stop
                          </Button>
                        ) : (
                          <Button size="sm" onClick={() => toggleService(svc, true)} disabled={busy === svc.name || !svc.installed}>
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
