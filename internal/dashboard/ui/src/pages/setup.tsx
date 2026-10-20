import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import {
  CheckCircle2,
  ChevronDown,
  Database,
  HardDrive,
  LoaderCircle,
  Mail,
  Rocket,
  Search,
  Server,
  Boxes,
  ArrowRight,
} from "lucide-react"

import api, { type SetupJob, type SetupStatus, type SetupTool } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Progress } from "@/components/ui/progress"
import { ToneBadge } from "@/components/status"
import { FieldError } from "@/components/ui/field"

const TOOL_ICONS: Record<string, typeof Database> = {
  postgresql: Database,
  redis: Boxes,
  mailpit: Mail,
  minio: HardDrive,
  meilisearch: Search,
}

const CORE_ICONS: Record<string, typeof Database> = {
  php: Server,
  mariadb: Database,
  phpmyadmin: Boxes,
}

export default function SetupPage() {
  const [status, setStatus] = useState<SetupStatus | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [advanced, setAdvanced] = useState(false)
  const [tld, setTld] = useState("localhost")
  const [httpPort, setHttpPort] = useState("8080")
  const [httpsPort, setHttpsPort] = useState("8443")
  const [sample, setSample] = useState(true)
  const [job, setJob] = useState<SetupJob | null>(null)
  const [progress, setProgress] = useState(5)
  const logRef = useRef<HTMLPreElement>(null)

  // Real component/tool inventory — never static text.
  useEffect(() => {
    api
      .setupStatus()
      .then(setStatus)
      .catch(() => toast.error("Failed to load installation status"))
  }, [])

  // Poll the setup job while it runs; progress is derived from the poll
  // result itself (no setState-in-effect).
  useEffect(() => {
    if (!job?.running) return
    const t = setInterval(() => {
      api
        .setupJob()
        .then((j) => {
          setJob(j)
          if (j.done) setProgress(100)
          else setProgress((p) => Math.min(p + 1.5 + Math.random() * 4, 95))
          // Orphaned job: the desktop shell restarted the sidecar after the
          // setup completed — this fresh server knows nothing about the job
          // (running=false, done=false, empty output). The install is over;
          // head to the dashboard instead of freezing here forever.
          if (!j.running && !j.done) {
            api
              .setupStatus()
              .then((s) => {
                if (s.bootstrapped) void reloadWhenReady()
              })
              .catch(() => {})
          }
        })
        .catch(() => {})
    }, 800)
    return () => clearInterval(t)
  }, [job?.running])

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
    if (job?.done && !job?.running && !job?.error) {
      // The desktop shell restarts the sidecar in full mode right after the
      // config lands — wait until the REAL server answers (proxy bound),
      // then reload. A blind reload here lands on a dead port.
      void reloadWhenReady()
    }
  }, [job?.done, job?.running, job?.error])

  function toggleTool(key: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  // Validation belongs to the fields (components/ui/field.tsx explains why):
  // a toast about a port shows up far from the input, expires on its own, and
  // leaves the input looking perfectly fine.
  const [errors, setErrors] = useState<{ tld?: string; ports?: string }>({})

  async function start() {
    const hp = parseInt(httpPort, 10)
    const sp = parseInt(httpsPort, 10)
    const next: { tld?: string; ports?: string } = {}
    if (!Number.isInteger(hp) || hp < 1 || hp > 65535 || !Number.isInteger(sp) || sp < 1 || sp > 65535) {
      next.ports = "Every port must be a number from 1 to 65535."
    }
    if (!/^[a-z0-9.]+$/.test(tld.trim())) {
      next.tld = "Lowercase letters, numbers and dots only."
    }
    setErrors(next)
    if (next.tld || next.ports) return
    setProgress(5)
    const r = await api.runSetup({
      install_mariadb: true,
      db_engine: "mariadb",
      tools: [...selected],
      tld: tld.trim(),
      http_port: hp,
      https_port: sp,
      create_sample_site: sample,
    })
    if (r.error) {
      toast.error(r.error)
      return
    }
    setJob({ running: true, done: false, output: "" })
  }

  const installing = job !== null
  const success = Boolean(job?.done && !job?.running && !job?.error)
  const availableTools = (status?.tools ?? []).filter((t) => !t.installed)
  const activeTools = (status?.tools ?? []).filter((t) => t.installed)

  return (
    <div className="flex min-h-[calc(100dvh-var(--tb-h,0px))] flex-col bg-background">
      {installing ? (
        <InstallPanel job={job} progress={progress} success={success} logRef={logRef} />
      ) : (
        <>
          <main className="relative mx-auto w-full max-w-5xl flex-1 px-4 py-4 sm:px-6 sm:py-5">
            {/* Header / Brand */}
            <div className="mb-4 flex flex-col gap-1 sm:mb-5 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-3">
                <div className="flex size-10 shrink-0 items-center justify-center rounded-xl border border-ring-outer bg-ring-gap p-[3px] shadow-2xs">
                  <div className="flex size-full items-center justify-center rounded-[9px] border border-ring-inner bg-surface p-1">
                    <img
                      src="/logo.png"
                      alt="Sabdopalon"
                      className="size-full object-contain"
                    />
                  </div>
                </div>
                <div className="flex flex-col">
                  <div className="flex items-center gap-2">
                    <h1 className="text-[22px] leading-tight font-semibold tracking-[-0.025em] text-text-primary sm:text-[26px]">
                      Sabdopalon
                    </h1>
                    <Badge variant="secondary" className="px-2 py-0 text-[10px] font-medium tracking-[0.08em] uppercase">
                      Setup
                    </Badge>
                  </div>
                  <p className="text-[11.5px] text-text-secondary">
                    Configure your local server and pick components to start your development environment.
                  </p>
                </div>
              </div>
            </div>

            <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-12">
              {/* LEFT — core + settings */}
              <div className="flex flex-col gap-4 lg:col-span-7">
                {/* Core components */}
                <Card size="sm">
                  <CardHeader className="pb-2.5">
                    <div className="flex items-center justify-between">
                      <div className="flex flex-col gap-0.5">
                        <CardTitle>Core Components</CardTitle>
                        <CardDescription>
                          Required components always provisioned in the local environment.
                        </CardDescription>
                      </div>
                      <ToneBadge tone="muted" className="shrink-0">
                        3 components
                      </ToneBadge>
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-2 pt-0">
                    {(status?.components ?? []).map((c) => (
                      <div
                        key={c.key}
                        className="flex items-center justify-between gap-3 rounded-md border border-ring-inner bg-surface-inset px-3 py-2"
                      >
                        <div className="flex min-w-0 items-center gap-2.5">
                          <div className="flex size-8 shrink-0 items-center justify-center rounded-md border border-ring-inner bg-surface-raised/70">
                            {coreIcon(c.key)}
                          </div>
                          <div className="flex min-w-0 flex-col">
                            <span className="truncate text-[12px] font-medium text-text-primary">{c.label}</span>
                            <span className="truncate text-[10.5px] text-text-muted">
                              {c.installed
                                ? c.version
                                  ? `v${c.version}`
                                  : "detected"
                                : "will be installed by the wizard"}
                            </span>
                          </div>
                        </div>
                        {c.installed ? (
                          <ToneBadge tone="success" className="shrink-0">
                            <CheckCircle2 className="size-3" /> Installed
                          </ToneBadge>
                        ) : (
                          <ToneBadge tone="muted" className="shrink-0">
                            Included
                          </ToneBadge>
                        )}
                      </div>
                    ))}
                    {!status && <SkeletonRows rows={3} />}
                  </CardContent>
                </Card>

                {/* Settings */}
                <Card size="sm">
                  <CardHeader className="pb-2.5">
                    <div className="flex flex-col gap-0.5">
                      <CardTitle>Settings</CardTitle>
                      <CardDescription>
                        Customize the initial local server configuration.
                      </CardDescription>
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-3 pt-0">
                    <label className="flex cursor-pointer items-start justify-between gap-3 rounded-md border border-ring-inner bg-surface-inset p-2.5 transition-colors hover:bg-surface-raised/60">
                      <span className="flex min-w-0 flex-col gap-0.5">
                        <span className="text-[12px] font-medium text-text-primary">Create sample site</span>
                        <span className="text-[10.5px] leading-relaxed text-text-muted">
                          The <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[10px] text-text-secondary">myapp</code> site will be ready to open once setup finishes
                        </span>
                      </span>
                      <Checkbox checked={sample} onCheckedChange={(v) => setSample(v === true)} className="mt-0.5 shrink-0" />
                    </label>

                    <div className="flex flex-col gap-2 rounded-md border border-ring-inner bg-surface-inset p-2.5">
                      <button
                        type="button"
                        onClick={() => setAdvanced((v) => !v)}
                        className="flex w-full items-center justify-between text-[11.5px] font-medium text-text-secondary transition-colors hover:text-text-primary"
                      >
                        <span className="flex items-center gap-1.5">
                          <ChevronDown className={`size-3.5 transition-transform duration-200 ${advanced ? "rotate-180" : ""}`} />
                          Advanced settings (domain & ports)
                        </span>
                        <span className="text-[10.5px] font-normal text-text-muted">
                          {advanced ? "Hide" : "Customize"}
                        </span>
                      </button>

                      {advanced && (
                        <div className="grid grid-cols-1 gap-2.5 border-t border-border-soft pt-3 sm:grid-cols-3">
                          <div className="flex flex-col gap-1">
                            <Label htmlFor="setup-tld">
                              Domain lokal (*.…)
                            </Label>
                            <Input
                              id="setup-tld"
                              value={tld}
                              onChange={(e) => setTld(e.target.value)}
                              placeholder="localhost"
                              className="h-[30px] font-mono text-[11.5px]"
                              aria-invalid={!!errors.tld}
                              aria-describedby={errors.tld ? "setup-tld-error" : undefined}
                            />
                            <FieldError id="setup-tld-error">{errors.tld}</FieldError>
                          </div>
                          <div className="flex flex-col gap-1">
                            <Label htmlFor="setup-http">
                              Port HTTP
                            </Label>
                            <Input
                              id="setup-http"
                              inputMode="numeric"
                              value={httpPort}
                              onChange={(e) => setHttpPort(e.target.value)}
                              className="h-[30px] font-mono text-[11.5px]"
                              aria-invalid={!!errors.ports}
                              aria-describedby={errors.ports ? "setup-ports-error" : undefined}
                            />
                          </div>
                          <div className="flex flex-col gap-1">
                            <Label htmlFor="setup-https">
                              Port HTTPS
                            </Label>
                            <Input
                              id="setup-https"
                              inputMode="numeric"
                              value={httpsPort}
                              onChange={(e) => setHttpsPort(e.target.value)}
                              className="h-[30px] font-mono text-[11.5px]"
                              aria-invalid={!!errors.ports}
                              aria-describedby={errors.ports ? "setup-ports-error" : undefined}
                            />
                          </div>
                          <FieldError id="setup-ports-error" className="col-span-full">
                            {errors.ports}
                          </FieldError>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </div>

              {/* RIGHT — optional tools */}
              <div className="flex flex-col gap-4 lg:col-span-5">
                <Card size="sm">
                  <CardHeader className="pb-2.5">
                    <div className="flex items-center justify-between">
                      <div className="flex flex-col gap-0.5">
                        <CardTitle>Additional Tools</CardTitle>
                        <CardDescription>
                          Check what to install now — the rest can be added later via Packages.
                        </CardDescription>
                      </div>
                      {selected.size > 0 && (
                        <ToneBadge tone="success" className="shrink-0">
                          {selected.size} selected
                        </ToneBadge>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-2 pt-0">
                    {availableTools.length === 0 && status && (
                      <div className="rounded-md border border-ring-inner bg-surface-inset p-3 text-center">
                        <p className="text-[11px] text-text-muted">All tools already installed 🎉</p>
                      </div>
                    )}
                    {availableTools.map((tool) => (
                      <ToolRow key={tool.key} tool={tool} checked={selected.has(tool.key)} onToggle={() => toggleTool(tool.key)} />
                    ))}
                    {!status && <SkeletonRows rows={4} />}

                    {activeTools.length > 0 && (
                      <div className="mt-2 flex flex-col gap-1.5 border-t border-border-soft pt-3">
                        <span className="text-[10px] font-medium uppercase tracking-[0.08em] text-text-muted">
                          Already Active
                        </span>
                        <div className="flex flex-wrap gap-1.5">
                          {activeTools.map((t) => (
                            <ToneBadge key={t.key} tone="success">
                              <CheckCircle2 className="size-3" /> {t.label}
                            </ToneBadge>
                          ))}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            </div>
          </main>

          {/* Footer — CTA always visible without scrolling */}
          <footer className="sticky bottom-0 z-30 mt-auto shrink-0 border-t border-border-soft bg-background/95 px-4 py-3 backdrop-blur-md sm:px-6">
            <div className="mx-auto flex w-full max-w-5xl items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-[11px] text-text-muted">
                <span className="size-1.5 shrink-0 animate-pulse rounded-full bg-success" />
                <span className="text-[11px]">
                  {selected.size > 0
                    ? `${selected.size} additional tools will be downloaded + 3 core components.`
                    : "Only the 3 core components will be provisioned."}
                </span>
              </div>
              <Button
                size="default"
                className="min-w-48 gap-2 font-medium sm:min-w-56"
                disabled={!status}
                onClick={start}
              >
                <Rocket className="size-4" /> Finish setup <ArrowRight className="size-4" />
              </Button>
            </div>
          </footer>
        </>
      )}
    </div>
  )
}

// Poll until the full server (proxy actually bound) replaces the setup-mode
// instance, then hand over. The desktop shell restarts the sidecar right after
// the wizard finishes — a blind reload here would land on a dead port.
// Bounded; falls back to the same handover.
//
// It lands on /dashboard explicitly: the wizard used to render its own success
// panel (with a "Go to Dashboard" button and a second copy of the same
// sentence) on top of a page that then reloaded into the dashboard, which read
// as the same screen twice.
async function reloadWhenReady() {
  for (let i = 0; i < 60; i++) {
    try {
      const r = await fetch("/api/status", { cache: "no-store" })
      if (r.ok) {
        const s = await r.json()
        if ((s?.http_port ?? 0) > 0) break
      }
    } catch {
      /* server restarting — keep waiting */
    }
    await new Promise((res) => setTimeout(res, 600))
  }
  window.location.replace("/dashboard")
}

function coreIcon(key: string) {
  const Icon = CORE_ICONS[key] ?? Server
  return <Icon className="size-4 text-muted-foreground" />
}

function ToolRow({ tool, checked, onToggle }: { tool: SetupTool; checked: boolean; onToggle: () => void }) {
  const Icon = TOOL_ICONS[tool.key] ?? Boxes
  return (
    <label
      className={`group flex cursor-pointer items-start justify-between gap-3 rounded-md border p-2.5 transition-all select-none ${
        checked
          ? "border-brand/35 bg-brand-soft"
          : "border-ring-inner bg-surface-inset hover:bg-surface-raised/60"
      }`}
    >
      <span className="flex min-w-0 items-start gap-2.5">
        <div
          className={`mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg border transition-colors ${
            checked ? "border-brand/30 bg-brand/10 text-brand" : "border-ring-inner bg-surface-raised text-text-muted"
          }`}
        >
          <Icon className="size-3.5" />
        </div>
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-[12px] font-medium text-text-primary">{tool.label}</span>
          <span className="mt-0.5 line-clamp-2 text-[10.5px] leading-relaxed text-text-muted">
            {tool.description}
          </span>
        </span>
      </span>
      <Checkbox
        checked={checked}
        onCheckedChange={onToggle}
        className="mt-0.5 shrink-0"
        aria-label={tool.label}
      />
    </label>
  )
}

function SkeletonRows({ rows }: { rows: number }) {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="h-11 animate-pulse rounded-md border border-ring-inner bg-surface-raised/60" />
      ))}
    </div>
  )
}

function InstallPanel({
  job,
  progress,
  success,
  logRef,
}: {
  job: SetupJob | null
  progress: number
  success: boolean
  logRef: React.RefObject<HTMLPreElement | null>
}) {
  return (
    <div className="mx-auto flex min-h-0 w-full max-w-3xl flex-1 flex-col gap-3 px-4 py-4 sm:px-6 sm:py-5">
      {/* Header Status Card */}
      <Card>
        <CardContent className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            {success ? (
              <div className="flex size-8 items-center justify-center rounded-md border border-success/20 bg-success-soft text-success">
                <CheckCircle2 className="size-4.5" />
              </div>
            ) : (
              <div className="flex size-8 items-center justify-center rounded-md border border-brand/20 bg-brand-soft text-brand">
                <LoaderCircle className="size-4.5 animate-spin" />
              </div>
            )}
            <div className="flex flex-col">
              <h2 className="text-[14.5px] font-medium text-text-primary">
                {success ? "Setup complete" : "Setting up Sabdopalon…"}
              </h2>
              <p className="text-[11px] text-text-muted">
                {success
                  ? "Opening the dashboard…"
                  : "Downloading packages and setting up the install directory..."}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <Progress value={progress} className="h-2 w-32 sm:w-44" />
            <span className="font-mono text-[11.5px] font-medium text-text-primary tabular-nums">
              {Math.round(progress)}%
            </span>
          </div>
        </div>
        </CardContent>
      </Card>

      {/* Terminal log panel */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-ring-inner bg-[#0a0a0a]">
        <div className="flex items-center justify-between border-b border-border-soft bg-surface-inset px-3.5 py-2">
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1.5">
              <span className="size-2 rounded-full bg-red-500/80" />
              <span className="size-2 rounded-full bg-amber-500/80" />
              <span className="size-2 rounded-full bg-emerald-500/80" />
            </div>
            <span className="ml-1.5 font-mono text-[10.5px] text-text-secondary">setup-process.log</span>
          </div>
          <span className="font-mono text-[10px] text-text-muted">Live output</span>
        </div>
        <pre
          ref={logRef}
          className="min-h-[140px] flex-1 overflow-y-auto p-4 font-mono text-[11.5px] leading-relaxed text-text-secondary select-text whitespace-pre-wrap sm:min-h-[180px]"
        >
          {job?.output || "Starting installation…"}
        </pre>
      </div>

      {job?.error && (
        <div className="rounded-xl border border-destructive/25 bg-danger-soft p-3.5">
          <p className="text-[12.5px] font-medium text-destructive">Setup failed: {job.error}</p>
          <p className="mt-1 text-[11px] text-text-muted">
            You can also finish later from the Packages page.
          </p>
          <Button
            size="sm"
            variant="outline"
            className="mt-2.5 cursor-pointer"
            onClick={() => window.location.reload()}
          >
            Reload wizard
          </Button>
        </div>
      )}
    </div>
  )
}
