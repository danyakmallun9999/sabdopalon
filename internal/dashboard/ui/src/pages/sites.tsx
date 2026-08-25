import { useEffect, useRef, useState } from "react"
import { Link, useNavigate } from "react-router-dom"
import { toast } from "sonner"
import {
  Box,
  Copy,
  EllipsisVerticalIcon,
  Eraser,
  ExternalLink,
  Globe,
  Link as LinkIcon,
  Lock,
  PanelRight,
  Play,
  Plus,
  RotateCw,
  ScrollText,
  Search,
  Settings2,
  Square,
  TerminalSquare,
  Trash2,
} from "lucide-react"

import api, {
  poll,
  type Site,
  type SiteConfigPayload,
} from "@/lib/api"
import TerminalPanel, {
  type TermStatus,
  type TerminalPanelHandle,
} from "@/components/terminal-panel"
import { useLive } from "@/lib/live"
import { Badge } from "@/components/ui/badge"
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
  { id: "laravel", label: "Laravel (needs composer)" },
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
  // Terminal tertutup secara default di semua ukuran layar — dibuka lewat
  // tombol Terminal di header. Pilihan user diingat via localStorage.
  return { open: false, width: 480 }
}

function StatusDot({ s }: { s: TermStatus }) {
  const color =
    s === "connected"
      ? "bg-emerald-500"
      : s === "connecting"
        ? "bg-amber-500 animate-pulse"
        : "bg-red-400"
  return (
    <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className={`size-2 rounded-full ${color}`} />
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
  const [creating, setCreating] = useState(false)

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
      pkgs.filter((p) => p.is_php).forEach((p) =>
        opts.push({ value: p.short, label: `${p.short}${p.installed ? "" : " (not downloaded)"}`, group: "Bundled" }),
      )
      const sys = await api.systemPHPs()
      sys.forEach((c) => opts.push({ value: c.path, label: `${c.version} — ${c.path}`, group: "System" }))
    } catch {
      /* options stay partial */
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

  async function create() {
    if (!name.trim()) return
    setCreating(true)
    try {
      const r = await api.createSite(name.trim().toLowerCase(), template)
      if (r.error) toast.error(r.error)
      else {
        toast.success(`Site "${r.name}" created — opening wizard done`, {
          description: r.url,
        })
        setOpen(false)
        setName("")
        load()
      }
    } finally {
      setCreating(false)
    }
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
        className="text-primary inline-flex items-center gap-1.5 text-sm hover:underline"
      >
        <Globe className="size-3.5 shrink-0" />
        {s.url.replace(/^https?:\/\//, "").replace(/\/$/, "")}
        <ExternalLink className="size-3 opacity-60" />
      </a>
      <a
        href={s.https}
        target="_blank"
        rel="noreferrer"
        className="text-muted-foreground inline-flex items-center gap-1.5 text-sm hover:underline"
      >
        <Lock className="size-3.5 shrink-0 text-emerald-500" />
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

  const phpBadge = (s: Site) =>
    s.php ? (
      <Badge variant="secondary" className="text-amber-500 dark:text-amber-400">
        PHP {s.php}
      </Badge>
    ) : (
      <span className="text-muted-foreground">default</span>
    )

  const runBadge = (s: Site) => (
    <span className="inline-flex items-center gap-1.5 text-sm">
      <span
        className={`size-2 rounded-full ${
          s.running ? "bg-emerald-500" : "bg-zinc-400 dark:bg-zinc-600"
        }`}
      />
      {s.running ? "running" : "stopped"}
    </span>
  )

  const startStopBtn = (s: Site) => (
    <Button
      size="sm"
      variant={s.running ? "outline" : "default"}
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
    <div className="text-muted-foreground flex h-40 flex-col items-center justify-center gap-2">
      <Globe className="size-6 opacity-50" />
      No sites yet — click “New Site” or create a folder under sites/.
    </div>
  )

  /* ------------------------------- listing ------------------------------- */

  const listing = (
    <>
      {/* Desktop/tablet: table */}
      <div className="bg-card hidden overflow-hidden rounded-xl border md:block">
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>Site</TableHead>
                <TableHead>URLs</TableHead>
                <TableHead>PHP</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-12 text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.length === 0 && (
                <TableRow>
                  <TableCell colSpan={5}>{emptyRow}</TableCell>
                </TableRow>
              )}
              {filtered.map((s) => (
                <TableRow key={s.name} className="cursor-pointer" onClick={() => navigate(`/sites/${s.name}`)}>
                  <TableCell className="font-medium">{s.name}</TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>{urlCell(s)}</TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>{phpBadge(s)}</TableCell>
                  <TableCell onClick={(e) => e.stopPropagation()}>{runBadge(s)}</TableCell>
                  <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                    <div className="inline-flex items-center gap-1">
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
          <div className="bg-card rounded-xl border">{emptyRow}</div>
        )}
        {filtered.map((s) => (
          <div key={s.name} className="bg-card flex flex-col gap-3 rounded-xl border p-4">
            <div className="flex items-center justify-between gap-2">
              <span className="inline-flex items-center gap-2 font-medium">
                {runBadge(s)} {s.name}
              </span>
              {phpBadge(s)}
            </div>
            {urlCell(s)}
            <div className="flex items-center justify-between gap-2 border-t pt-3">
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
    <div className="flex h-full min-h-0 flex-col px-4 lg:px-6">
      {/* Page header: description + search + actions */}
      <div className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-muted-foreground text-sm">
          Every folder in <code className="bg-muted rounded px-1.5 py-0.5">sites/</code> is
          automatically served at{" "}
          <code className="bg-muted rounded px-1.5 py-0.5">name.localhost</code>.
        </p>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Filter sites…"
              className="h-9 w-44 pl-8"
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
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger render={<Button size="sm" />}>
              <Plus /> New Site
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create a new site</DialogTitle>
                <DialogDescription>
                  The name becomes the local domain (e.g. <code>my-app</code> →{" "}
                  <code>my-app.localhost</code>).
                </DialogDescription>
              </DialogHeader>
              <div className="flex flex-col gap-4">
                <div className="flex flex-col gap-2">
                  <Label htmlFor="site-name">Site name</Label>
                  <Input
                    id="site-name"
                    value={name}
                    placeholder="my-app"
                    onChange={(e) => setName(e.target.value)}
                    onKeyDown={(e) => e.key === "Enter" && create()}
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
              </div>
              <DialogFooter>
                <Button onClick={create} disabled={creating || !name.trim()}>
                  {creating ? "Creating…" : "Create site"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {/* Two-pane app frame: content | terminal dock. Both scroll internally;
          the frame spans exactly to the bottom of the viewport. */}
      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        {/* Left: scrolling content column */}
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto pb-6 lg:pr-4">
          {listing}
        </div>

        {dock.open && (
          <>
            {/* Divider: vertical on desktop (drag width), horizontal on mobile */}
            <div
              className="bg-border/60 hover:bg-primary/40 group relative my-1 h-1.5 cursor-row-resize rounded lg:my-0 lg:h-auto lg:w-1.5 lg:cursor-col-resize"
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
              <span className="bg-primary/40 absolute top-1/2 left-1/2 h-1 w-10 -translate-x-1/2 -translate-y-1/2 rounded-full opacity-0 transition-opacity group-hover:opacity-100 lg:h-10 lg:w-1" />
            </div>

            {/* Right dock: full height on desktop, bottom panel on mobile */}
            <aside
              className="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-xl border bg-background max-lg:shrink-0 lg:rounded-none lg:border-y-0 lg:border-r-0"
              style={
                isDesktop
                  ? { width: dock.width }
                  : { height: mobileTermHeight }
              }
            >
              {/* Dock header */}
              <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2">
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
                <StatusDot s={termStatus} />
                <div className="ml-auto flex items-center gap-1">
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    title="Clear"
                    onClick={() => panelRef.current?.clear()}
                  >
                    <Eraser />
                  </Button>
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    title="Restart shell"
                    onClick={() => panelRef.current?.restart()}
                  >
                    <RotateCw />
                  </Button>
                  <Button
                    size="icon-sm"
                    variant="ghost"
                    title="Hide terminal"
                    className="max-lg:hidden"
                    onClick={() => setDock((d) => ({ ...d, open: false }))}
                  >
                    <PanelRight />
                  </Button>
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
              Stored in <code>.sabdopalon.yml</code>. Saving applies changes immediately — a running
              site is restarted automatically.
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
                    <>
                      <div key={group} className="text-muted-foreground px-2 py-1 text-xs font-medium">
                        {group}
                      </div>
                      {phpOptions
                        .filter((o) => o.group === group)
                        .map((o) => (
                          <SelectItem key={o.value || "__default"} value={o.value}>
                            {o.label}
                          </SelectItem>
                        ))}
                    </>
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
              <p className="text-muted-foreground text-xs">
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
              <p className="text-muted-foreground text-xs">
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
