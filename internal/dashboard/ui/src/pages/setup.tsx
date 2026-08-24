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
          <main className="relative mx-auto w-full max-w-6xl flex-1 px-5 pt-5">
            {/* Brand — kecil, langsung di atas kartu pertama */}
            <div className="mb-4 flex items-center gap-2.5">
              <img
                src="/logo.png"
                alt="Sabdopalon"
                className="size-9 rounded-lg object-contain"
              />
              <span className="text-lg font-semibold tracking-tight">Sabdopalon</span>
            </div>

            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
              {/* LEFT — core + settings */}
              <div className="flex flex-col gap-4">
                <Card>
                  <CardHeader className="p-4 pb-2">
                    <CardTitle className="text-sm">Termasuk dalam paket</CardTitle>
                    <CardDescription className="text-xs">
                      Komponen inti selalu dipasang — status dibaca langsung dari folder instalasi.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="flex flex-col divide-y px-4 pb-3">
                    {(status?.components ?? []).map((c) => (
                      <div key={c.key} className="flex items-center justify-between py-2 first:pt-0 last:pb-0">
                        <div className="flex items-center gap-2.5">
                          <div className="bg-muted flex size-8 items-center justify-center rounded-lg">
                            {coreIcon(c.key)}
                          </div>
                          <div className="flex flex-col">
                            <span className="text-sm font-medium">{c.label}</span>
                            <span className="text-muted-foreground text-xs">
                              {c.installed ? (c.version ? `v${c.version}` : "terdeteksi") : "akan dipasang oleh wizard"}
                            </span>
                          </div>
                        </div>
                        {c.installed ? (
                          <Badge className="bg-emerald-500/15 text-emerald-600 dark:text-emerald-400" variant="secondary">
                            <CheckCircle2 className="size-3.5" /> Terpasang
                          </Badge>
                        ) : (
                          <Badge variant="outline" className="text-muted-foreground">
                            Termasuk paket
                          </Badge>
                        )}
                      </div>
                    ))}
                    {!status && <SkeletonRows rows={3} />}
                  </CardContent>
                </Card>

                <Card className="flex-1">
                  <CardHeader className="p-4 pb-2">
                    <CardTitle className="text-sm">Pengaturan</CardTitle>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-3 px-4 pb-4">
                    <label className="flex cursor-pointer items-start justify-between gap-3 rounded-lg border p-2.5">
                      <span className="flex flex-col gap-0.5">
                        <span className="text-sm font-medium">Buat situs contoh</span>
                        <span className="text-muted-foreground text-xs">
                          Situs "myapp" langsung bisa dibuka setelah selesai
                        </span>
                      </span>
                      <Checkbox checked={sample} onCheckedChange={(v) => setSample(v === true)} className="mt-0.5" />
                    </label>

                    <button
                      type="button"
                      onClick={() => setAdvanced((v) => !v)}
                      className="text-muted-foreground flex w-fit items-center gap-1 text-xs hover:underline"
                    >
                      <ChevronDown className={`size-3.5 transition-transform ${advanced ? "rotate-180" : ""}`} />
                      Pengaturan lanjutan (domain & port)
                    </button>
                    {advanced && (
                      <div className="grid grid-cols-1 gap-3 @lg/main:grid-cols-3">
                        <div className="flex flex-col gap-1.5">
                          <Label htmlFor="setup-tld" className="text-xs">
                            Domain lokal (*.…)
                          </Label>
                          <Input id="setup-tld" value={tld} onChange={(e) => setTld(e.target.value)} placeholder="localhost" />
                        </div>
                        <div className="flex flex-col gap-1.5">
                          <Label htmlFor="setup-http" className="text-xs">
                            Port HTTP
                          </Label>
                          <Input id="setup-http" inputMode="numeric" value={httpPort} onChange={(e) => setHttpPort(e.target.value)} />
                        </div>
                        <div className="flex flex-col gap-1.5">
                          <Label htmlFor="setup-https" className="text-xs">
                            Port HTTPS
                          </Label>
                          <Input id="setup-https" inputMode="numeric" value={httpsPort} onChange={(e) => setHttpsPort(e.target.value)} />
                        </div>
                      </div>
                    )}
                  </CardContent>
                </Card>
              </div>

              {/* RIGHT — optional tools */}
              <Card>
                <CardHeader className="p-4 pb-2">
                  <CardTitle className="text-sm">Tools tambahan</CardTitle>
                  <CardDescription className="text-xs">
                    Centang yang mau dipasang sekarang — sisanya bisa kapan saja lewat halaman Packages.
                  </CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-1.5 px-4 pb-4">
                  {availableTools.length === 0 && status && (
                    <p className="text-muted-foreground text-sm">Semua tools sudah terpasang 🎉</p>
                  )}
                  {availableTools.map((tool) => (
                    <ToolRow key={tool.key} tool={tool} checked={selected.has(tool.key)} onToggle={() => toggleTool(tool.key)} />
                  ))}
                  {!status && <SkeletonRows rows={4} />}

                  {activeTools.length > 0 && (
                    <div className="mt-2 flex flex-col gap-1.5">
                      <span className="text-muted-foreground text-xs font-medium uppercase tracking-wide">Sudah aktif</span>
                      <div className="flex flex-wrap gap-1.5">
                        {activeTools.map((t) => (
                          <Badge key={t.key} variant="secondary" className="bg-emerald-500/15 text-emerald-600 dark:text-emerald-400">
                            <CheckCircle2 className="size-3" /> {t.label}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}
                </CardContent>
              </Card>
            </div>
          </main>

          {/* Footer — CTA selalu terlihat tanpa scroll */}
          <footer className="bg-background/85 border-border/60 relative shrink-0 border-t px-5 py-3 backdrop-blur">
            <div className="mx-auto flex w-full max-w-6xl items-center justify-between gap-3">
              <p className="text-muted-foreground text-xs">
                {selected.size > 0
                  ? `${selected.size} tools tambahan akan diunduh + 3 komponen inti.`
                  : "Hanya 3 komponen inti yang akan dipersiapkan."}
              </p>
              <Button className="min-w-56" disabled={!status} onClick={start}>
                <Rocket /> Selesaikan persiapan <ArrowRight />
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
  return <Icon className="text-muted-foreground size-4" />
}

function ToolRow({ tool, checked, onToggle }: { tool: SetupTool; checked: boolean; onToggle: () => void }) {
  const Icon = TOOL_ICONS[tool.key] ?? Boxes
  return (
    <label
      className={`flex cursor-pointer items-start justify-between gap-3 rounded-lg border p-2.5 transition-colors ${
        checked ? "border-primary/50 bg-primary/5" : "hover:bg-muted/40"
      }`}
    >
      <span className="flex min-w-0 items-start gap-2.5">
        <div className="bg-muted mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg">
          <Icon className="text-muted-foreground size-3.5" />
        </div>
        <span className="flex min-w-0 flex-col">
          <span className="text-sm font-medium">{tool.label}</span>
          <span className="text-muted-foreground truncate text-xs">{tool.description}</span>
        </span>
      </span>
      <Checkbox checked={checked} onCheckedChange={onToggle} className="mt-1 shrink-0" aria-label={tool.label} />
    </label>
  )
}

function SkeletonRows({ rows }: { rows: number }) {
  return (
    <div className="flex flex-col gap-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="bg-muted h-11 animate-pulse rounded-lg" />
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
    <Card className="mx-auto flex min-h-0 w-full max-w-3xl flex-1 flex-col">
      <CardHeader className="flex min-h-0 flex-1 flex-col items-stretch gap-3 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <CardTitle className="text-base">
            {success ? (
              <span className="inline-flex items-center gap-2 text-emerald-500">
                <CheckCircle2 /> Sabdopalon siap digunakan!
              </span>
            ) : (
              <span className="inline-flex items-center gap-2">
                <LoaderCircle className="size-4 animate-spin" /> Menyiapkan Sabdopalon…
              </span>
            )}
          </CardTitle>
          <div className="flex items-center gap-2">
            <Progress value={progress} className="w-44" />
            <span className="text-muted-foreground text-xs tabular-nums">{Math.round(progress)}%</span>
          </div>
        </div>
        <pre
          ref={logRef}
          className="bg-background text-muted-foreground min-h-0 flex-1 overflow-y-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap"
        >
          {job?.output || "Memulai…"}
        </pre>
        {job?.error && (
          <div className="bg-destructive/10 rounded-lg border border-destructive/30 p-3">
            <p className="text-destructive text-sm">Setup gagal: {job.error}</p>
            <p className="text-muted-foreground mt-1 text-xs">
              Kamu bisa mencoba lagi dari sini, atau lanjutkan manual lewat halaman Packages nanti.
            </p>
            <Button
              size="sm"
              variant="outline"
              className="mt-2"
              onClick={() => window.location.reload()}
            >
              Muat ulang wizard
            </Button>
          </div>
        )}
        {success && (
          <div className="bg-emerald-500/10 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-emerald-500/30 p-3">
            <p className="text-sm">Semuanya sudah terpasang dan terkonfigurasi.</p>
            <Button size="sm" onClick={() => void reloadWhenReady()}>
              Masuk ke Dashboard <ArrowRight />
            </Button>
          </div>
        )}
      </CardHeader>
    </Card>
  )
}
