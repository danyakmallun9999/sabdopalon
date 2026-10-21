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
import { Area, AreaChart, Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts"

import api, {
  poll,
  type ServiceStatus,
  type SetupStatus,
  type TrafficPoint,
} from "@/lib/api"
import SetupPage from "@/pages/setup"
import { useLive } from "@/lib/live-context"
import { dbEngineRunning } from "@/lib/dbstatus"
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
import { IconButton } from "@/components/icon-button"
import { usePageRefresh } from "@/components/page-refresh"
import { MetricStrip } from "@/components/metric-strip"
import { StatusDot, ToneBadge } from "@/components/status"
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
    color: "var(--chart-1)",
  },
} satisfies ChartConfig

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
            <CardTitle>{svc.label}</CardTitle>
            <CardDescription className="font-mono text-[13.5px]">
              {svc.ports?.join("  ") || "no ports"}
            </CardDescription>
          </div>
          <ToneBadge tone={svc.running ? "success" : "muted"}>
            <StatusDot status={svc.running ? "running" : "stopped"} />
            {svc.running ? "running" : svc.installed ? "installed" : "not installed"}
          </ToneBadge>
        </div>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex items-center justify-between rounded-md border border-ring-inner bg-surface-inset px-3 py-2.5">
          <span className="text-[13.5px] text-text-secondary">
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
              className="inline-flex items-center gap-1 text-[13.5px] text-brand hover:underline"
            >
              Open UI <ExternalLink className="size-3.5" />
            </a>
          )}
        </div>

        {!svc.installed && !svc.running && (
          <p className="text-[13.5px] leading-relaxed text-text-muted">{svc.hint || "Install its package from Packages."}</p>
        )}
        {svc.last_error && !svc.running && (
          <p className="flex items-center gap-1 text-[13.5px] text-destructive">
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
          <p className="text-[13px] text-text-secondary">{error}</p>
        ) : (
          <pre
            ref={preRef}
            className="max-h-[55vh] overflow-y-auto rounded-xl border border-ring-inner bg-surface-inset p-4 font-mono text-[13.5px] leading-relaxed whitespace-pre-wrap text-text-secondary"
          >
            {lines.length ? lines.join("\n") : "No logs yet."}
          </pre>
        )}
      </DialogContent>
    </Dialog>
  )
}

// Last known dashboard state, kept outside the component.
//
// Switching tabs unmounts this page, so every return used to start from
// `setup = null` and refetch: the render fell into the skeleton branch below for
// the length of one round trip to /api/setup/status. That is the flash of
// skeleton seen when moving to Dashboard and back, and it happened even though
// nothing had changed and the data had been on screen a second earlier.
//
// Holding the snapshot at module scope makes a revisit render what was already
// known while the refresh runs in the background. The skeleton is then reserved
// for what it was meant for: the first load of the session, when there is
// genuinely nothing to show.
type DashboardSnapshot = {
  setup: SetupStatus | null
  services: ServiceStatus[]
  traffic: TrafficPoint[]
  trafficTotal: number
  history: { t: number; running: number; enabled: number }[]
  setupDone: boolean
}

