import { useEffect, useState } from "react"
import { Link } from "react-router-dom"
import { toast } from "sonner"
import {
  Copy,
  EllipsisVerticalIcon,
  ExternalLink,
  Globe,
  Link as LinkIcon,
  Lock,
  Play,
  Plus,
  RotateCw,
  ScrollText,
  Settings2,
  ShieldAlert,
  Square,
  Trash2,
} from "lucide-react"

import api, {
  poll,
  type Site,
  type SiteConfigPayload,
  type Status,
} from "@/lib/api"
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

function CleanUrlBanner({ tld }: { tld?: string }) {
  return (
    <div className="bg-card flex flex-col gap-2 rounded-xl border p-4 sm:flex-row sm:items-center">
      <ShieldAlert className="text-amber-500 size-5 shrink-0" />
      <p className="text-muted-foreground flex-1 text-sm">
        URLs currently include a port. Run{" "}
        <code className="bg-muted rounded px-1.5 py-0.5">sudo ./sabdopalon enable-ports</code>{" "}
        once and restart to unlock clean URLs like{" "}
        <code className="bg-muted rounded px-1.5 py-0.5">
          https://my-app.{tld ?? "localhost"}
        </code>
        .
      </p>
      <Button
        size="sm"
        variant="outline"
        onClick={() => {
          navigator.clipboard.writeText("sudo ./sabdopalon enable-ports")
          toast.success("Command copied — run it, then restart Sabdopalon")
        }}
      >
        <Copy /> Copy command
      </Button>
    </div>
  )
}

export default function SitesPage() {
  const [sites, setSites] = useState<Site[]>([])
  const [status, setStatus] = useState<Status | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Site | null>(null)
  const [cfgSite, setCfgSite] = useState<Site | null>(null)
  const [cfg, setCfg] = useState<SiteConfigPayload>({ php: "", docroot: "", aliases: [], env: {} })
  const [phpOptions, setPhpOptions] = useState<{ value: string; label: string; group: string }[]>([])
  const [savingCfg, setSavingCfg] = useState(false)

  // New-site dialog state
  const [open, setOpen] = useState(false)
  const [name, setName] = useState("")
  const [template, setTemplate] = useState("blank")
  const [creating, setCreating] = useState(false)

  const load = () => api.listSites().then(setSites).catch(() => {})

  useEffect(() => {
    const t = poll(load, 4000)
    api.status().then(setStatus).catch(() => {})
    const st = setInterval(() => api.status().then(setStatus).catch(() => {}), 6000)
    return () => {
      clearInterval(t)
      clearInterval(st)
    }
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
    // PHP options: default + bundled + system (composed once per open)
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
    setDeleteTarget(null)
    load()
  }

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <div className="flex items-center justify-between gap-2">
        <p className="text-muted-foreground text-sm">
          Every folder in <code className="bg-muted rounded px-1.5 py-0.5">sites/</code> is
          automatically served at <code className="bg-muted rounded px-1.5 py-0.5">name.localhost</code>.
        </p>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger render={<Button />}>
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

      {!status?.low_ports && <CleanUrlBanner tld={status?.tld} />}

      <div className="bg-card overflow-hidden rounded-xl border">
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
            {sites.length === 0 && (
              <TableRow>
                <TableCell colSpan={5} className="text-muted-foreground h-28 text-center">
                  <Globe className="mx-auto mb-2 size-6 opacity-50" />
                  No sites yet — click “New Site” or create a folder under sites/.
                </TableCell>
              </TableRow>
            )}
            {sites.map((s) => (
              <TableRow key={s.name}>
                <TableCell className="font-medium">{s.name}</TableCell>
                <TableCell>
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
                            <a
                              key={a}
                              href={aliasTarget(s, a)}
                              target="_blank"
                              rel="noreferrer"
                              title={`Open http://${a}`}
                            >
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
                </TableCell>
                <TableCell>
                  {s.php ? (
                    <Badge variant="secondary" className="text-amber-500 dark:text-amber-400">
                      PHP {s.php}
                    </Badge>
                  ) : (
                    <span className="text-muted-foreground">default</span>
                  )}
                </TableCell>
                <TableCell>
                  <Badge variant={s.running ? "default" : "outline"}>
                    {s.running ? "running" : "stopped"}
                  </Badge>
                </TableCell>
                <TableCell className="text-right">
                  <div className="inline-flex items-center gap-1">
                    <Button
                      size="sm"
                      variant={s.running ? "outline" : "default"}
                      disabled={busy === s.name + "start" || busy === s.name + "stop"}
                      onClick={() => act(s, s.running ? "stop" : "start")}
                    >
                      {s.running ? <Square /> : <Play />}
                      {s.running ? "Stop" : "Start"}
                    </Button>
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
                        <DropdownMenuItem onClick={() => act(s, "restart")} disabled={!s.running}>
                          <RotateCw /> Restart
                        </DropdownMenuItem>
                        <DropdownMenuItem onClick={() => openConfigure(s)}>
                          <Settings2 /> Configure…
                        </DropdownMenuItem>
                        <DropdownMenuItem render={<Link to={`/logs?site=${s.name}`} />}>
                          <ScrollText /> Logs
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          variant="destructive"
                          onClick={() => setDeleteTarget(s)}
                        >
                          <Trash2 /> Delete…
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
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
