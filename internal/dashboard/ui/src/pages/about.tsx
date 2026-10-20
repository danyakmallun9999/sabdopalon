import { useCallback, useEffect, useState } from "react"
import { Check, Copy, ExternalLink, FolderOpen, Heart, RefreshCw } from "lucide-react"

import api, { type Package } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ToneBadge } from "@/components/status"
import { useLive } from "@/lib/live"
import { PageActions } from "@/components/page-actions"
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
  logs?: { dir: string; files: number; bytes: number; newest: string }
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
async function invokeShell<T>(cmd: string, args?: Record<string, unknown>): Promise<T> {
  const tauri = (window as unknown as {
    __TAURI__?: { core?: { invoke?: <R>(c: string, a?: Record<string, unknown>) => Promise<R> } }
  }).__TAURI__
  if (!tauri?.core?.invoke) {
    throw new Error("Updating in place needs the desktop app — this is a plain browser.")
  }
  return tauri.core.invoke<T>(cmd, args)
}

/** True when this dashboard runs inside the desktop shell rather than a browser. */
function inDesktopShell(): boolean {
  return Boolean((window as unknown as { __TAURI__?: unknown }).__TAURI__)
}

/**
 * Open a folder with the OS file manager.
 *
 * Through the shell's own command rather than the opener plugin: that one is scoped to a
 * static set of paths, and this program's data root is whatever the user configured.
 */