let dashboardSnapshot: DashboardSnapshot | null = null
export default function DashboardPage() {
  const navigate = useNavigate()
  const { status } = useLive()
  const [setup, setSetup] = useState<SetupStatus | null>(dashboardSnapshot?.setup ?? null)
  const [services, setServices] = useState<ServiceStatus[]>(dashboardSnapshot?.services ?? [])
  const [traffic, setTraffic] = useState<TrafficPoint[]>(dashboardSnapshot?.traffic ?? [])
  const [trafficTotal, setTrafficTotal] = useState(dashboardSnapshot?.trafficTotal ?? 0)
  const [busy, setBusy] = useState<string | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [logFor, setLogFor] = useState<string | null>(null)
  const [dbBusy, setDbBusy] = useState<string | null>(null)
  const [serviceHistory, setServiceHistory] = useState<{ t: number; running: number; enabled: number }[]>(
    dashboardSnapshot?.history ?? [],
  )
  const [setupDone, setSetupDone] = useState(dashboardSnapshot?.setupDone ?? false)

  // Mirror the current state into the module-level snapshot. Written on every
  // change rather than on unmount, because a tab switch can unmount this page
  // without running a cleanup here (and a cleanup would capture stale values).
  useEffect(() => {
    dashboardSnapshot = { setup, services, traffic, trafficTotal, history: serviceHistory, setupDone }
  }, [setup, services, traffic, trafficTotal, serviceHistory, setupDone])
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

  // Stable polling callback — useCallback with [] deps so the interval NEVER
  // restarts (the [services]-dependent callback previously reset the effect on
  // every fetch → fetch/setState loop that hung the UI on navigation).
  // Everything it reads lives in a ref or a functional update.
  const loadAll = useCallback(() => {
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
  }, [])

  useEffect(() => {
    if (!setupDone) return
    const t = poll(loadAll, 5000)
    return () => clearInterval(t)
  }, [setupDone, loadAll])

  // The refresh button lives in the top bar, so the bar needs to know what
  // refreshing this page means.
  usePageRefresh(loadAll)

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

  // Runtime start/stop (does not change the enable config).
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
      loadAll()
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
      loadAll()
    } finally {
      setBusy(null)
    }
  }

  // Inline DBMS start/stop (MariaDB & PostgreSQL) — same API the Database
  // page uses. Lets users start/stop a database without leaving the dashboard.
  // The full Database page keeps port config, restart, and backups.
  async function dbAction(engine: "mariadb" | "postgresql", action: "start" | "stop") {
    setDbBusy(engine + action)
    try {
      const r = await api.databaseControl(engine, action)
      if (r.error) toast.error(`${engine} ${action}: ${r.error}`)
      else toast.success(r.message ?? `${engine}: ${action} OK`)
      // db_states arrives via the live status poll; nothing extra to fetch.
    } finally {
      setDbBusy(null)
    }
  }

  // Start All / Stop All: only installed services.
  async function startAll() {
    setBusy("all")
    try {
      let attempted = 0
      let failed = 0
      const failures: string[] = []
      for (const svc of services) {
        if (svc.running || !svc.installed) continue
        attempted++
        const r = await api.startService(svc.name)
        if (r.error) {
          failed++
          failures.push(`${svc.label}: ${r.error}`)
          setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
        }
      }
      if (attempted === 0) {
        toast.info("No services to start.")
      } else if (failed === 0) {
        toast.success(`Start All done — ${attempted} services started.`)
      } else if (failed === attempted) {
        toast.error(`Start All failed — ${failed}/${attempted} services could not be started.`, {
          description: failures.join("\n"),
        })
      } else {
        toast.warning(`Start All: ${attempted - failed} succeeded, ${failed} failed.`, {
          description: failures.join("\n"),
        })
      }
      loadAll()
    } finally {
      setBusy(null)
    }
  }

  async function stopAll() {
    setBusy("all")
    try {
      let attempted = 0
      let failed = 0
      const failures: string[] = []
      for (const svc of services) {
        if (!svc.running) continue
        attempted++
        const r = await api.stopService(svc.name)
        if (r.error) {
          failed++
          failures.push(`${svc.label}: ${r.error}`)
          setErrors((e) => ({ ...e, [svc.name]: r.error || "" }))
        }
      }
      if (attempted === 0) {
        toast.info("No services currently running.")
      } else if (failed === 0) {
        toast.success(`Stop All done — ${attempted} services stopped.`)
      } else if (failed === attempted) {
        toast.error(`Stop All failed — ${failed}/${attempted} services could not be stopped.`, {
          description: failures.join("\n"),
        })
      } else {
        toast.warning(`Stop All: ${attempted - failed} succeeded, ${failed} failed.`, {
          description: failures.join("\n"),
        })
      }
      loadAll()
    } finally {
      setBusy(null)
    }
  }

  if (!setup) {
    return (
      <div className="flex flex-col gap-5 px-5 py-5 lg:px-7">
        <Skeleton className="h-14 w-64" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (!setup.bootstrapped) {
    return (
      <div className="flex flex-col gap-5 px-5 py-5 lg:px-7">
        <SetupPage />
      </div>
    )
  }

  // Lifecycle controls live on the Database page — this card links there.

  // Only installed tools are shown & can be started.
  const installed = services.filter((s) => s.installed)
  const runningCount = installed.filter((s) => s.running).length
  const anyError = Object.values(errors).some(Boolean) || installed.some((s) => s.last_error)
  // An engine counts as active only when its daemon is actually ready (sqlite always is).
  // Unknown state (snapshot not yet arrived) must not render as ✓.
  const dbRunning = dbEngineRunning(status)
  const dbEngine = status?.database ?? "—"
  const peakTraffic = traffic.reduce((max, p) => Math.max(max, p.requests ?? 0), 0)

  return (
    <div className="flex flex-col gap-5 px-5 py-7 lg:px-7">
      {/* The four numbers are one card, not four: the divider between the
          cells is what makes them read as one fact about the machine. */}
      <MetricStrip
        metrics={[
          {
            icon: Globe,
            label: "Sites",
            value: String(status?.sites_count ?? 0),
            sub: `*.${status?.tld ?? "localhost"}`,
          },
          {
            icon: Boxes,
            label: "Services",
            value: `${runningCount}/${installed.length}`,
            sub: `${installed.length} installed`,
          },
          {
            icon: Activity,
            label: "Requests",
            value: String(trafficTotal),
            sub: `HTTP :${status?.http_port ?? "?"} · HTTPS :${status?.https_port ?? "?"}`,
          },
          {
            icon: Database,
            label: "Databases",
            value:
              Object.entries(status?.db_states ?? {}).filter(([, v]) => v).length +
              "/2",
            sub: (
              <span className="flex flex-wrap items-center gap-1.5">
                {Object.entries(status?.db_states ?? {}).length > 0
                  ? Object.entries(status?.db_states ?? {}).map(
                      ([k, v]) => (
                        <span key={k} className="inline-flex items-center gap-1.5 text-text-secondary">
                          <StatusDot status={v ? "running" : "stopped"} />
                          {k}
                        </span>
                      )
                    )
                  : "mariadb · postgresql"}
              </span>
            ),
            onClick: () => navigate("/database"),
            title: "Manage databases — start/stop/restart",
          },
        ]}
      />

      {/* Error banner (port conflict etc.) */}
      {anyError && (
        <div className="flex items-start gap-3 rounded-xl border border-destructive/25 bg-danger-soft p-4">
          <CircleAlert className="text-destructive mt-0.5 size-5 shrink-0" />
          <div className="flex flex-col gap-1">
            <p className="text-[13.5px] font-medium text-text-primary">Some services failed to start</p>
            {Object.entries(errors).map(([name, msg]) => (
              <p key={name} className="text-[13.5px] text-text-secondary">
                <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[13.5px] text-text-primary">{name}</code>: {msg}
              </p>
            ))}
            {installed
              .filter((s) => s.last_error)
              .map((s) => (
                <p key={s.name} className="text-[13.5px] text-text-secondary">
                  <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[13.5px] text-text-primary">{s.name}</code>: {s.last_error}
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
              <CardTitle className="flex items-center gap-2">
                <LayoutDashboard className="size-[15px] text-brand" /> Server
                <ToneBadge tone={runningCount > 0 ? "success" : "muted"}>
                  {runningCount > 0
                    ? `${runningCount} services running`
                    : "No services running"}
                </ToneBadge>
              </CardTitle>
              <CardDescription>
                {runningCount > 0 ? `${runningCount} services running` : "No services running"} · DB{" "}
                {Object.entries(status?.db_states ?? {})
                  .map(([k, v]) => `${k} ${v ? "✓" : "✗"}`)
                  .join(" · ") || `${dbEngine} ${dbRunning ? "✓" : "✗"}`}{" "}
                · Proxy :{status?.http_port ?? "?"}
              </CardDescription>
            </div>
            <div className="flex items-center gap-3">
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

      {/* Database engines — inline start/stop (port config & backups on the
          Database page). Each engine is independent and can run at once. */}
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2">
        {(["mariadb", "postgresql"] as const).map((engine) => {
          const running = Boolean(status?.db_states?.[engine])
          const err = status?.db_errors?.[engine]
          const isBusy = dbBusy !== null && dbBusy.startsWith(engine)
          return (
            <Card key={engine}>
              <CardHeader>
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="flex flex-col gap-1">
                    <CardTitle>{engine === "mariadb" ? "MariaDB" : "PostgreSQL"}</CardTitle>
                    <CardDescription className="font-mono text-[13.5px]">
                      {engine === "mariadb" ? ":3306" : ":5433"}
                    </CardDescription>
                  </div>
                  <ToneBadge tone={running ? "success" : "muted"}>
                    <StatusDot status={running ? "running" : "stopped"} />
                    {running ? "running" : "stopped"}
                  </ToneBadge>
                </div>
                {err && (
                  <p className="mt-1 flex items-center gap-1 text-[13.5px] text-destructive">
                    <CircleAlert className="size-3" /> {err}
                  </p>
                )}
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  {running ? (
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={isBusy}
                      onClick={() => dbAction(engine, "stop")}
                    >
                      {isBusy && dbBusy === engine + "stop" ? <RefreshCw className="animate-spin" /> : <Square />}
                      Stop
                    </Button>
                  ) : (
                    <Button size="sm" disabled={isBusy} onClick={() => dbAction(engine, "start")}>
                      {isBusy && dbBusy === engine + "start" ? <RefreshCw className="animate-spin" /> : <Play />}
                      Start
                    </Button>
                  )}
                  <a
                    href="/database"
                    className="inline-flex items-center gap-1 text-[13.5px] text-brand hover:underline"
                  >
                    Manage <ExternalLink className="size-3.5" />
                  </a>
                </div>
              </CardHeader>
            </Card>
          )
        })}
      </div>

      {/* Service table (compact overview) */}
      <Card>
        <CardHeader>
          <CardTitle>Installed services</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Service</TableHead>
                  <TableHead>Ports</TableHead>
                  <TableHead className="w-24">Status</TableHead>
                  <TableHead className="w-40 text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {installed.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-[13.5px] text-text-muted">
                      No services installed yet — add Mailpit, Redis, MinIO or
                      Meilisearch from Packages.
                    </TableCell>
                  </TableRow>
                )}
                {installed.map((svc) => (
                  <TableRow key={svc.name}>
                    <TableCell className="font-medium">{svc.label}</TableCell>
                    <TableCell className="font-mono text-[13.5px] text-text-secondary">{svc.ports?.join("  ")}</TableCell>
                    <TableCell>
                      {/* Same vocabulary as the Sites table: a state is never
                          the primary button colour, and the dot always travels
                          with its label. */}
                      <ToneBadge tone={svc.running ? "success" : "muted"}>
                        <StatusDot status={svc.running ? "running" : "stopped"} />
                        {svc.running ? "running" : svc.enabled ? "enabled" : "stopped"}
                      </ToneBadge>
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        {svc.running ? (
                          <Button size="sm" variant="outline" onClick={() => stopService(svc)} disabled={busy === svc.name}>
                            <Square /> Stop
                          </Button>
                        ) : (
                          <Button size="sm" variant="outline" onClick={() => startService(svc)} disabled={busy === svc.name}>
                            <Play /> Start
                          </Button>
                        )}
                        <IconButton
                          label="Logs"
                          size="icon-sm"
                          variant="ghost"
                          onClick={() => setLogFor(svc.name)}
                        >
                          <ScrollText />
                        </IconButton>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Service grid — installed tools only */}
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
            <CardTitle>Additional services</CardTitle>
            <CardDescription>
              No additional tools installed yet. Install via the{" "}
              <a href="/packages" className="text-brand hover:underline">Packages</a>{" "}
              (Mailpit, Redis, MinIO, Meilisearch, Adminer).
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      {/* Charts */}
      <div className="grid grid-cols-1 gap-4 @5xl/main:grid-cols-2">
        <Card>
          <CardHeader>
            <div className="flex items-start justify-between gap-3">
              <div className="flex flex-col gap-1">
                <CardTitle>Traffic</CardTitle>
                <CardDescription>Requests per minute · last 30 minutes</CardDescription>
              </div>
              <ToneBadge tone="muted">peak {peakTraffic}/min</ToneBadge>
            </div>
          </CardHeader>
          <CardContent>
            <ChartContainer config={chartConfig} className="h-[220px] w-full">
              <BarChart data={traffic} margin={{ left: -20, right: 8, top: 8 }}>
                <CartesianGrid vertical={false} stroke="var(--border-soft)" />
                <XAxis
                  dataKey="t"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  minTickGap={24}
                  tick={{ fill: "var(--text-muted)", fontSize: 10 }}
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  allowDecimals={false}
                  tick={{ fill: "var(--text-muted)", fontSize: 10 }}
                />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar dataKey="requests" fill="var(--color-requests)" radius={[3, 3, 0, 0]} maxBarSize={18} />
              </BarChart>
            </ChartContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Service status</CardTitle>
            <CardDescription>Running vs enabled (last 30 samples)</CardDescription>
          </CardHeader>
          <CardContent>
            <ChartContainer
              config={{
                running: { label: "Running", color: "var(--chart-2)" },
                enabled: { label: "Enabled", color: "var(--chart-3)" },
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
                <CartesianGrid vertical={false} stroke="var(--border-soft)" />
                <XAxis
                  dataKey="t"
                  tickLine={false}
                  axisLine={false}
                  tickMargin={8}
                  minTickGap={24}
                  tick={{ fill: "var(--text-muted)", fontSize: 10 }}
                  // t = epoch minutes → display as hour:minute
                  tickFormatter={(v: number) =>
                    new Date(v * 60000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
                  }
                />
                <YAxis
                  tickLine={false}
                  axisLine={false}
                  allowDecimals={false}
                  tick={{ fill: "var(--text-muted)", fontSize: 10 }}
                />
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

      <LogDialog name={logFor} onClose={() => setLogFor(null)} />
    </div>
  )
}
