import { useEffect, useMemo, useRef, useState } from "react"
import { toast } from "sonner"
import { CheckCircle2, Download, Loader2, X, XCircle } from "lucide-react"

import api, {
  poll,
  type InstallJob,
  type Package,
  type SystemPHP,
  type SysTool,
  type SysToolJob,
} from "@/lib/api"
import { PageHeader } from "@/components/page-header"
import { ToneBadge } from "@/components/status"
import { Spinner } from "@/components/ui/spinner"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"


const LABELS: Record<string, string> = {
  php: "Default PHP (auto-used by all sites)",
  php81: "PHP 8.1",
  php82: "PHP 8.2",
  php83: "PHP 8.3",
  php84: "PHP 8.4",
  php85: "PHP 8.5",
  mariadb: "MariaDB server (auto-managed daemon)",
  mailpit: "Mailpit — local e-mail catcher",
  cloudflared: "cloudflared — public URLs for your sites (Cloudflare Tunnel)",
}

function SystemPHPCard() {
  const [sys, setSys] = useState<SystemPHP[]>([])
  useEffect(() => {
    const load = () => api.systemPHPs().then((d) => setSys(Array.isArray(d) ? d : [])).catch(() => {})
    poll(load, 10000)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div className="flex flex-col gap-1.5">
            <CardTitle>System PHP (your machine)</CardTitle>
            <CardDescription>
              Detected outside Sabdopalon's bin/. Default priority is system-first — pin a version
              per site via .sabdopalon.yml (php: "8.5").
            </CardDescription>
          </div>
        </div>
        {sys.length === 0 ? (
          <p className="text-[11.5px] text-text-muted">No system PHP found.</p>
        ) : (
          <div className="mt-2 flex flex-col gap-1.5">
            {sys.map((c) => (
              <div key={c.path} className="flex items-center justify-between gap-2 rounded-md border border-ring-inner bg-surface-inset px-3 py-2 text-[11.5px]">
                <span className="font-mono text-[10.5px] break-all text-text-secondary">{c.path}</span>
                {c.active ? (
                  <ToneBadge tone="success">default</ToneBadge>
                ) : (
                  <ToneBadge tone="muted">
                    PHP {c.version}
                  </ToneBadge>
                )}
              </div>
            ))}
          </div>
        )}
      </CardHeader>
    </Card>
  )
}

// SystemToolsCard manages per-user installs of Node.js & Composer onto the
// user's system (not into bin/). One-click install with live progress.
function SystemToolsCard() {
  const [tools, setTools] = useState<SysTool[]>([])
  const [job, setJob] = useState<SysToolJob | null>(null)
  const logRef = useRef<HTMLPreElement>(null)

  const load = () =>
    api.listSysTools().then((d) => setTools(Array.isArray(d) ? d : [])).catch(() => {})

  useEffect(() => {
    load()
    const t = poll(load, 8000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Poll the install job while it runs.
  useEffect(() => {
    if (!job?.running) return
    const t = setInterval(() => {
      api.sysToolJob().then((j) => {
        setJob(j)
        if (j.output) requestAnimationFrame(() => logRef.current?.scrollTo({ top: logRef.current.scrollHeight }))
      }).catch(() => {})
    }, 500)
    return () => clearInterval(t)
  }, [job?.running])

  useEffect(() => {
    if (job?.done && !job?.running) {
      if (job.error) toast.error(job.error)
      else toast.success(`${job.name} installed ✓`)
      load()
    }
  }, [job?.done, job?.running])

  async function install(name: string) {
    const r = await api.installSysTool(name)
    if (r.error) {
      toast.error(r.error)
      return
    }
    setJob({ name, running: true, done: false, output: "" })
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex flex-col gap-1.5">
          <CardTitle>System tools (Node.js &amp; Composer)</CardTitle>
          <CardDescription>
            Installed per-user onto your system (not <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[10.5px] text-text-secondary">bin/</code>), for
            Composer, npm and Vite.
          </CardDescription>
        </div>
        {tools.length === 0 ? (
          <Spinner />
        ) : (
          <div className="mt-2 flex flex-col gap-3">
            {tools.map((t) => (
              <div key={t.name} className="flex items-center justify-between gap-2 rounded-md border border-ring-inner bg-surface-inset px-3 py-2.5">
                <div className="flex flex-col gap-0.5">
                  <span className="text-[12px] font-medium text-text-primary">{t.label}</span>
                  {t.installed ? (
                    <span className="font-mono text-[10.5px] text-text-muted">{t.version}</span>
                  ) : (
                    <span className="text-[10.5px] text-text-muted">not installed yet</span>
                  )}
                </div>
                {t.installed ? (
                  <ToneBadge tone="success">
                    installed
                  </ToneBadge>
                ) : (
                  <Button size="sm" disabled={!!job?.running} onClick={() => install(t.name)}>
                    <Download className="size-4" />
                    Install
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
        {job && (
          <div className="mt-3 flex flex-col gap-2">
            <div className="flex items-center gap-2 text-[12px]">
              {job.running ? (
                <Loader2 className="size-3.5 animate-spin text-text-muted" />
              ) : job.error ? (
                <XCircle className="size-4 text-destructive" />
              ) : (
                <CheckCircle2 className="size-4 text-success" />
              )}
              <span>
                {job.running
                  ? `Installing ${job.name}…`
                  : job.error
                    ? `Failed to install ${job.name}`
                    : `${job.name} installed`}
              </span>
              {!job.running && (
                <Button variant="ghost" size="icon" className="size-7" onClick={() => setJob(null)} aria-label="Close">
                  <X />
                </Button>
              )}
            </div>
            <pre
              ref={logRef}
              className={`max-h-40 overflow-y-auto rounded-xl border border-ring-inner bg-surface-inset p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-text-secondary ${job.error ? "border-destructive/40" : ""}`}
            >
              {job.output || (job.running ? "starting…" : "")}
            </pre>
          </div>
        )}
      </CardHeader>
    </Card>
  )
}

export default function PackagesPage() {
  const [pkgs, setPkgs] = useState<Package[]>([])
  const [loadError, setLoadError] = useState<string | null>(null)
  const [job, setJob] = useState<InstallJob | null>(null)
  const [progress, setProgress] = useState(15)
  const logRef = useRef<HTMLPreElement>(null)

  // listPackages resolves with {error} instead of an array when the package
  // registry cannot be loaded — render that state instead of crashing on
  // pkgs.map().
  const load = () =>
    api
      .listPackages()
      .then((raw) => {
        const d = raw as Package[] | { error?: string }
        if (Array.isArray(d)) {
          setPkgs(d)
          setLoadError(null)
        } else {
          setLoadError(d.error ?? "Unexpected response from /api/packages")
        }
      })
      .catch(() => {})

  useEffect(() => {
    const t = poll(load, 6000)
    return () => clearInterval(t)
  }, [])

  // The registry has both a generic "[php]" entry (the default, resolved by
  // `sabdopalon add php`) and versioned "[php85]" entries. They often point
  // at the SAME artifact (same version, same SHA) — rendering both produces
  // two near-identical cards, which confused users ("which one do I install?").
  // De-duplicate PHP packages by short version: when the generic "php" and a
  // "phpNN" entry share a version, keep only one. We prefer the explicit
  // "phpNN" name (it reads as "PHP 8.5"), but carry over the "php" label so the
  // card still describes itself as the default.
  const deduped = useMemo(() => {
    const byShort = new Map<string, Package>()
    for (const p of pkgs) {
      if (!p.is_php) {
        byShort.set("__pkg::" + p.name, p)
        continue
      }
      const key = p.short
      const existing = byShort.get(key)
      // Prefer the explicit "phpNN" name; fall back to "php" only if absent.
      if (!existing || (!existing.name.startsWith("php8") && p.name.startsWith("php8"))) {
        byShort.set(key, p)
      }
    }
    return Array.from(byShort.values())
  }, [pkgs])

  // Poll the install job while it runs; update the live progress log.
  useEffect(() => {
    if (!job?.running) {
      setProgress((p) => (job?.done ? 100 : p))
      if (!job?.running) return
    }
    // Nudge the progress bar while running (the package manager reports no
    // numeric progress, so this is an indeterminate feel capped at 90%; it
    // jumps to 100 when done lands).
    const t = setInterval(() => {
      setProgress((p) => Math.min(p + 1 + Math.random() * 3, 90))
      api.installJob().then((j) => {
        setJob(j)
        // Auto-scroll the log to the bottom as new lines arrive.
        if (j.output) requestAnimationFrame(() => logRef.current?.scrollTo({ top: logRef.current.scrollHeight }))
      }).catch(() => {})
    }, 500)
    return () => clearInterval(t)
  }, [job?.running])

  useEffect(() => {
    if (job?.done && !job?.running) {
      if (job.error) toast.error(job.error)
      else toast.success(`Installed ✓`)
      load()
    }
  }, [job?.done, job?.running])

  async function install(name: string) {
    setProgress(10)
    const r = await api.installPackage(name)
    if (r.error) {
      toast.error(r.error)
      return
    }
    setJob({ name, running: true, done: false, output: "" })
  }

  return (
    <div className="flex flex-col gap-5 px-5 py-7 lg:px-7">
      <PageHeader
        title="Packages"
        description={
          <>
            Install runtimes and services inside <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[10.5px] text-text-secondary">bin/</code> — nothing touches your OS.
          </>
        }
      />

      <SystemPHPCard />

      <SystemToolsCard />

      {loadError && (
        <Card className="border-destructive/40">
          <CardHeader>
            <CardTitle className="text-destructive">Package registry failed to load</CardTitle>
            <CardDescription className="break-all">
              {loadError} — check the app log or re-run setup. The default registry is seeded automatically when
              file <code>packages/packages.toml</code> is missing.
            </CardDescription>
          </CardHeader>
        </Card>
      )}

      <div className="grid grid-cols-1 gap-3.5 @xl/main:grid-cols-2 @5xl/main:grid-cols-3">
        {deduped.map((p) => {
          // Status priority: active (in use now) > installed (bundled) >
          // not installed. "active" covers PHP supplied by the host system
          // that the "installed" check (bin/ only) does not see.
          const status = p.active ? "active" : p.installed ? "installed" : "available"
          // For the active PHP, show "Default PHP (auto-used by all sites)" so
          // it is obvious this is the one Sabdopalon runs; other PHP versions
          // show "PHP 8.x".
          const label = p.active && p.is_php
            ? LABELS["php"] ?? `PHP ${p.short}`
            : LABELS[p.name] ?? (p.is_php ? `PHP ${p.short}` : p.name)
          return (
            <Card key={p.name}>
              <CardHeader>
                <div className="flex items-start justify-between gap-2">
                  <div className="flex flex-col gap-1.5">
                    <CardTitle>{label}</CardTitle>
                    <CardDescription>
                      v{p.version}
                      {p.license ? ` · ${p.license}` : ""}
                    </CardDescription>
                  </div>
                  <ToneBadge tone={status === "active" ? "success" : "muted"}>
                    {status === "active" ? "active" : status === "installed" ? "installed" : "not installed"}
                  </ToneBadge>
                </div>
                <Button
                  className="mt-3 w-fit"
                  variant={status === "available" ? "default" : "secondary"}
                  disabled={status !== "available" || !!job?.running}
                  onClick={() => install(p.name)}
                >
                  {status === "available" ? <Download /> : <CheckCircle2 />}
                  {status === "available" ? "Install" : status === "active" ? "Active" : "Installed"}
                </Button>
              </CardHeader>
            </Card>
          )
        })}
      </div>

      {job && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                {job.running ? (
                  <Loader2 className="size-3.5 animate-spin text-text-muted" />
                ) : job.error ? (
                  <XCircle className="size-4 text-destructive" />
                ) : (
                  <CheckCircle2 className="size-4 text-success" />
                )}
                <CardTitle>
                  {job.running
                    ? `Installing ${job.name}…`
                    : job.error
                      ? `Failed to install ${job.name}`
                      : `${job.name} installed`}
                </CardTitle>
              </div>
              {!job.running && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-7"
                  onClick={() => setJob(null)}
                  aria-label="Close"
                >
                  <X />
                </Button>
              )}
            </div>
            {job.running && <Progress value={progress} className="w-full" />}
            <pre
              ref={logRef}
              className={`max-h-56 overflow-y-auto rounded-xl border border-ring-inner bg-surface-inset p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-text-secondary ${job.error ? "border-destructive/40" : ""}`}
            >
              {job.output || (job.running ? "starting…" : "")}
            </pre>
          </CardHeader>
        </Card>
      )}
    </div>
  )
}
