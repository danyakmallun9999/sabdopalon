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
      .catch(() => toast.error("Gagal memuat status instalasi"))
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

  async function start() {
    const hp = parseInt(httpPort, 10)
    const sp = parseInt(httpsPort, 10)
    if (!Number.isInteger(hp) || hp < 1 || hp > 65535 || !Number.isInteger(sp) || sp < 1 || sp > 65535) {
      toast.error("Port harus berupa angka 1–65535")
      return
    }
    if (!/^[a-z0-9.]+$/.test(tld.trim())) {
      toast.error("TLD hanya boleh berisi huruf kecil, angka, dan titik")
      return
    }
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
                <div className="flex size-9 shrink-0 items-center justify-center rounded-xl border border-border/80 bg-card p-1 shadow-2xs">
                  <img
                    src="/logo.png"
                    alt="Sabdopalon"
                    className="size-full object-contain"
                  />
                </div>
                <div className="flex flex-col">
                  <div className="flex items-center gap-2">
                    <h1 className="font-heading text-lg font-semibold tracking-tight text-foreground sm:text-xl">
                      Sabdopalon
                    </h1>
                    <Badge variant="outline" className="px-1.5 py-0 text-[10px] font-medium tracking-wide uppercase">
                      Setup
                    </Badge>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Konfigurasi server lokal dan pilih komponen untuk memulai lingkungan pengembangan.
                  </p>
                </div>
              </div>
            </div>

            <div className="grid grid-cols-1 items-start gap-4 lg:grid-cols-12">
              {/* LEFT — core + settings */}
              <div className="flex flex-col gap-4 lg:col-span-7">
                {/* Core components */}
                <Card size="sm" className="border-border/80 shadow-2xs">
                  <CardHeader className="pb-2.5">
                    <div className="flex items-center justify-between">
                      <div className="flex flex-col gap-0.5">
                        <CardTitle className="font-heading text-sm font-semibold">Komponen Inti</CardTitle>
                        <CardDescription className="text-xs">
                          Komponen wajib yang selalu dipersiapkan dalam lingkungan lokal.
                        </CardDescription>
                      </div>
                      <Badge variant="secondary" className="shrink-0 text-[11px] font-normal">
                        3 Komponen
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-2 pt-0">
                    {(status?.components ?? []).map((c) => (
                      <div
                        key={c.key}
                        className="flex items-center justify-between gap-3 rounded-lg border border-border/50 bg-muted/20 px-3 py-2 transition-colors hover:bg-muted/40"
                      >
                        <div className="flex min-w-0 items-center gap-2.5">
                          <div className="bg-background flex size-8 shrink-0 items-center justify-center rounded-lg border border-border/60 shadow-2xs">
                            {coreIcon(c.key)}
                          </div>
                          <div className="flex min-w-0 flex-col">
                            <span className="truncate text-xs font-medium text-foreground sm:text-sm">{c.label}</span>
                            <span className="truncate text-[11px] text-muted-foreground">
                              {c.installed
                                ? c.version
                                  ? `v${c.version}`
                                  : "terdeteksi"
                                : "akan dipasang oleh wizard"}
                            </span>
                          </div>
                        </div>
                        {c.installed ? (
                          <Badge className="shrink-0 gap-1 bg-emerald-500/15 text-[11px] text-emerald-600 dark:text-emerald-400" variant="secondary">
                            <CheckCircle2 className="size-3" /> Terpasang
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="shrink-0 text-[11px] text-muted-foreground">
                            Termasuk paket
                          </Badge>
                        )}
                      </div>
                    ))}
                    {!status && <SkeletonRows rows={3} />}
                  </CardContent>
                </Card>

                {/* Settings */}
                <Card size="sm" className="border-border/80 shadow-2xs">
                  <CardHeader className="pb-2.5">
                    <div className="flex flex-col gap-0.5">
                      <CardTitle className="font-heading text-sm font-semibold">Pengaturan</CardTitle>
                      <CardDescription className="text-xs">
                        Sesuaikan preferensi konfigurasi awal server lokal.
                      </CardDescription>
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-3 pt-0">
                    <label className="flex cursor-pointer items-start justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 p-2.5 transition-colors hover:bg-muted/40">
                      <span className="flex min-w-0 flex-col gap-0.5">
                        <span className="text-xs font-medium text-foreground sm:text-sm">Buat situs contoh</span>
                        <span className="text-[11px] text-muted-foreground sm:text-xs">
                          Situs <code className="bg-muted rounded px-1 py-0.5 font-mono text-[11px]">myapp</code> langsung bisa dibuka setelah selesai
                        </span>
                      </span>
                      <Checkbox checked={sample} onCheckedChange={(v) => setSample(v === true)} className="mt-0.5 shrink-0" />
                    </label>

                    <div className="flex flex-col gap-2 rounded-lg border border-border/60 bg-muted/10 p-2.5">
                      <button
                        type="button"
                        onClick={() => setAdvanced((v) => !v)}
                        className="text-foreground/80 hover:text-foreground flex w-full items-center justify-between text-xs font-medium transition-colors"
                      >
                        <span className="flex items-center gap-1.5">
                          <ChevronDown className={`size-3.5 transition-transform duration-200 ${advanced ? "rotate-180" : ""}`} />
                          Pengaturan lanjutan (domain & port)
                        </span>
                        <span className="text-[11px] text-muted-foreground font-normal">
                          {advanced ? "Sembunyikan" : "Sesuaikan"}
                        </span>
                      </button>

                      {advanced && (
                        <div className="grid grid-cols-1 gap-2.5 border-t border-border/40 pt-2 sm:grid-cols-3">
                          <div className="flex flex-col gap-1">
                            <Label htmlFor="setup-tld" className="text-[11px] font-medium text-muted-foreground">
                              Domain lokal (*.…)
                            </Label>
                            <Input
                              id="setup-tld"
                              value={tld}
                              onChange={(e) => setTld(e.target.value)}
                              placeholder="localhost"
                              className="h-8 text-xs font-mono"
                            />
                          </div>
                          <div className="flex flex-col gap-1">
                            <Label htmlFor="setup-http" className="text-[11px] font-medium text-muted-foreground">
                              Port HTTP
                            </Label>
                            <Input
                              id="setup-http"
                              inputMode="numeric"
                              value={httpPort}
                              onChange={(e) => setHttpPort(e.target.value)}
                              className="h-8 text-xs font-mono"
                            />
                          </div>
                          <div className="flex flex-col gap-1">
                            <Label htmlFor="setup-https" className="text-[11px] font-medium text-muted-foreground">
                              Port HTTPS
                            </Label>
                            <Input
                              id="setup-https"
                              inputMode="numeric"
                              value={httpsPort}
                              onChange={(e) => setHttpsPort(e.target.value)}
                              className="h-8 text-xs font-mono"
                            />
                          </div>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              </div>

              {/* RIGHT — optional tools */}
              <div className="flex flex-col gap-4 lg:col-span-5">
                <Card size="sm" className="border-border/80 shadow-2xs">
                  <CardHeader className="pb-2.5">
                    <div className="flex items-center justify-between">
                      <div className="flex flex-col gap-0.5">
                        <CardTitle className="font-heading text-sm font-semibold">Tools Tambahan</CardTitle>
                        <CardDescription className="text-xs">
                          Centang yang mau dipasang sekarang — sisanya bisa lewat Packages.
                        </CardDescription>
                      </div>
                      {selected.size > 0 && (
                        <Badge variant="default" className="shrink-0 text-[11px] font-normal">
                          {selected.size} Dipilih
                        </Badge>
                      )}
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-2 pt-0">
                    {availableTools.length === 0 && status && (
                      <div className="rounded-lg border border-border/50 bg-muted/20 p-3 text-center">
                        <p className="text-xs text-muted-foreground">Semua tools sudah terpasang 🎉</p>
                      </div>
                    )}
                    {availableTools.map((tool) => (
                      <ToolRow key={tool.key} tool={tool} checked={selected.has(tool.key)} onToggle={() => toggleTool(tool.key)} />
                    ))}
                    {!status && <SkeletonRows rows={4} />}

                    {activeTools.length > 0 && (
                      <div className="mt-2 flex flex-col gap-1.5 border-t border-border/50 pt-2.5">
                        <span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
                          Sudah Aktif
                        </span>
                        <div className="flex flex-wrap gap-1.5">
                          {activeTools.map((t) => (
                            <Badge key={t.key} variant="secondary" className="gap-1 bg-emerald-500/15 text-[11px] text-emerald-600 dark:text-emerald-400">
                              <CheckCircle2 className="size-3" /> {t.label}
                            </Badge>
                          ))}
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>
            </div>
          </main>

          {/* Footer — CTA selalu terlihat tanpa scroll */}
          <footer className="border-border/80 bg-background/95 sticky bottom-0 z-30 mt-auto shrink-0 border-t px-4 py-3 shadow-xs backdrop-blur-md sm:px-6">
            <div className="mx-auto flex w-full max-w-5xl items-center justify-between gap-3">
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span className="bg-primary/80 size-2 shrink-0 rounded-full animate-pulse" />
                <span className="text-[11px] sm:text-xs">
                  {selected.size > 0
                    ? `${selected.size} tools tambahan akan diunduh + 3 komponen inti.`
                    : "Hanya 3 komponen inti yang akan dipersiapkan."}
                </span>
              </div>
              <Button
                size="default"
                className="min-w-48 sm:min-w-56 font-medium gap-2 shadow-xs cursor-pointer"
                disabled={!status}
                onClick={start}
              >
                <Rocket className="size-4" /> Selesaikan persiapan <ArrowRight className="size-4" />
              </Button>
            </div>
          </footer>
        </>
      )}
    </div>
  )
}

// Poll until the full server (proxy actually bound) replaces the setup-mode
// instance, then reload. The desktop shell restarts the sidecar right after
// the wizard finishes — a blind reload here would land on a dead port.
// Bounded; falls back to a plain reload.
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
  window.location.reload()
}