async function openPath(path: string): Promise<void> {
  try {
    await invokeShell<void>("open_path", { path })
  } catch (e) {
    toast.error(e instanceof Error ? e.message : String(e))
  }
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
    <div className="flex items-start justify-between gap-3 border-b border-border-soft py-2 last:border-0">
      <div className="min-w-0">
        <div className="text-[13.5px] font-medium text-text-secondary">{label}</div>
        <div className="truncate font-mono text-[13.5px] text-text-primary" title={value}>
          {value || "—"}
        </div>
        {hint ? <div className="text-[13.5px] text-text-muted">{hint}</div> : null}
      </div>
      <div className="flex shrink-0 items-center">
        <Button variant="ghost" size="sm" onClick={copy} disabled={!value} aria-label={`Copy ${label}`}>
          {done ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
        </Button>
        {/* Hidden in a plain browser, where there is nothing to open with. */}
        {value && inDesktopShell() ? (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => void openPath(value)}
            aria-label={`Open ${label}`}
          >
            <FolderOpen className="size-3.5" />
          </Button>
        ) : null}
      </div>
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
  // Remembered in the browser: the channel is a preference of whoever is using this
  // dashboard, not a property of the installation, so it does not belong in config.
  const [channel, setChannel] = useState<string>(() => {
    try {
      return localStorage.getItem("sabdopalon.updateChannel") === "beta" ? "beta" : "stable"
    } catch {
      return "stable"
    }
  })
  const chooseChannel = useCallback((next: string) => {
    setChannel(next)
    try {
      localStorage.setItem("sabdopalon.updateChannel", next)
    } catch {
      // Storage refused: the choice still applies to this session.
    }
  }, [])
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
      const r = await api.checkUpdate(channel)
      setUpd(r)
      if (r.error) toast.error(r.error)
      else if (r.available) toast.info(`${r.latest} is available`)
      else toast.success("You are on the latest version")
    } finally {
      setChecking(false)
    }
  }, [channel])

  // On arrival, show what the last check found, and check once if nothing has been
  // checked in this run. The cache lives in the sidecar's memory, so it is the
  // once-per-run guard the design asked for — no setting needed to keep GitHub
  // requests rare, and the read itself never touches the network.
  useEffect(() => {
    let cancelled = false
    void (async () => {
      const last = await api.update()
      if (cancelled) return
      if (last.checked_at) {
        setUpd(last)
        return
      }
      await check()
    })()
    return () => {
      cancelled = true
    }
  }, [check])

  const version = info?.app.version ?? status?.version ?? "—"
  // One source for the repository URL, so a link can never point somewhere else.
  const repo = info?.app.repo ?? "https://github.com/danyakmallun9999/sabdopalon"

  // The kind is passed in rather than read from state: this callback outlives a
  // render, and a stale kind would pick the wrong way to apply an update.
  const startUpdate = useCallback(async (kind: string) => {
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
      if (kind === "installer") {
        await invokeShell<void>("apply_installer_update", { path: done.path })
        toast.success("Installer started — Sabdopalon will close and come back")
      } else {
        await invokeShell<string>("apply_sidecar_update", { path: done.path })
        toast.success("Update applied — Sabdopalon is restarting")
        setTimeout(() => window.location.reload(), 3000)
      }
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
    // Facts rather than an error count: what a maintainer needs first is whether a log
    // exists and whether it is recent, and a wrong error count is worse than none.
    `Logs: ${
      info?.logs
        ? `${info.logs.files} files, ${mb(info.logs.bytes)}, newest ${
            info.logs.newest ? new Date(info.logs.newest).toLocaleString() : "never"
          }`
        : "?"
    }`,
  ].join("\n")

  // A bug report is only useful with the details to hand, so the issue opens
  // prefilled with exactly the diagnostics this page shows — the same value, not a
  // second copy that could drift from what the user reads.
  const issueUrl =
    `${repo}/issues/new?title=${encodeURIComponent(`[${version}] `)}` +
    `&body=${encodeURIComponent(
      `What happened:\n\n\nWhat I expected:\n\n\nDiagnostics:\n\`\`\`\n${diagnostics}\n\`\`\`\n`,
    )}`

  return (
    <div className="flex h-full min-h-0 flex-col gap-5 overflow-auto px-5 py-7 lg:px-7">
      <PageActions>
          <Button variant="outline" size="sm" onClick={() => void load()} disabled={busy}>
            <RefreshCw className={busy ? "size-3.5 animate-pulse" : "size-3.5"} />
            Refresh
          </Button>
        </PageActions>

      {error ? (
        <Card className="border-destructive/40">
          <CardContent className="py-4 text-[13.5px] text-destructive">{error}</CardContent>
        </Card>
      ) : null}

      {/* Identity */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center gap-3">
            <div className="flex size-10 shrink-0 items-center justify-center rounded-xl border border-ring-inner bg-surface-raised p-1.5">
              <img src="/logo.png" alt="" className="size-full object-contain" />
            </div>
            <div className="flex flex-col gap-1">
              <div className="flex flex-wrap items-center gap-2">
                <CardTitle>Sabdopalon</CardTitle>
                {/* The channel you follow, not the channel this build came from. They are
                    different axes, and showing the build's would contradict the Updates card
                    for anyone following beta. */}
                <Badge variant="outline" className="text-[13.5px]">
                  {channel}
                </Badge>
              </div>
              <span className="font-mono text-[13.5px] text-text-muted">
                Version {version}
                {info ? ` · build ${info.app.runtime}` : ""}
              </span>
            </div>
          </div>
          <CardDescription>
            A local PHP, database and tunnel environment for Windows, Linux and macOS.
          </CardDescription>
        </CardHeader>
        <CardContent className="text-[13.5px]">
          <div className="grid gap-x-6 gap-y-1 sm:grid-cols-3">
            <div className="text-text-muted">Platform</div>
            <div className="font-mono text-[13.5px] text-text-primary">
              {info ? `${info.app.platform} · ${info.app.arch}` : "—"}
            </div>
            <div className="text-text-muted">Runtime</div>
            <div className="font-mono text-[13.5px] text-text-primary">{info?.app.runtime ?? "—"}</div>
            <div className="text-text-muted">Interface</div>
            <div>Desktop shell (Tauri) with a Go sidecar</div>
          </div>
        </CardContent>
      </Card>

      {/* Update. Checking is separated from applying on purpose: this tool holds
          the user's database and running servers, so it says a release exists and
          never installs one on its own. */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2">
            <RefreshCw className="size-4 text-text-muted" />
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
        <CardContent className="flex flex-col gap-3 text-[13.5px]">
          {upd?.available ? (
            <div className="flex flex-col gap-2">
              <div className="font-medium text-text-primary">
                {upd.kind === "sidecar"
                  ? "Updates in about 10 seconds, without closing Sabdopalon: the dashboard, proxy and CLI are replaced."
                  : "This one includes shell changes, so it installs like a normal update — Sabdopalon closes and reopens, roughly a minute. Sites and databases stop and come back."}
              </div>
              {upd.notes ? (
                <pre className="max-h-40 overflow-auto rounded-xl border border-ring-inner bg-surface-inset p-3 font-mono text-[13.5px] leading-relaxed text-text-secondary">
                  {upd.notes}
                </pre>
              ) : null}
            </div>
          ) : null}

          {upd?.error ? <div className="text-[13.5px] text-destructive">{upd.error}</div> : null}

          {upd?.available ? (
            <div className="flex flex-col gap-2">
              <div className="flex flex-wrap items-center gap-2">
                <Button
                  size="sm"
                  onClick={() => void startUpdate(upd.kind ?? "sidecar")}
                  disabled={job?.running || applying}
                >
                  {applying
                    ? upd.kind === "installer"
                      ? "Starting installer…"
                      : "Applying…"
                    : job?.running
                      ? "Downloading…"
                      : upd.kind === "installer"
                        ? "Download and install"
                        : "Update now"}
                </Button>
                <span className="text-[13.5px] leading-relaxed text-text-muted">
                  {upd.kind === "installer"
                    ? "The file is verified first, then Sabdopalon closes so the installer can replace it — roughly a minute."
                    : "The file is checked against the checksum the release published before anything is replaced."}
                </span>
              </div>

              {job && (job.running || job.done > 0) ? (
                <div className="flex flex-col gap-1">
                  <div
                    className="h-1.5 w-full overflow-hidden rounded-full bg-surface-raised"
                    role="presentation"
                  >
                    <div
                      className="h-full rounded-full bg-brand-fill transition-all"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className="text-[13.5px] text-text-muted">
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
              <span className="text-[13.5px] text-text-muted">
                Last checked {new Date(upd.checked_at).toLocaleString()} on{" "}
                {upd.channel ?? channel}
              </span>
            ) : null}
            {/* Stable follows /releases/latest, which GitHub keeps free of
                pre-releases; beta reads the list, pre-releases included. */}
            <div className="ml-auto inline-flex rounded-lg border border-ring-inner bg-surface-raised p-[3px]">
              {["stable", "beta"].map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => chooseChannel(c)}
                  className={`h-[24px] rounded-md border px-2.5 text-[13.5px] capitalize transition ${
                    channel === c
                      ? "border-[#3d3d3d] bg-[#333333] text-text-primary"
                      : "border-transparent text-text-muted hover:text-text-primary"
                  }`}
                >
                  {c}
                </button>
              ))}
            </div>
          </div>

          <p className="text-[13.5px] leading-relaxed text-text-muted">
            Sabdopalon never updates itself and never touches your sites, databases or the bundled
            stack. Only the program is replaced.
          </p>
        </CardContent>
      </Card>

      {/* Stack — what this installation is actually made of. */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle>What it is made of</CardTitle>
          <CardDescription>
            Bundled components and what is installed on this machine. Anything missing can be added
            from the Packages page.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-2 @xl/main:grid-cols-3">
          {pkgs.length === 0 ? (
            <span className="text-[13.5px] text-text-muted">No components reported yet.</span>
          ) : (
            pkgs.map((p) => (
              <div
                key={p.name}
                className="flex items-center justify-between gap-3 rounded-lg border border-ring-inner bg-surface-inset px-3 py-2"
              >
                <span className="truncate font-mono text-[13.5px] text-text-primary">
                  {p.name}
                </span>
                <span className="flex shrink-0 items-center gap-2">
                  <span className="font-mono text-[13.5px] text-text-muted">
                    {p.version}
                  </span>
                  {p.active ? (
                    <Badge variant="secondary" className="text-[13.5px]">
                      in use
                    </Badge>
                  ) : p.installed ? (
                    <ToneBadge tone="success">installed</ToneBadge>
                  ) : (
                    <ToneBadge tone="muted">missing</ToneBadge>
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
          <CardTitle className="flex items-center gap-2">
            <Heart className="size-4 text-text-muted" />
            Credits
          </CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-3 text-[13.5px]">
          <div>
            <div className="font-medium text-text-primary">Built by {info?.app.author ?? "danyakmallun"}</div>
            <a
              className="inline-flex items-center gap-1 font-mono text-[13.5px] text-brand hover:underline"
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
              onClick={() => window.open(issueUrl, "_blank", "noreferrer")}
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
          <p className="text-[13.5px] leading-relaxed text-text-muted">
            Licence terms ship with the repository. Bundled components keep their own licences —
            PHP, MariaDB, PostgreSQL, Cloudflare cloudflared, Mailpit, MinIO, Meilisearch, Node,
            Composer — and are downloaded from their own vendors.
          </p>
        </CardContent>
      </Card>

      {/* Paths */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle>Paths</CardTitle>
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
          <CardTitle>What it talks to</CardTitle>
          <CardDescription>
            No telemetry, no analytics, no account. These are the only outbound hosts, and each one
            is a download you asked for.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-1 text-[13.5px]">
          {(info?.privacy ?? []).map((h) => (
            <div key={h.host} className="flex flex-col border-b border-border-soft py-2 last:border-0 sm:flex-row sm:gap-3">
              <span className="min-w-64 font-mono text-[13.5px] text-text-primary">{h.host}</span>
              <span className="text-[13.5px] text-text-muted">{h.why}</span>
            </div>
          ))}
        </CardContent>
      </Card>

      {/* Diagnostics */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle>Diagnostics</CardTitle>
          <CardDescription>
            Paste this into a bug report — it is the first thing anyone would ask for.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <pre className="overflow-auto rounded-xl border border-ring-inner bg-surface-inset p-4 font-mono text-[13.5px] leading-relaxed text-text-secondary">
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
