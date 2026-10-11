import { useCallback, useEffect, useState } from "react"
import { Check, Copy, ExternalLink, Heart, Info, RefreshCw } from "lucide-react"

import api, { type Package } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { useLive } from "@/lib/live"
import { toast } from "sonner"

/**
 * About — what am I running, what is it doing to my machine, what does it talk to.
 *
 * Deliberately a reference page: nothing here polls, and nothing here changes
 * second to second. Live ports and daemon state belong to the Dashboard, and the
 * component list belongs to Packages, so each fact keeps one home.
 */

type AboutInfo = {
  app: {
    version: string
    channel: string
    platform: string
    arch: string
    runtime: string
    author: string
    repo: string
  }
  paths: Record<string, string>
  privacy: { host: string; why: string }[]
}

/** The shape the API returns for update checks — derived, so it cannot drift. */
type UpdateInfo = Awaited<ReturnType<typeof api.update>>
type DownloadState = Awaited<ReturnType<typeof api.downloadState>>

/**
 * The desktop shell's one command: replace the running sidecar and restart it.
 *
 * Only the shell can do this — the process being replaced is the one that would have
 * to do it — so the download and its checksum happen here in Go, and the swap is
 * asked for through the Tauri API the shell exposes to this dashboard origin.
 */
async function invokeApplySidecar(path: string): Promise<string> {
  const tauri = (window as unknown as {
    __TAURI__?: { core?: { invoke?: <T>(cmd: string, args?: Record<string, unknown>) => Promise<T> } }
  }).__TAURI__
  if (!tauri?.core?.invoke) {
    throw new Error("Updating in place needs the desktop app — this is a plain browser.")
  }
  return tauri.core.invoke<string>("apply_sidecar_update", { path })
}

/** Bytes as something a person reads without counting digits. */
function mb(bytes: number): string {
  if (!bytes) return "0 MB"
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/** Copies a value and says so, because a silent copy button is a guess. */
function CopyRow({ label, value, hint }: { label: string; value: string; hint?: string }) {
  const [done, setDone] = useState(false)
  const copy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value)
      setDone(true)
      setTimeout(() => setDone(false), 1500)
    } catch {
      toast.error("The clipboard refused the copy — select the text instead")
    }
  }, [value])

  return (
    <div className="flex items-start justify-between gap-3 border-b py-2 last:border-0">
      <div className="min-w-0">
        <div className="text-xs font-medium text-muted-foreground">{label}</div>
        <div className="truncate font-mono text-xs" title={value}>
          {value || "—"}
        </div>
        {hint ? <div className="text-xs text-muted-foreground">{hint}</div> : null}
      </div>
      <Button variant="ghost" size="sm" onClick={copy} disabled={!value} aria-label={`Copy ${label}`}>
        {done ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      </Button>
    </div>
  )
}