function coreIcon(key: string) {
  const Icon = CORE_ICONS[key] ?? Server
  return <Icon className="size-4 text-muted-foreground" />
}

function ToolRow({ tool, checked, onToggle }: { tool: SetupTool; checked: boolean; onToggle: () => void }) {
  const Icon = TOOL_ICONS[tool.key] ?? Boxes
  return (
    <label
      className={`group flex cursor-pointer items-start justify-between gap-3 rounded-lg border p-2.5 transition-all select-none ${
        checked
          ? "border-primary/50 bg-primary/5 shadow-2xs"
          : "border-border/60 bg-card hover:border-border hover:bg-muted/40"
      }`}
    >
      <span className="flex min-w-0 items-start gap-2.5">
        <div
          className={`mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg border transition-colors ${
            checked ? "border-primary/30 bg-primary/10 text-primary" : "border-border/40 bg-muted text-muted-foreground"
          }`}
        >
          <Icon className="size-3.5" />
        </div>
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-xs font-medium text-foreground sm:text-sm">{tool.label}</span>
          <span className="mt-0.5 line-clamp-2 text-[11px] leading-relaxed text-muted-foreground sm:text-xs">
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
        <div key={i} className="bg-muted/60 h-11 animate-pulse rounded-lg border border-border/40" />
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
      <div className="flex flex-col gap-3 rounded-xl border border-border/80 bg-card p-3 shadow-2xs sm:p-3.5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5">
            {success ? (
              <div className="flex size-8 items-center justify-center rounded-lg bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="size-4.5" />
              </div>
            ) : (
              <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <LoaderCircle className="size-4.5 animate-spin" />
              </div>
            )}
            <div className="flex flex-col">
              <h2 className="font-heading text-sm font-semibold text-foreground sm:text-base">
                {success ? "Sabdopalon Siap Digunakan!" : "Menyiapkan Sabdopalon…"}
              </h2>
              <p className="text-xs text-muted-foreground">
                {success
                  ? "Semua komponen inti dan tools pilihan telah berhasil dikonfigurasi."
                  : "Mengunduh paket dan mengatur direktori instalasi..."}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2.5">
            <Progress value={progress} className="h-2 w-32 sm:w-44" />
            <span className="font-mono text-xs font-medium text-foreground tabular-nums">
              {Math.round(progress)}%
            </span>
          </div>
        </div>
      </div>

      {/* Terminal log panel */}
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-border/80 bg-zinc-950 shadow-sm dark:bg-black/90">
        <div className="flex items-center justify-between border-b border-zinc-800/80 bg-zinc-900/60 px-3.5 py-1.5">
          <div className="flex items-center gap-2">
            <div className="flex items-center gap-1.5">
              <span className="size-2 rounded-full bg-red-500/80" />
              <span className="size-2 rounded-full bg-amber-500/80" />
              <span className="size-2 rounded-full bg-emerald-500/80" />
            </div>
            <span className="ml-1.5 font-mono text-[11px] text-zinc-400">setup-process.log</span>
          </div>
          <span className="font-mono text-[10px] text-zinc-500">Live Output</span>
        </div>
        <pre
          ref={logRef}
          className="min-h-[140px] flex-1 overflow-y-auto p-3.5 font-mono text-xs leading-relaxed text-zinc-300 select-text whitespace-pre-wrap sm:min-h-[180px]"
        >
          {job?.output || "Memulai proses instalasi…"}
        </pre>
      </div>

      {job?.error && (
        <div className="bg-destructive/10 rounded-xl border border-destructive/30 p-3.5">
          <p className="text-destructive font-medium text-xs sm:text-sm">Setup gagal: {job.error}</p>
          <p className="text-muted-foreground mt-1 text-xs">
            Kamu bisa mencoba lagi dari sini, atau lanjutkan manual lewat halaman Packages nanti.
          </p>
          <Button
            size="sm"
            variant="outline"
            className="mt-2.5 cursor-pointer"
            onClick={() => window.location.reload()}
          >
            Muat ulang wizard
          </Button>
        </div>
      )}

      {success && (
        <div className="bg-emerald-500/10 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-emerald-500/30 p-3.5">
          <div className="flex flex-col gap-0.5">
            <p className="font-medium text-xs text-foreground sm:text-sm">Semuanya sudah terpasang dan terkonfigurasi.</p>
            <p className="text-xs text-muted-foreground">Server lokal siap digunakan untuk pengembangan aplikasi.</p>
          </div>
          <Button size="default" className="cursor-pointer gap-1.5" onClick={() => void reloadWhenReady()}>
            Masuk ke Dashboard <ArrowRight className="size-4" />
          </Button>
        </div>
      )}
    </div>
  )
}
