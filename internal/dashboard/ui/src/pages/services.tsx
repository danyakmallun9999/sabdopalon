import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Copy, CircleAlert, ExternalLink, Play, Square } from "lucide-react"

import api, { poll, type ServiceStatus } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"

// Laravel .env suggestions for each service (from the legacy web/ page).
const LARAVEL_ENV: Record<string, string> = {
  mailpit: `MAIL_MAILER=smtp
MAIL_HOST=127.0.0.1
MAIL_PORT=1025`,
  redis: `CACHE_STORE=redis
QUEUE_CONNECTION=redis
REDIS_CLIENT=predis
REDIS_HOST=127.0.0.1
REDIS_PORT=6379`,
  minio: `FILESYSTEM_DISK=s3
AWS_ENDPOINT=http://127.0.0.1:9000
AWS_ACCESS_KEY_ID=sabdopalon
AWS_SECRET_ACCESS_KEY=sabdopalon-secret
AWS_DEFAULT_REGION=us-east-1
AWS_BUCKET=sabdopalon-bucket`,
  meilisearch: `SCOUT_DRIVER=meilisearch
MEILISEARCH_HOST=http://127.0.0.1:7700`,
}

function EnvSnippet({ services }: { services: ServiceStatus[] }) {
  const running = services.filter((s) => s.running && LARAVEL_ENV[s.name])
  const [tab, setTab] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const current = tab && running.some((s) => s.name === tab) ? tab : running[0]?.name

  useEffect(() => {
    if (!current) setCopied(false)
  }, [current])

  if (!current) return null

  const snippet = LARAVEL_ENV[current]
  return (
    <Card>
      <CardHeader>
        <CardTitle>Laravel .env suggestions</CardTitle>
        <CardDescription>
          Add these to your app&apos;s .env to use the running service. Click to copy.
        </CardDescription>
        {running.length > 1 && (
          <Tabs value={current} onValueChange={setTab}>
            <TabsList>
              {running.map((s) => (
                <TabsTrigger key={s.name} value={s.name}>
                  {s.label.split(" — ")[0]}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>
        )}
        <pre className="bg-background text-muted-foreground max-h-56 overflow-y-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap">
          {snippet}
        </pre>
        <Button
          className="mt-3 w-fit"
          size="sm"
          variant="outline"
          onClick={async () => {
            await navigator.clipboard.writeText(snippet)
            setCopied(true)
            setTimeout(() => setCopied(false), 1500)
          }}
        >
          <Copy /> {copied ? "Copied ✓" : "Copy .env snippet"}
        </Button>
      </CardHeader>
    </Card>
  )
}

export default function ServicesPage() {
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [busy, setBusy] = useState<string | null>(null)

  const load = () =>
    api.services().then((d) => setServices(d.services || [])).catch(() => {})

  useEffect(() => {
    const t = poll(load, 5000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Runtime start/stop (tidak menyentuh config auto-start).
  async function start(svc: ServiceStatus) {
    setBusy(svc.name)
    try {
      const r = await api.startService(svc.name)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `${svc.name}: start`)
      load()
    } finally {
      setBusy(null)
    }
  }

  async function stop(svc: ServiceStatus) {
    setBusy(svc.name)
    try {
      const r = await api.stopService(svc.name)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `${svc.name}: stop`)
      load()
    } finally {
      setBusy(null)
    }
  }

  // Auto-start toggle (persist ke config — service ikut nyala saat app dibuka).
  async function toggleAutoStart(svc: ServiceStatus, enabled: boolean) {
    setBusy(svc.name)
    try {
      const r = await api.toggleService(svc.name, enabled)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `Auto-start ${enabled ? "ON" : "OFF"} untuk ${svc.name}`)
      load()
    } finally {
      setBusy(null)
    }
  }

  const installed = services.filter((s) => s.installed)

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <p className="text-muted-foreground text-sm">
        Layanan tambahan — <strong>Start/Stop</strong> menjalankan/menghentikan tool saat itu
        juga; <strong>Auto-start</strong> menentukan apakah tool ikut nyala saat Sabdopalon
        dibuka.
      </p>

      {installed.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Belum ada layanan terinstall</CardTitle>
            <CardDescription>
              Pasang tool lewat halaman{" "}
              <a href="/packages" className="text-primary hover:underline">Packages</a>{" "}
              (Mailpit, Redis, MinIO, Meilisearch) — begitu terinstall, tool otomatis ikut
              nyala saat Sabdopalon dibuka.
            </CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-2">
          {installed.map((svc) => (
            <Card key={svc.name}>
              <CardHeader>
                <div className="flex items-start justify-between gap-2">
                  <div className="flex flex-col gap-1.5">
                    <CardTitle className="text-base">{svc.label}</CardTitle>
                    <CardDescription>
                      {svc.ports?.length ? (
                        <span className="font-mono text-xs">{svc.ports.join("  ")}</span>
                      ) : null}
                    </CardDescription>
                  </div>
                  <Badge variant={svc.running ? "default" : "outline"}>
                    {svc.running ? "running" : "stopped"}
                  </Badge>
                </div>

                <div className="mt-2 flex flex-col gap-2">
                  {svc.running && svc.ui ? (
                    <a
                      href={svc.ui}
                      target="_blank"
                      rel="noreferrer"
                      className="text-primary hover:underline inline-flex items-center gap-1 text-sm"
                    >
                      Open UI <ExternalLink className="size-3.5" />
                    </a>
                  ) : null}

                  <div className="flex items-center gap-2">
                    {svc.running ? (
                      <Button size="sm" variant="outline" disabled={busy === svc.name} onClick={() => stop(svc)}>
                        <Square /> Stop
                      </Button>
                    ) : (
                      <Button size="sm" disabled={busy === svc.name} onClick={() => start(svc)}>
                        <Play /> Start
                      </Button>
                    )}
                  </div>

                  <div className="flex items-center justify-between rounded-lg border p-3">
                    <Label htmlFor={`svc-${svc.name}`} className="font-normal">
                      Auto-start saat aplikasi dibuka
                    </Label>
                    <Switch
                      id={`svc-${svc.name}`}
                      checked={!!svc.enabled}
                      disabled={busy === svc.name}
                      onCheckedChange={(v) => toggleAutoStart(svc, v)}
                    />
                  </div>

                  {svc.last_error && !svc.running && (
                    <p className="text-destructive flex items-center gap-1 text-xs">
                      <CircleAlert className="size-3" /> {svc.last_error}
                    </p>
                  )}
                </div>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}

      <EnvSnippet services={services} />
    </div>
  )
}
