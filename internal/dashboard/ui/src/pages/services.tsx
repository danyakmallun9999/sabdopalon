import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Copy, CircleAlert, ExternalLink, Play, Square } from "lucide-react"

import api, { poll, type ServiceStatus } from "@/lib/api"
import { PageHeader } from "@/components/page-header"
import { StatusDot, ToneBadge } from "@/components/status"
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
        <pre className="max-h-56 overflow-y-auto rounded-xl border border-ring-inner bg-surface-inset p-4 font-mono text-[11.5px] leading-relaxed whitespace-pre-wrap text-text-secondary">
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

  // Runtime start/stop (does not touch the auto-start config).
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

  // Auto-start toggle (persists to config — service starts with the app).
  async function toggleAutoStart(svc: ServiceStatus, enabled: boolean) {
    setBusy(svc.name)
    try {
      const r = await api.toggleService(svc.name, enabled)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `Auto-start ${enabled ? "ON" : "OFF"} for ${svc.name}`)
      load()
    } finally {
      setBusy(null)
    }
  }

  const installed = services.filter((s) => s.installed)

  return (
    <div className="flex flex-col gap-5 px-5 py-7 lg:px-7">
      <PageHeader
        title="Services"
        description="Start or stop a tool now, or let it come up automatically when Sabdopalon opens."
      />

      {installed.length === 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>No services installed yet</CardTitle>
            <CardDescription>
              Install tools via the{" "}
              <a href="/packages" className="text-brand hover:underline">Packages</a>{" "}
              (Mailpit, Redis, MinIO, Meilisearch) — once installed, tools automatically
              start when Sabdopalon opens.
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
                    <CardTitle>{svc.label}</CardTitle>
                    <CardDescription>
                      {svc.ports?.length ? (
                        <span className="font-mono text-[10.5px]">{svc.ports.join("  ")}</span>
                      ) : null}
                    </CardDescription>
                  </div>
                  <ToneBadge tone={svc.running ? "success" : "muted"}>
                    <StatusDot status={svc.running ? "running" : "stopped"} />
                    {svc.running ? "running" : "stopped"}
                  </ToneBadge>
                </div>

                <div className="mt-2 flex flex-col gap-2">
                  {svc.running && svc.ui ? (
                    <a
                      href={svc.ui}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 text-[11.5px] text-brand hover:underline"
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

                  <div className="flex items-center justify-between rounded-md border border-ring-inner bg-surface-inset px-3 py-2.5">
                    <Label htmlFor={`svc-${svc.name}`} className="font-normal">
                      Auto-start when the app opens
                    </Label>
                    <Switch
                      id={`svc-${svc.name}`}
                      checked={!!svc.enabled}
                      disabled={busy === svc.name}
                      onCheckedChange={(v) => toggleAutoStart(svc, v)}
                    />
                  </div>

                  {svc.last_error && !svc.running && (
                    <p className="flex items-center gap-1 text-[11px] text-destructive">
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
