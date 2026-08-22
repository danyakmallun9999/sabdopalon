import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import {
  Check,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  LoaderCircle,
  Rocket,
  Server,
  Sparkles,
} from "lucide-react"

import api, { type SetupJob } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Progress } from "@/components/ui/progress"
import { Separator } from "@/components/ui/separator"
import { Switch } from "@/components/ui/switch"

const STEPS = ["Pilihan stack", "Instalasi", "Selesai"]

export default function SetupPage() {
  const [step, setStep] = useState(0)
  const [installMariaDB, setInstallMariaDB] = useState(true)
  const [installPostgres, setInstallPostgres] = useState(false)
  const [createSample, setCreateSample] = useState(true)
  const [job, setJob] = useState<SetupJob | null>(null)
  const [progress, setProgress] = useState(10)
  const logRef = useRef<HTMLPreElement>(null)

  // Poll the setup job while it runs.
  useEffect(() => {
    if (!job?.running) {
      setProgress((p) => (job?.done ? 100 : p))
      return
    }
    const t = setInterval(() => {
      setProgress((p) => Math.min(p + 1.5 + Math.random() * 4, 95))
      api.setupJob().then(setJob).catch(() => {})
    }, 800)
    return () => clearInterval(t)
  }, [job?.running])

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
    if (job?.done && !job?.running) {
      if (job.error) {
        toast.error(job.error)
      } else {
        setStep(2)
        toast.success("Sabdopalon is ready! Reloading…")
        setTimeout(() => window.location.reload(), 1800)
      }
    }
  }, [job?.done, job?.running])

  async function start() {
    setProgress(5)
    setStep(1)
    const r = await api.runSetup({
      install_mariadb: installMariaDB,
      install_postgres: installPostgres,
      create_sample_site: createSample,
      db_engine: installMariaDB ? "mariadb" : "sqlite",
      http_port: 8080,
      https_port: 8443,
      tld: "localhost",
    })
    if (r.error) {
      toast.error(r.error)
      setStep(0)
      return
    }
    setJob({ running: true, done: false, output: "" })
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-6">
      {/* Header */}
      <div className="flex flex-col items-center gap-2 text-center">
        <div className="bg-primary/10 flex size-14 items-center justify-center rounded-2xl">
          <Sparkles className="text-primary size-7" />
        </div>
        <h2 className="text-2xl font-semibold">Selamat datang di Sabdopalon 🐫</h2>
        <p className="text-muted-foreground max-w-xl text-sm">
          Lingkungan development PHP lokal — semuanya tersimpan dalam satu folder, tanpa
          menyentuh sistem operasi kamu. Pilih stack, dan kami pasang semuanya dalam beberapa menit.
        </p>
      </div>

      {/* Stepper */}
      <ol className="flex items-center justify-center gap-2">
        {STEPS.map((label, i) => {
          const done = step > i || (step === 2 && i === 2)
          const active = step === i
          return (
            <li key={label} className="flex items-center gap-2">
              <span
                className={`flex size-7 items-center justify-center rounded-full text-xs font-medium ${
                  done
                    ? "bg-primary text-primary-foreground"
                    : active
                      ? "border-primary text-primary border-2"
                      : "bg-muted text-muted-foreground"
                }`}
              >
                {done ? <Check className="size-4" /> : i + 1}
              </span>
              <span className={`text-sm ${active ? "text-foreground font-medium" : "text-muted-foreground"}`}>
                {label}
              </span>
              {i < STEPS.length - 1 && <Separator className="w-8" />}
            </li>
          )
        })}
      </ol>

      {step === 0 && !job && (
        <>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Server className="size-4" /> Pilihan stack
              </CardTitle>
              <CardDescription>
                PHP + MariaDB adalah kombinasi klasik untuk WordPress dan Laravel. Semua bisa
                diubah nanti.
              </CardDescription>
            </CardHeader>
            <CardContent className="flex flex-col gap-3">
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="flex flex-col gap-0.5">
                  <Label htmlFor="setup-mariadb" className="font-medium">
                    MariaDB (database MySQL-compatible)
                  </Label>
                  <span className="text-muted-foreground text-xs">
                    Database bawaan untuk semua proyek PHP
                  </span>
                </div>
                <Switch
                  id="setup-mariadb"
                  checked={installMariaDB}
                  onCheckedChange={setInstallMariaDB}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="flex flex-col gap-0.5">
                  <Label htmlFor="setup-postgres" className="font-medium">
                    PostgreSQL <Badge variant="secondary" className="ml-1">opsional</Badge>
                  </Label>
                  <span className="text-muted-foreground text-xs">
                    Hanya jika kamu butuh PostgreSQL (Laravel/Node projects)
                  </span>
                </div>
                <Switch
                  id="setup-postgres"
                  checked={installPostgres}
                  onCheckedChange={setInstallPostgres}
                />
              </div>
              <div className="flex items-center justify-between rounded-lg border p-4">
                <div className="flex flex-col gap-0.5">
                  <Label htmlFor="setup-sample" className="font-medium">
                    Buat situs contoh
                  </Label>
                  <span className="text-muted-foreground text-xs">
                    Situs "myapp" langsung bisa dibuka setelah selesai
                  </span>
                </div>
                <Switch
                  id="setup-sample"
                  checked={createSample}
                  onCheckedChange={setCreateSample}
                />
              </div>
            </CardContent>
          </Card>

          <div className="flex items-center justify-between">
            <p className="text-muted-foreground max-w-sm text-xs">
              Unduhan diverifikasi (SHA-256) dan disimpan di dalam folder Sabdopalon — tidak ada
              instalasi ke sistem.
            </p>
            <Button size="lg" onClick={start}>
              <Rocket /> Mulai instalasi <ChevronRight />
            </Button>
          </div>
        </>
      )}

      {step >= 1 && (
        <Card>
          <CardHeader>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <CardTitle className="text-base">
                {step === 2 && job?.done && !job?.error ? (
                  <span className="inline-flex items-center gap-2 text-emerald-500">
                    <CheckCircle2 /> Sabdopalon siap digunakan!
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-2">
                    {job?.running && <LoaderCircle className="size-4 animate-spin" />}
                    Menyiapkan Sabdopalon…
                  </span>
                )}
              </CardTitle>
              <div className="flex items-center gap-2">
                <Progress value={progress} className="w-44" />
                <span className="text-muted-foreground text-xs tabular-nums">{progress}%</span>
              </div>
            </div>
            <pre
              ref={logRef}
              className="bg-background text-muted-foreground max-h-80 overflow-y-auto rounded-lg border p-4 font-mono text-xs whitespace-pre-wrap"
            >
              {job?.output || "Memulai…"}
            </pre>
            {job?.error && (
              <div className="bg-destructive/10 rounded-lg border border-destructive/30 p-3">
                <p className="text-destructive text-sm">
                  Setup gagal: {job.error}
                </p>
                <p className="text-muted-foreground mt-1 text-xs">
                  Kamu bisa mencoba lagi nanti lewat halaman Packages, atau jalankan ulang wizard.
                </p>
                <Button
                  size="sm"
                  variant="outline"
                  className="mt-2"
                  onClick={() => {
                    setJob(null)
                    setStep(0)
                    setProgress(10)
                  }}
                >
                  <ChevronLeft /> Kembali
                </Button>
              </div>
            )}
            {step === 2 && job?.done && !job?.error && (
              <div className="bg-emerald-500/10 mt-2 rounded-lg border border-emerald-500/30 p-3">
                <p className="text-sm">
                  Dashboard akan dimuat ulang — situs kamu siap dibuka di{" "}
                  <code className="bg-muted rounded px-1 py-0.5">http://myapp.localhost:8080</code>
                </p>
              </div>
            )}
          </CardHeader>
        </Card>
      )}
    </div>
  )
}
