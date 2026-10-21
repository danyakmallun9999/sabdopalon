import { Fragment, useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import {
  Box,
  CheckCircle2,
  Copy,
  EllipsisVerticalIcon,
  Eraser,
  ExternalLink,
  Globe,
  Link as LinkIcon,
  Loader2,
  Lock,
  PanelRight,
  Play,
  Plus,
  RotateCw,
  ScrollText,
  Search,
  Settings2,
  Share2,
  Square,
  TerminalSquare,
  Trash2,
  XCircle,
} from "lucide-react"

import api, {
  poll,
  type CreateJob,
  type Site,
  type SiteConfigPayload,
} from "@/lib/api"
import TerminalPanel, {
  type TermStatus,
  type TerminalPanelHandle,
} from "@/components/terminal-panel"
import { useLive } from "@/lib/live-context"
import { Badge } from "@/components/ui/badge"
import { PageActions } from "@/components/page-actions"
import { StatusDot, ToneBadge, type DotStatus } from "@/components/status"
import { IconButton } from "@/components/icon-button"
import { Empty, EmptyTitle } from "@/components/ui/empty"
import { Button } from "@/components/ui/button"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

const TEMPLATES = [
  { id: "blank", label: "Blank PHP" },
  { id: "laravel", label: "Laravel (plain skeleton)" },
  // The same starter kits `laravel new` offers: four stacks, each with and
  // without authentication scaffolding.
  { id: "laravel-react", label: "Laravel + React starter kit (auth)" },
  { id: "laravel-vue", label: "Laravel + Vue starter kit (auth)" },
  { id: "laravel-svelte", label: "Laravel + Svelte starter kit (auth)" },
  { id: "laravel-livewire", label: "Laravel + Livewire starter kit (auth)" },
  { id: "laravel-react-blank", label: "Laravel + React (no auth)" },
  { id: "laravel-vue-blank", label: "Laravel + Vue (no auth)" },
  { id: "laravel-svelte-blank", label: "Laravel + Svelte (no auth)" },
  { id: "laravel-livewire-blank", label: "Laravel + Livewire (no auth)" },
  { id: "wordpress", label: "WordPress (downloads latest)" },
  { id: "codeigniter", label: "CodeIgniter 4 (needs composer)" },
]

const DOCK_KEY = "sabdopalon.sites.terminalDock" // { open, width }

type DockState = { open: boolean; width: number }

function loadDock(): DockState {
  try {
    const raw = localStorage.getItem(DOCK_KEY)
    if (raw) {
      const d = JSON.parse(raw) as DockState
      return { open: !!d.open, width: Math.min(Math.max(d.width || 480, 300), 900) }
    }
  } catch {
    /* fresh */
  }
  // Terminal is closed by default on all screen sizes — opened via
  // the Terminal button in the header. The user's choice is remembered via localStorage.
  return { open: false, width: 480 }
}

function TermStatusLabel({ s }: { s: TermStatus }) {
  const dot: DotStatus =
    s === "connected" ? "running" : s === "connecting" ? "connecting" : "error"
  return (
    <span className="text-muted-foreground inline-flex items-center gap-1.5 text-[13px]">
      <StatusDot status={dot} />
      {s}
    </span>
  )
}

export default function SitesPage() {
  const { status } = useLive()
  const navigate = useNavigate()
  const [sites, setSites] = useState<Site[]>([])
  const [busy, setBusy] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Site | null>(null)
  const [cfgSite, setCfgSite] = useState<Site | null>(null)
  const [search, setSearch] = useState("")

  // --- terminal dock state ---
  const [dock, setDock] = useState<DockState>(loadDock)
  const [termSite, setTermSite] = useState<Site | null>(null) // null = sites/ root
  const [termStatus, setTermStatus] = useState<TermStatus>("connecting")
  const [mobileTermHeight, setMobileTermHeight] = useState(320)
  const [isDesktop, setIsDesktop] = useState(() => window.innerWidth >= 1024)
  const draggingRef = useRef(false)
  const panelRef = useRef<TerminalPanelHandle>(null)

  useEffect(() => {
    localStorage.setItem(DOCK_KEY, JSON.stringify(dock))
  }, [dock])

  const [cfg, setCfg] = useState<SiteConfigPayload>({ php: "", php_ini: "", docroot: "", aliases: [], env: {} })
  const [phpOptions, setPhpOptions] = useState<{ value: string; label: string; group: string }[]>([])
  const [savingCfg, setSavingCfg] = useState(false)

  // New-site dialog state
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [template, setTemplate] = useState("blank")
  // The same two choices `laravel new` asks about. MariaDB is worth offering
  // because Sabdopalon creates the database and writes the credentials itself.
  const [database, setDatabase] = useState("sqlite")
  const [testing, setTesting] = useState("pest")
  // Site creation is a job the dialog watches: `createState` is its last snapshot,
  // and it survives closing the dialog so a long composer run is never lost.
  const [createState, setCreateState] = useState<CreateJob | null>(null)
  const [elapsed, setElapsed] = useState(0)
  const [rawLog, setRawLog] = useState(false)
  const logRef = useRef<HTMLPreElement>(null)

  const filtered = sites.filter((s) =>
    search.trim()
      ? (s.name + " " + (s.url ?? "") + " " + (s.aliases ?? []).join(" "))
          .toLowerCase()
          .includes(search.trim().toLowerCase())
      : true,
  )

  const load = () => api.listSites().then(setSites).catch(() => {})

  useEffect(() => {
    const t = poll(load, 4000)
    return () => clearInterval(t)
  }, [])

  useEffect(() => {
    const onResize = () => setIsDesktop(window.innerWidth >= 1024)
    window.addEventListener("resize", onResize)
    return () => window.removeEventListener("resize", onResize)
  }, [])

  function aliasTarget(site: Site, alias: string): string {
    try {
      const u = new URL(site.url)
      u.hostname = alias
      return u.toString()
    } catch {
      return `http://${alias}/`
    }
  }


  async function openConfigure(site: Site) {
    setCfgSite(site)
    const opts: { value: string; label: string; group: string }[] = [
      { value: "", label: `Default (${(status?.php_version ?? "system-first")})`, group: "General" },
    ]
    try {
      const pkgs = await api.listPackages()
      // Array.isArray, not a bare .filter: a failed or older sidecar can answer
      // with an error object or `null` (a machine with no system PHP is a normal
      // answer), and a throw here left the select showing nothing but its group
      // headings — which reads as "there is no way to choose a PHP version".
      if (Array.isArray(pkgs)) {
        pkgs
          .filter((p) => p.is_php)
          .forEach((p) => opts.push({ value: p.short, label: `${p.short}${p.installed ? "" : " (not downloaded)"}`, group: "Bundled" }))
      }
      const sys = await api.systemPHPs()
      if (Array.isArray(sys)) {
        sys.forEach((c) => opts.push({ value: c.path, label: `${c.version} — ${c.path}`, group: "System" }))
      }
    } catch {
      /* the default entry is still offered */
    }
    setPhpOptions(opts)

    const cfgData = await api.siteConfig(site.name)
    if (!("error" in (cfgData as object))) {
      setCfg({
        php: cfgData.php ?? "",
        php_ini: cfgData.php_ini ?? "",
        docroot: cfgData.docroot ?? "",
        aliases: cfgData.aliases ?? [],
        env: cfgData.env ?? {},
      })
    }
  }

  async function saveConfigure() {
    if (!cfgSite) return
    setSavingCfg(true)
    try {
      const payload: SiteConfigPayload = { ...cfg }
      const r = await api.saveSiteConfig(cfgSite.name, payload)
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? "Saved")
      setCfgSite(null)
      load()
    } finally {
      setSavingCfg(false)
    }
  }

  // A value can be stored and still not be in the list: a version that is not
  // downloaded, a binary path that moved, or a hand-edited file. The select
  // then falls back to its placeholder, which reads as "it went back to
  // Default" — show what is actually in the file as its own entry instead.
  const phpChoices =
    cfg.php && !phpOptions.some((o) => o.value === cfg.php)
      ? [...phpOptions, { value: cfg.php, label: `${cfg.php} (as configured)`, group: "General" }]
      : phpOptions

  async function copyHostsLine(alias: string) {
    await navigator.clipboard.writeText(`127.0.0.1 ${alias}`).catch(() => {})
    toast.info("Hosts line copied", {
      description: "Paste it into /etc/hosts (Linux/macOS) or C:\\Windows\\System32\\drivers\\etc\\hosts (Windows), then reload.",
    })
  }

  async function act(site: Site, action: "start" | "stop" | "restart") {
    setBusy(site.name + action)
    try {
      const r = await api.siteAction(site.name, action)
      if (r.error) toast.error(r.error)
      else if (action === "restart") toast.success(`${site.name}: restarted`)
    } finally {
      setBusy(null)
      load()
    }
  }

  // Watch a running create job. This is deliberately not tied to the dialog being
  // open: closing it means "let it run", not "stop telling me" — the log keeps
  // filling in, the elapsed time keeps counting, and the toast still arrives.
  useEffect(() => {
    if (!createState?.running) return
    const t = setInterval(() => {
      api
        .createJob()
        .then((j) => {
          setCreateState(j)
          if (j.started) setElapsed(Math.max(0, Math.round((Date.now() - Date.parse(j.started)) / 1000)))
          if (j.output) requestAnimationFrame(() => logRef.current?.scrollTo({ top: logRef.current.scrollHeight }))
        })
        .catch(() => {})
    }, 500)
    return () => clearInterval(t)
  }, [createState?.running])

  // One completion handler for both cases (dialog open or closed).
  useEffect(() => {
    if (!createState?.done || createState.running) return
    if (createState.error) toast.error(`${createState.name} could not be created`, { description: createState.error })
    else toast.success(`Site "${createState.name}" created`, { description: createState.url })
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [createState?.done, createState?.running])

  // Opening the dialog should show a job that is already running — after a page
  // reload, or after the dialog was closed and reopened. A finished job from last
  // time is history, not state: without clearing it, "New Site" would reopen on the
  // previous result and offer nothing but Close.
  async function openCreate() {
    setOpen(true)
    setCreateState((s) => (s && !s.running ? null : s))
    try {
      const j = await api.createJob()
      if (j.running) {
        setCreateState(j)
        if (j.started) setElapsed(Math.max(0, Math.round((Date.now() - Date.parse(j.started)) / 1000)))
      }
    } catch {
      /* an idle dashboard answers with an empty job; nothing to show */
    }
  }

  async function create() {
    if (!name.trim()) return
    const r = await api.createSite(name.trim().toLowerCase(), template, database, testing)
    if (r.error) {
      toast.error(r.error)
      return
    }
    if (r.job) setCreateState(r.job)
    setElapsed(0)
    setName("")
  }

  async function doDelete() {
    if (!deleteTarget) return
    const r = await api.deleteSite(deleteTarget.name)
    if (r.error) toast.error(r.error)
    else toast.success(r.message ?? "Moved to trash")
    if (termSite?.name === deleteTarget.name) setTermSite(null)
    setDeleteTarget(null)
    load()
  }

  // No email field on this page, so a tunnel started from the row menu is
  // open to anyone; the site page is where an allow-list can be set.
  async function toggleShare(s: Site) {
    const shared = !!status?.tunnels?.[s.name]?.enabled
    setBusy(s.name + "tunnel")
    try {
      const r = shared
        ? await api.stopSiteTunnel(s.name)
        : await api.startSiteTunnel(s.name, [])
      if (r.error) toast.error(r.error)
      else if (r.url) toast.success(r.url)
    } finally {
      setBusy(null)
    }
  }

  function openTerminalFor(site: Site | null) {
    setTermSite(site)
    setDock((d) => ({ ...d, open: true }))
  }

  /* ------------------------------ site rows ------------------------------ */

  const urlCell = (s: Site) => (
    <div className="flex flex-col gap-1">
      <a
        href={s.url}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1.5 text-[13.5px] text-brand hover:underline"
      >
        <Globe className="size-3.5 shrink-0" />
        {s.url.replace(/^https?:\/\//, "").replace(/\/$/, "")}
        <ExternalLink className="size-3 opacity-60" />
      </a>
      <a
        href={s.https}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1.5 text-[13.5px] text-text-secondary hover:text-text-primary hover:underline"
      >
        <Lock className="size-3.5 shrink-0 text-success" />
        {s.https.replace(/^https?:\/\//, "").replace(/\/$/, "")}
        <ExternalLink className="size-3 opacity-60" />
      </a>
      {(s.aliases ?? []).length > 0 && (
        <div className="mt-1 flex flex-wrap gap-1.5">
          {(s.aliases ?? []).map((a) => {
            const resolvable = a.endsWith("." + (status?.tld ?? "localhost"))
            return resolvable ? (
              <a key={a} href={aliasTarget(s, a)} target="_blank" rel="noreferrer" title={`Open http://${a}`}>
                <Badge variant="secondary" className="cursor-pointer hover:bg-secondary/80">
                  <LinkIcon className="size-3" /> {a}
                </Badge>
              </a>
            ) : (
              <Badge
                key={a}
                variant="outline"
                className="cursor-pointer"
                title="Custom domain — click to copy the /etc/hosts line"
                onClick={() => copyHostsLine(a)}
              >
                <Copy className="size-3" /> {a}
              </Badge>
            )
          })}
        </div>
      )}
    </div>
  )

  // Both state cells render a Badge, every time. A bare <span> for the
  // "default"/"stopped" case and a Badge for the other gave the two columns
  // different font sizes and heights — that mismatch is what made the
  // right-hand side of the table look ragged rather than aligned.
  const phpBadge = (s: Site) =>
    s.php ? (
      <ToneBadge tone="warning">PHP {s.php}</ToneBadge>
    ) : (
      <ToneBadge tone="muted">PHP default</ToneBadge>
    )

  const runBadge = (s: Site) => (
    <ToneBadge tone={s.running ? "success" : "muted"}>
      <StatusDot status={s.running ? "running" : "stopped"} />
      {s.running ? "running" : "stopped"}
    </ToneBadge>
  )

  // Sharing is a live fact, not something this page asked for, so the badge is
  // driven entirely by the snapshot from /api/events.
  const tunnelBadge = (s: Site) => {
    const t = status?.tunnels?.[s.name]
    if (!t) return null
    if (t.status === "online") return <ToneBadge tone="success">public</ToneBadge>
    if (t.status === "starting") return <ToneBadge tone="warning">starting…</ToneBadge>
    if (t.status === "error") return <ToneBadge tone="danger">share failed</ToneBadge>
    return null
  }

  const startStopBtn = (s: Site) => (
    <Button
      size="sm"
      variant="outline"
      disabled={busy === s.name + "start" || busy === s.name + "stop"}
      onClick={() => act(s, s.running ? "stop" : "start")}
    >
      {s.running ? <Square /> : <Play />}
      {s.running ? "Stop" : "Start"}
    </Button>
  )

  const rowMenu = (s: Site) => (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button size="icon-sm" variant="ghost">
            <EllipsisVerticalIcon />
            <span className="sr-only">More actions</span>
          </Button>
        }
      />
      <DropdownMenuContent align="end">
        <DropdownMenuItem render={<Link to={`/sites/${s.name}`} />}>
          <Box /> View details
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => act(s, "restart")} disabled={!s.running}>
          <RotateCw /> Restart
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => openConfigure(s)}>
          <Settings2 /> Configure…
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => openTerminalFor(s)}>
          <TerminalSquare /> Open in terminal
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => toggleShare(s)}
          disabled={busy === s.name + "tunnel"}
        >
          {status?.tunnels?.[s.name]?.enabled ? <Square /> : <Share2 />}
          {status?.tunnels?.[s.name]?.enabled ? "Stop sharing" : "Share publicly…"}
        </DropdownMenuItem>
        <DropdownMenuItem render={<Link to={`/logs?site=${s.name}`} />}>
          <ScrollText /> Logs
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" onClick={() => setDeleteTarget(s)}>
          <Trash2 /> Delete…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )

  const emptyRow = (
    <Empty className="h-40 border-0">
      <Globe className="size-6 opacity-50" />
      <EmptyTitle>No sites yet.</EmptyTitle>
    </Empty>
  )

  /* ------------------------------- listing ------------------------------- */

  const listing = (
    <>
      {/* Desktop/tablet: table */}
      <div className="hidden rounded-2xl border border-ring-outer bg-ring-gap p-[3px] md:block">
        <div className="overflow-x-auto rounded-[14px] border border-ring-inner bg-card">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>Site</TableHead>
                <TableHead>URLs</TableHead>
                {/* Fixed widths for the right-hand columns: the site name and
                    the two URL lines are the only flexible content, so the
                    badges and buttons line up across every row instead of
                    drifting with whatever text each cell happens to hold. The
                    Actions column has to fit a sm button plus the kebab menu. */}
                <TableHead className="w-28">PHP</TableHead>
                <TableHead className="w-32">Status</TableHead>
                <TableHead className="w-40 text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5}>{emptyRow}</TableCell>
                </TableRow>
              )}
              {filtered.map((s) => (
                <TableRow
                  key={s.name}
                  className="cursor-pointer even:bg-surface-raised/25 hover:bg-surface-raised/60"
                  onClick={() => navigate(`/sites/${s.name}`)}
                >
                  {/* The name is a real link: the row's onClick is a convenience
                      for the mouse, but it leaves keyboard and screen-reader
                      users no way in. A link also gives the row a middle-click
                      and copy-link target. */}
                  <TableCell className="font-medium">
                    <span className="inline-flex items-center gap-2">
                      <Link to={`/sites/${s.name}`} className="hover:underline">
                        {s.name}
                      </Link>
                      {tunnelBadge(s)}
                    </span>
                  </TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>{urlCell(s)}</TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>{phpBadge(s)}</TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>{runBadge(s)}</TableCell>
                  <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                    <div className="flex items-center justify-end gap-1.5">
                      {startStopBtn(s)}
                      {rowMenu(s)}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>

      {/* Mobile: cards */}
      <div className="flex flex-col gap-3 md:hidden">
        {filtered.length === 0 && (
          <div className="overflow-hidden rounded-xl border border-ring-inner bg-surface">{emptyRow}</div>
        )}
        {filtered.map((s) => (
          <div key={s.name} className="flex flex-col gap-3 rounded-xl border border-ring-inner bg-surface p-4">
            <div className="flex items-center justify-between gap-2">
              <span className="inline-flex items-center gap-2 font-medium">
                {runBadge(s)} {s.name} {tunnelBadge(s)}
              </span>
              {phpBadge(s)}
            </div>
            {urlCell(s)}
            <div className="flex items-center justify-between gap-2 border-t border-border-soft pt-3">
              {startStopBtn(s)}
              <div className="flex items-center gap-1">
                <Button size="sm" variant="ghost" onClick={() => openTerminalFor(s)}>
                  <TerminalSquare /> Terminal
                </Button>
                {rowMenu(s)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </>
  )

  /* -------------------------------- layout ------------------------------- */

  const termDir = termSite?.dir ?? ""

  return (
    <div className="flex h-full min-h-0 flex-col px-5 pb-6 lg:px-7">
      <PageActions className="pt-7">
          <>
          <div className="relative">
            <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-text-muted" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter sites…"
              className="h-[30px] w-52 pl-8 text-[13.5px]"
            />
          </div>
          <Button
            variant={dock.open ? "secondary" : "outline"}
            size="sm"
            className="max-lg:hidden"
            onClick={() => setDock((d) => ({ ...d, open: !d.open }))}
            title={dock.open ? "Hide terminal" : "Show terminal"}
          >
            <PanelRight /> Terminal
          </Button>
          <Dialog
            open={open}
            onOpenChange={(v) => {
              if (v) void openCreate()
              else setOpen(false)
            }}
          >
            <DialogTrigger render={<Button size="sm" />}>
              <Plus /> New Site
            </DialogTrigger>
            <DialogContent className="max-h-[calc(100vh-3rem)] overflow-y-auto sm:max-w-2xl">
              <DialogHeader>
                <DialogTitle>
                  {createState?.running
                    ? `Creating ${createState.template} site "${createState.name}"`
                    : createState?.done && !createState.error
                      ? `"${createState.name}" is ready`
                      : "Create a new site"}
                </DialogTitle>
                {/* A scaffold is minutes of composer (and npm), so the dialog says
                    so up front instead of looking stalled halfway through. */}
                {createState?.running ? (
                  <DialogDescription>
                    Composer resolves and installs packages one at a time{createState.template.startsWith("laravel") ? ", and a starter kit runs npm too" : ""}.
                    This can take a few minutes; the log below is live, and you can close this window and let it finish.
                  </DialogDescription>
                ) : null}
              </DialogHeader>

              {createState && (createState.running || createState.done) ? (
                <div className="flex min-w-0 flex-col gap-3">
                  <div className="flex flex-wrap items-center gap-2 text-sm">
                    {createState.running ? (
                      <Loader2 className="size-4 animate-spin text-muted-foreground" />
                    ) : createState.error ? (
                      <XCircle className="size-4 text-destructive" />
                    ) : (
                      <CheckCircle2 className="size-4 text-success" />
                    )}
                    <span>
                      {createState.running
                        ? `Working… ${elapsed}s`
                        : createState.error
                          ? "Creation failed"
                          : "Created"}
                    </span>
                    {createState.done && !createState.error && createState.url ? (
                      <a
                        href={createState.url}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 text-[13.5px] text-brand hover:underline"
                      >
                        <Globe className="size-3.5" /> Open site
                      </a>
                    ) : null}
                    {/* The simplified view is what a person reads; the raw bytes are
                        one click away because that is what a bug report needs. */}
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-muted-foreground ml-auto h-7 px-2 text-[13px]"
                      onClick={() => setRawLog((v) => !v)}
                    >
                      {rawLog ? "Simplified log" : "Raw log"}
                    </Button>
                  </div>

                  <pre
                    ref={logRef}
                    className={`max-h-72 min-h-40 overflow-x-hidden overflow-y-auto rounded-xl border border-ring-inner bg-surface-inset p-4 font-mono text-[13.5px] leading-relaxed whitespace-pre-wrap text-text-secondary ${
                      rawLog ? "break-all" : "break-words"
                    } ${createState.error ? "border-destructive/40" : ""}`}
                  >
                    {(rawLog ? createState.output_raw : createState.output) ||
                      (createState.running ? "starting…" : "")}
                  </pre>

                  {createState.error ? (
                    <p className="text-destructive text-[13px]">{createState.error}</p>
                  ) : null}
                  {createState.note ? (
                    <p className="text-warning text-[13px]">{createState.note}</p>
                  ) : null}
                </div>
              ) : (
                <div className="flex flex-col gap-4">
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="site-name">Site name</Label>
                    <Input
                      id="site-name"
                      value={name}
                      placeholder="my-app"
                      onChange={(e) => setName(e.target.value)}
                      onKeyDown={(e) => e.key === "Enter" && void create()}
                    />
                  </div>
                  <div className="flex flex-col gap-2">
                    <Label>Template</Label>
                    <Select value={template} onValueChange={(v) => setTemplate(v ?? "blank")}>
                      <SelectTrigger className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {TEMPLATES.map((t) => (
                          <SelectItem key={t.id} value={t.id}>
                            {t.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  {/* Laravel only: a blank or WordPress template has no database to
                      point anywhere and no tests to convert. */}
                  {template.startsWith("laravel") && (
                    <div className="grid grid-cols-2 gap-4">
                      <div className="flex flex-col gap-2">
                        <Label>Database</Label>
                        <Select value={database} onValueChange={(v) => setDatabase(v ?? "sqlite")}>
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="sqlite">SQLite (default)</SelectItem>
                            <SelectItem value="mariadb">MariaDB — created for you</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="flex flex-col gap-2">
                        <Label>Testing</Label>
                        <Select value={testing} onValueChange={(v) => setTesting(v ?? "pest")}>
                          <SelectTrigger className="w-full">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="pest">Pest (default)</SelectItem>
                            <SelectItem value="phpunit">PHPUnit</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                  )}
                </div>
              )}

              <DialogFooter>
                {createState?.running ? (
                  <Button variant="outline" onClick={() => setOpen(false)}>
                    Run in background
                  </Button>
                ) : createState?.done ? (
                  <>
                    {createState.error ? (
                      <Button variant="outline" onClick={() => setCreateState(null)}>
                        Try again
                      </Button>
                    ) : (
                      <Button variant="outline" onClick={() => setCreateState(null)}>
                        Create another
                      </Button>
                    )}
                    <Button onClick={() => setOpen(false)}>Close</Button>
                  </>
                ) : (
                  <Button onClick={() => void create()} disabled={!name.trim()}>
                    Create site
                  </Button>
                )}
              </DialogFooter>
            </DialogContent>
          </Dialog>
          </>
        </PageActions>

      {/* Two-pane app frame: content | terminal dock. Both scroll internally;
          the frame spans exactly to the bottom of the viewport. The content
          column carries NO extra right padding: with lg:pr-4 here the table
          ended 16px short of the toolbar buttons above it. The gap belongs to
          the dock, which is the only thing that needs separating. */}
      <div className="mt-4 flex min-h-0 flex-1 flex-col lg:flex-row">
        {/* Left: scrolling content column */}
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto pb-6">
          {listing}
        </div>

        {dock.open && (
          <>
            {/* Divider: vertical on desktop (drag width), horizontal on mobile */}
            <div
              className="group relative my-1 h-1.5 cursor-row-resize rounded-full bg-border-soft hover:bg-brand/40 lg:my-0 lg:ml-4 lg:h-auto lg:w-1.5 lg:cursor-col-resize"
              onPointerDown={(e) => {
                draggingRef.current = true
                e.currentTarget.setPointerCapture(e.pointerId)
              }}
              onPointerMove={(e) => {
                if (!draggingRef.current) return
                const container = e.currentTarget.parentElement
                if (!container) return
                const rect = container.getBoundingClientRect()
                if (isDesktop) {
                  const w = rect.right - e.clientX
                  setDock((d) => ({ ...d, width: Math.min(Math.max(w, 300), rect.width * 0.7) }))
                } else {
                  const h = rect.bottom - e.clientY
                  setMobileTermHeight(Math.min(Math.max(h, 200), rect.height * 0.7))
                }
              }}
              onPointerUp={() => (draggingRef.current = false)}
              onPointerCancel={() => (draggingRef.current = false)}
              title="Drag to resize"
            >
              <span className="absolute top-1/2 left-1/2 h-1 w-10 -translate-x-1/2 -translate-y-1/2 rounded-full bg-brand/50 opacity-0 transition-opacity group-hover:opacity-100 lg:h-10 lg:w-1" />
            </div>

            {/* Right dock: full height on desktop, bottom panel on mobile */}
            <aside
              className="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl border border-ring-inner bg-surface-inset max-lg:shrink-0 lg:rounded-none lg:border-y-0 lg:border-r-0"
              style={
                isDesktop
                  ? { width: dock.width }
                  : { height: mobileTermHeight }
              }
            >
              {/* Dock header */}
              <div className="flex h-10 shrink-0 items-center gap-2 border-b border-border-soft px-3">
                <TerminalSquare className="size-4 shrink-0" />
                <Select
                  value={termSite?.name ?? "__root"}
                  onValueChange={(v) =>
                    setTermSite(v === "__root" ? null : (sites.find((s) => s.name === v) ?? null))
                  }
                >
                  <SelectTrigger size="sm" className="h-7 w-40 font-medium">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="__root">sites/ root</SelectItem>
                    {sites.map((s) => (
                      <SelectItem key={s.name} value={s.name}>
                        {s.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <TermStatusLabel s={termStatus} />
                <div className="ml-auto flex items-center gap-1">
                  <IconButton
                    label="Clear"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => panelRef.current?.clear()}
                  >
                    <Eraser />
                  </IconButton>
                  <IconButton
                    label="Restart shell"
                    size="icon-sm"
                    variant="ghost"
                    onClick={() => panelRef.current?.restart()}
                  >
                    <RotateCw />
                  </IconButton>
                  <IconButton
                    label="Hide terminal"
                    size="icon-sm"
                    variant="ghost"
                    className="max-lg:hidden"
                    onClick={() => setDock((d) => ({ ...d, open: false }))}
                  >
                    <PanelRight />
                  </IconButton>
                </div>
              </div>
              <div className="relative min-h-0 flex-1 overflow-hidden">
                <TerminalPanel
                  ref={panelRef}
                  dir={termDir}
                  sessionKey={`sites-dock:${termSite?.name ?? "root"}`}
                  className="absolute inset-0 h-auto border-0 rounded-none"
                  onStatus={setTermStatus}
                />
              </div>
            </aside>
          </>
        )}
      </div>

      <Dialog open={!!cfgSite} onOpenChange={(v) => !v && setCfgSite(null)}>
        <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Configure “{cfgSite?.name}”</DialogTitle>
            <DialogDescription>
              Changes apply immediately; a running site restarts.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label>PHP version</Label>
              <Select value={cfg.php} onValueChange={(v) => setCfg({ ...cfg, php: v ?? "" })}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Default" />
                </SelectTrigger>
                <SelectContent>
                  {["General", "Bundled", "System"].map((group) => (
                    <Fragment key={group}>
                      <div className="text-muted-foreground px-2 py-1 text-[13px] font-medium">
                        {group}
                      </div>
                      {phpChoices
                        .filter((o) => o.group === group)
                        .map((o) => (
                          <SelectItem key={o.value || "__default"} value={o.value}>
                            {o.label}
                          </SelectItem>
                        ))}
                    </Fragment>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="cfg-phpini">Custom php.ini (absolute path or relative to site folder)</Label>
              <Input
                id="cfg-phpini"
                placeholder="php-custom.ini"
                value={cfg.php_ini ?? ""}
                onChange={(e) => setCfg({ ...cfg, php_ini: e.target.value })}
              />
              <p className="text-muted-foreground text-[13px]">
                Override the global php.ini for this site only. Leave empty to use the global config.
              </p>
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="cfg-docroot">Document root (relative to site folder)</Label>
              <Input
                id="cfg-docroot"
                placeholder="public"
                value={cfg.docroot}
                onChange={(e) => setCfg({ ...cfg, docroot: e.target.value })}
              />
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="cfg-aliases">Aliases (one per line)</Label>
              <Textarea
                id="cfg-aliases"
                rows={3}
                placeholder={"www.myapp.localhost\napi.myapp.test"}
                value={(cfg.aliases ?? []).join("\n")}
                onChange={(e) =>
                  setCfg({ ...cfg, aliases: e.target.value.split("\n").map((x) => x.trim()) })
                }
              />
              <p className="text-muted-foreground text-[13px]">
                Custom domains (not *.localhost) need a hosts entry to resolve.
              </p>
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="cfg-env">Environment variables (KEY=value per line)</Label>
              <Textarea
                id="cfg-env"
                rows={4}
                placeholder={"APP_ENV=local\nAPP_DEBUG=true"}
                value={Object.entries(cfg.env ?? {})
                  .map(([k, v]) => `${k}=${v}`)
                  .join("\n")}
                onChange={(e) => {
                  const env: Record<string, string> = {}
                  e.target.value.split("\n").forEach((line) => {
                    const idx = line.indexOf("=")
                    if (idx > 0) env[line.slice(0, idx).trim()] = line.slice(idx + 1).trim()
                  })
                  setCfg({ ...cfg, env })
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCfgSite(null)}>
              Cancel
            </Button>
            <Button onClick={saveConfigure} disabled={savingCfg}>
              {savingCfg ? "Saving…" : "Save & apply"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog open={!!deleteTarget} onOpenChange={(v) => !v && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete “{deleteTarget?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              The folder is moved to <code>sites/.trash/</code> — nothing is permanently lost.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={doDelete}>
              <Trash2 /> Move to trash
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