export default function AboutPage() {
  const [info, setInfo] = useState<AboutInfo | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [upd, setUpd] = useState<UpdateInfo | null>(null)
  const [checking, setChecking] = useState(false)
  const [pkgs, setPkgs] = useState<Package[]>([])
  const [job, setJob] = useState<DownloadState | null>(null)
  const [applying, setApplying] = useState(false)
  const { status } = useLive()

  const load = useCallback(async () => {
    setBusy(true)
    try {
      const r = await api.about()
      if ("error" in r && r.error) setError(String(r.error))
      else {
        setInfo(r as AboutInfo)
        setError(null)
      }
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  // Versions come from the same registry the Packages page manages, so the two can
  // never disagree about what is installed.
  useEffect(() => {
    void api
      .listPackages()
      .then((r) => {
        if (Array.isArray(r)) setPkgs(r)
      })
      .catch(() => {})
  }, [])

  const check = useCallback(async () => {
    setChecking(true)
    try {
      const r = await api.checkUpdate()
      setUpd(r)
      if (r.error) toast.error(r.error)
      else if (r.available) toast.info(`${r.latest} is available`)
      else toast.success("You are on the latest version")
    } finally {
      setChecking(false)
    }
  }, [])

  const version = info?.app.version ?? status?.version ?? "—"
  // One source for the repository URL, so a link can never point somewhere else.
  const repo = info?.app.repo ?? "https://github.com/danyakmallun9999/sabdopalon"

  const startUpdate = useCallback(async () => {
    const first = await api.downloadUpdate()
    if (first.error) {
      toast.error(first.error)
      return
    }
    setJob(first)

    // Poll until it stops running, then apply. Downloading and applying are separate
    // steps on purpose: the bytes are verified before anything is replaced.
    for (;;) {
      await new Promise((r) => setTimeout(r, 700))
      const s = await api.downloadState()
      setJob(s)
      if (s.error) {
        toast.error(s.error)
        return
      }
      if (!s.running) break
    }

    const done = await api.downloadState()
    if (!done.path) {
      toast.error("The download finished without a file to install.")
      return
    }

    setApplying(true)
    try {
      await invokeApplySidecar(done.path)
      toast.success("Update applied — Sabdopalon is restarting")
      setTimeout(() => window.location.reload(), 3000)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setApplying(false)
    }
  }, [])

  const pct = job && job.total > 0 ? Math.min(100, Math.round((job.done / job.total) * 100)) : 0
  const diagnostics = [
    `Sabdopalon ${version}${info ? ` (${info.app.channel})` : ""} · ${info?.app.platform ?? "?"} ${info?.app.arch ?? ""}`,
    `Go ${info?.app.runtime ?? "?"}`,
    `Data root: ${info?.paths.data_root ?? "?"}`,
    `Sites: ${info?.paths.sites ?? "?"}`,
    `Bin: ${info?.paths.bin ?? "?"}`,
    `PHP: ${info?.paths.php ?? "?"}`,
    `Dashboard port: ${window.location.port || "80"}`,
    `Proxy ports: ${status?.http_port ?? "?"} / ${status?.https_port ?? "?"}`,
  ].join("\n")

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-auto px-4 py-6 lg:px-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">About</h1>
          <p className="text-sm text-muted-foreground">
            Everything this installation is made of, and everything it talks to.
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={() => void load()} disabled={busy}>
          <RefreshCw className={busy ? "size-3.5 animate-pulse" : "size-3.5"} />
          Refresh
        </Button>
      </div>

      {error ? (
        <Card className="border-destructive/40">
          <CardContent className="py-4 text-sm text-destructive">{error}</CardContent>
        </Card>
      ) : null}

      {/* Identity */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Info className="size-4 text-muted-foreground" />
            Sabdopalon
            <Badge variant="secondary" className="font-mono text-xs">
              {version}
            </Badge>
            {info ? (
              <Badge variant="outline" className="text-xs">
                {info.app.channel}
              </Badge>
            ) : null}
          </CardTitle>
          <CardDescription>
            A local PHP, database and tunnel environment for Windows, Linux and macOS.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm">
          <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2">
            <div className="text-muted-foreground">Platform</div>
            <div className="font-mono text-xs">
              {info ? `${info.app.platform} · ${info.app.arch}` : "—"}
            </div>
            <div className="text-muted-foreground">Runtime</div>
            <div className="font-mono text-xs">{info?.app.runtime ?? "—"}</div>
            <div className="text-muted-foreground">Interface</div>
            <div>Desktop shell (Tauri) with a Go sidecar</div>
          </div>
        </CardContent>
      </Card>

      {/* Update. Checking is separated from applying on purpose: this tool holds
          the user's database and running servers, so it says a release exists and
          never installs one on its own. */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <RefreshCw className="size-4 text-muted-foreground" />
            Updates
          </CardTitle>
          <CardDescription>
            {upd?.available
              ? `${upd.latest} is available.`
              : upd
                ? `You are on the latest version (${upd.current}).`
                : "Check whether a newer release exists."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm">
          {upd?.available ? (
            <div className="flex flex-col gap-2">
              <div className="font-medium">
                {upd.kind === "sidecar"
                  ? "Updates in about 10 seconds, without closing Sabdopalon: the dashboard, proxy and CLI are replaced."
                  : "This one includes shell changes, so it installs like a normal update — Sabdopalon closes and reopens, roughly a minute. Sites and databases stop and come back."}
              </div>
              {upd.notes ? (
                <pre className="max-h-40 overflow-auto rounded-md border bg-muted/40 p-2 font-mono text-xs leading-relaxed">
                  {upd.notes}
                </pre>
              ) : null}
            </div>
          ) : null}

          {upd?.error ? <div className="text-destructive">{upd.error}</div> : null}

          {upd?.available ? (
            <div className="flex flex-col gap-2">
              {upd.kind === "sidecar" ? (
                <div className="flex flex-wrap items-center gap-2">
                  <Button
                    size="sm"
                    onClick={() => void startUpdate()}
                    disabled={job?.running || applying}
                  >
                    {applying ? "Applying…" : job?.running ? "Downloading…" : "Update now"}
                  </Button>
                  <span className="text-xs text-muted-foreground">
                    The file is checked against the checksum the release published before anything is
                    replaced.
                  </span>
                </div>
              ) : (
                <span className="text-xs text-muted-foreground">
                  This release changed the desktop shell, so it needs the installer — updating in place
                  is not available for it yet.
                </span>
              )}

              {job && (job.running || job.done > 0) ? (
                <div className="flex flex-col gap-1">
                  <div
                    className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
                    role="presentation"
                  >
                    <div
                      className="h-full bg-primary transition-all"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className="text-xs text-muted-foreground">
                    <span aria-live="polite">
                      {mb(job.done)}
                      {job.total > 0 ? ` of ${mb(job.total)} (${pct}%)` : ""}
                    </span>
                    {applying ? " — applying, Sabdopalon will restart" : ""}
                  </span>
                </div>
              ) : null}
            </div>
          ) : null}

          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => void check()} disabled={checking}>
              <RefreshCw className={checking ? "size-3.5 animate-pulse" : "size-3.5"} />
              {checking ? "Checking…" : "Check for updates"}
            </Button>
            {upd?.available && upd.url ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => window.open(upd.url, "_blank", "noreferrer")}
              >
                Open release page
              </Button>
            ) : null}
            {upd?.checked_at ? (
              <span className="text-xs text-muted-foreground">
                Last checked {new Date(upd.checked_at).toLocaleString()}
              </span>
            ) : null}
          </div>

          <p className="text-xs text-muted-foreground">
            Sabdopalon never updates itself and never touches your sites, databases or the bundled
            stack. Only the program is replaced.
          </p>
        </CardContent>
      </Card>

      {/* Stack — what this installation is actually made of. */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">What it is made of</CardTitle>
          <CardDescription>
            Bundled components and what is installed on this machine. Anything missing can be added
            from the Packages page.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-1 text-sm">
          {pkgs.length === 0 ? (
            <span className="text-muted-foreground">No components reported yet.</span>
          ) : (
            pkgs.map((p) => (
              <div
                key={p.name}
                className="flex items-center justify-between gap-3 border-b py-1.5 last:border-0"
              >
                <span className="font-mono text-xs">{p.name}</span>
                <span className="flex items-center gap-2">
                  <span className="font-mono text-xs text-muted-foreground">{p.version}</span>
                  {p.active ? (
                    <Badge variant="secondary" className="text-xs">
                      in use
                    </Badge>
                  ) : null}
                  {p.installed ? (
                    <span className="flex items-center gap-1 text-xs text-muted-foreground">
                      <Check className="size-3" />
                      installed
                    </span>
                  ) : (
                    <span className="text-xs text-muted-foreground">not installed</span>
                  )}
                </span>
              </div>
            ))
          )}
        </CardContent>
      </Card>

      {/* Credits — the author asked for these, and they belong somewhere visible. */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-base">
            <Heart className="size-4 text-muted-foreground" />
            Credits
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-sm">
          <div>
            <div className="font-medium">Built by {info?.app.author ?? "danyakmallun"}</div>
            <a
              className="inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
              href={info?.app.repo ?? "https://github.com/danyakmallun9999/sabdopalon"}
              target="_blank"
              rel="noreferrer"
            >
              {info?.app.repo ?? "https://github.com/danyakmallun9999/sabdopalon"}
              <ExternalLink className="size-3" />
            </a>
          </div>
          {/* Button has no asChild in this design system, so links open through
              window.open — same visual, and the anchor semantics were only cosmetic. */}
          <div className="flex flex-wrap gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => window.open(`${repo}/issues/new`, "_blank", "noreferrer")}
            >
              Report an issue
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => window.open(`${repo}/releases`, "_blank", "noreferrer")}
            >
              All releases
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => window.open(`${repo}/blob/main/CHANGELOG.md`, "_blank", "noreferrer")}
            >
              Changelog
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            Licence terms ship with the repository. Bundled components keep their own licences —
            PHP, MariaDB, PostgreSQL, Cloudflare cloudflared, Mailpit, MinIO, Meilisearch, Node,
            Composer — and are downloaded from their own vendors.
          </p>
        </CardContent>
      </Card>

      {/* Paths */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Paths</CardTitle>
          <CardDescription>Where this installation keeps everything.</CardDescription>
        </CardHeader>
        <CardContent>
          <CopyRow label="Data root" value={info?.paths.data_root ?? ""} />
          <CopyRow label="Sites" value={info?.paths.sites ?? ""} />
          <CopyRow label="Bundled tools" value={info?.paths.bin ?? ""} />
          <CopyRow label="PHP binary" value={info?.paths.php ?? ""} />
        </CardContent>
      </Card>

      {/* Privacy */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">What it talks to</CardTitle>
          <CardDescription>
            No telemetry, no analytics, no account. These are the only outbound hosts, and each one
            is a download you asked for.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-1 text-sm">
          {(info?.privacy ?? []).map((h) => (
            <div key={h.host} className="flex flex-col border-b py-1.5 last:border-0 sm:flex-row sm:gap-3">
              <span className="min-w-64 font-mono text-xs">{h.host}</span>
              <span className="text-xs text-muted-foreground">{h.why}</span>
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Diagnostics */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Diagnostics</CardTitle>
          <CardDescription>
            Paste this into a bug report — it is the first thing anyone would ask for.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <pre className="overflow-auto rounded-md border bg-muted/40 p-3 font-mono text-xs leading-relaxed">
            {diagnostics}
          </pre>
          <div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => {
                void navigator.clipboard
                  .writeText(diagnostics)
                  .then(() => toast.success("Diagnostics copied"))
                  .catch(() => toast.error("The clipboard refused the copy"))
              }}
            >
              <Copy className="size-3.5" />
              Copy diagnostics
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
