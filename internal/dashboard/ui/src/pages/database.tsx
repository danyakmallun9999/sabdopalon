import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { Database, HardDriveDownload, Play, RotateCw, Server, Square, Terminal as TerminalIcon } from "lucide-react"

import api, { poll, type Backup, type ConfigPayload } from "@/lib/api"
import { useLive } from "@/lib/live"
import TerminalPanel, { type TermStatus } from "@/components/terminal-panel"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type DbAction = "start" | "stop" | "restart"
type Tab = "daemons" | "backups" | "terminal"
type Engine = "mariadb" | "postgresql"

type DaemonCfg = {
  key: Engine
  label: string
  enabled?: boolean
  port?: number
  installed?: boolean
}

// Every daemon gets its own card and can run at the same time — each with
// its own port ("default aktif semua").

// 0 in the config means "unset" (legacy port field wins) — never display or
// prefill it; show the fallback instead.
function portOr(v: number | undefined, fallback: number): number {
  return v && v > 0 ? v : fallback
}

function daemonCards(cfg: ConfigPayload): DaemonCfg[] {
  return [
    {
      key: "mariadb",
      label: "MariaDB",
      enabled: cfg.db_mariadb_enabled ?? true,
      port: portOr(cfg.db_mariadb_port, 3306),
      installed: cfg.db_installed?.mariadb ?? false,
    },
    {
      key: "postgresql",
      label: "PostgreSQL",
      enabled: cfg.db_pg_enabled ?? true,
      port: portOr(cfg.db_pg_port, 5433),
      installed: cfg.db_installed?.postgresql ?? false,
    },
  ]
}

// CLI client binary spawned by the Terminal tab per engine. The terminal
// backend seeds MYSQL_TCP_PORT / PGHOST etc. so these connect with no flags.
function dbClientCmd(engine: Engine): string[] {
  return engine === "mariadb" ? ["mariadb"] : ["psql"]
}

export default function DatabasePage() {
  const { status } = useLive()
  const [cfg, setCfg] = useState<ConfigPayload>({})
  const [backups, setBackups] = useState<Backup[]>([])
  const [busyKey, setBusyKey] = useState<string | null>(null)
  const [busyBackup, setBusyBackup] = useState(false)
  const [ports, setPorts] = useState<Record<string, number | "">>({})
  const [tab, setTab] = useState<Tab>("daemons")
  // Port drafts sync once per load; polling never clobbers edits.
  const syncedRef = useRef(false)

  useEffect(() => {
    async function first() {
      const c = await api.getConfig().catch(() => null)
      if (c) {
        setCfg(c)
        if (!syncedRef.current) {
          setPorts({
            mariadb: portOr(c.db_mariadb_port, 3306),
            postgresql: portOr(c.db_pg_port, 5433),
          })
          syncedRef.current = true
        }
      }
      const b = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(b) ? b : [])
    }
    first()
    const t = poll(async () => {
      api.getConfig().then(setCfg).catch(() => {})
      const b = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(b) ? b : [])
    }, 6000)
    return () => clearInterval(t)
  }, [])

  function setPort(key: string, v: number | "") {
    setPorts((p) => ({ ...p, [key]: v }))
  }

  async function savePort(key: "mariadb" | "postgresql") {
    const field = key === "mariadb" ? "db_mariadb_port" : "db_pg_port"
    const v = ports[key]
    if (v === "" || !v) return
    try {
      await api.saveConfig({ [field]: Number(v) })
      toast.success(`Port ${key} disimpan`, {
        description: "Berlaku setelah restart daemon (Restart di kartu ini) atau restart Sabdopalon.",
      })
    } catch {
      toast.error("Gagal menyimpan port")
    }
  }

  async function toggleEnabled(key: "mariadb" | "postgresql", enabled: boolean) {
    const field = key === "mariadb" ? "db_mariadb_enabled" : "db_pg_enabled"
    try {
      await api.saveConfig({ [field]: enabled })
      toast.success(
        `${key} ${enabled ? "diaktifkan" : "dinonaktifkan"}`,
        { description: enabled ? "Daemon sedang dinyalakan…" : undefined },
      )
      setTimeout(async () => {
        const c = await api.getConfig().catch(() => null)
        if (c) setCfg(c)
        }, 1500)
    } catch {
      toast.error("Gagal mengubah status")
    }
  }

  async function doAction(key: "mariadb" | "postgresql", a: DbAction) {
    setBusyKey(key + a)
    try {
      const r = await api.databaseControl(key, a)
      if (r.error) toast.error(`${key} ${a}: ${r.error}`)
      else toast.success(`${key}: ${a} OK`)
    } finally {
      setBusyKey(null)
    }
  }

  async function doBackup() {
    setBusyBackup(true)
    try {
      const r = await api.backupNow()
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `Backup dibuat: ${r.backup}`)
      const b = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(b) ? b : [])
    } finally {
      setBusyBackup(false)
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 px-4 lg:px-6">
      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)} className="flex min-h-0 flex-1 flex-col">
        <TabsList className="flex-wrap">
          <TabsTrigger value="daemons"><Server className="size-3.5" /> Daemons</TabsTrigger>
          <TabsTrigger value="backups"><Database className="size-3.5" /> Backups</TabsTrigger>
          <TabsTrigger value="terminal"><TerminalIcon className="size-3.5" /> Terminal</TabsTrigger>
        </TabsList>

        <TabsContent value="daemons" className="mt-4 min-h-0 flex-1 overflow-y-auto">
          <DaemonsTab
            cfg={cfg}
            status={status}
            ports={ports}
            busyKey={busyKey}
            setPort={setPort}
            savePort={savePort}
            toggleEnabled={toggleEnabled}
            doAction={doAction}
          />
        </TabsContent>

        <TabsContent value="backups" className="mt-4 min-h-0 flex-1 overflow-y-auto">
          <BackupsTab
            backups={backups}
            busyBackup={busyBackup}
            doBackup={doBackup}
          />
        </TabsContent>

        <TabsContent value="terminal" className="mt-4 min-h-0 flex-1 overflow-hidden">
          <TerminalTab cfg={cfg} status={status} />
        </TabsContent>
      </Tabs>
    </div>
  )
}

/* --------------------------------- Daemons --------------------------------- */

function DaemonsTab({
  cfg,
  status,
  ports,
  busyKey,
  setPort,
  savePort,
  toggleEnabled,
  doAction,
}: {
  cfg: ConfigPayload
  status: ReturnType<typeof useLive>["status"]
  ports: Record<string, number | "">
  busyKey: string | null
  setPort: (key: string, v: number | "") => void
  savePort: (key: "mariadb" | "postgresql") => void
  toggleEnabled: (key: "mariadb" | "postgresql", enabled: boolean) => void
  doAction: (key: "mariadb" | "postgresql", a: DbAction) => void
}) {
  return (
    <>
      {/* One card per database engine — all can run simultaneously. */}
      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2">
        {daemonCards(cfg).map((d) => {
          const running = status?.db_states?.[d.key] ?? cfg.db_states?.[d.key] ?? false
          const err = status?.db_errors?.[d.key] ?? cfg.db_errors?.[d.key]
          const dirty = ports[d.key] !== undefined && Number(ports[d.key]) !== d.port
          return (
            <Card key={d.key}>
              <CardHeader>
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="flex flex-col gap-1">
                    <CardTitle>{d.label}</CardTitle>
                    <CardDescription>
                      Port {running ? `aktif di :${d.port}` : `: ${d.port}`}
                    </CardDescription>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={running ? "default" : "outline"}>
                      {running ? "berjalan" : "berhenti"}
                    </Badge>
                    <Switch
                      checked={!!d.enabled}
                      onCheckedChange={(v) => toggleEnabled(d.key, v === true)}
                      title={d.enabled ? "Nonaktifkan daemon ini" : "Aktifkan daemon ini"}
                    />
                  </div>
                </div>

                {!d.installed && (
                  <p className="text-destructive mt-2 text-xs">
                    Belum terpasang — pasang dulu di halaman Packages.
                  </p>
                )}
                {err && <p className="text-destructive mt-2 text-xs">Gagal start: {err}</p>}

                {d.enabled && (
                  <>
                    <div className="mt-3 flex flex-wrap items-center gap-2">
                      {!running && (
                        <Button
                          size="sm"
                          disabled={!d.installed || busyKey !== null}
                          onClick={() => doAction(d.key, "start")}
                        >
                          <Play /> Start
                        </Button>
                      )}
                      {running && (
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busyKey !== null}
                          onClick={() => doAction(d.key, "stop")}
                        >
                          <Square /> Stop
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={!d.installed || busyKey !== null || !running}
                        onClick={() => doAction(d.key, "restart")}
                      >
                        <RotateCw /> Restart
                      </Button>
                      {(busyKey === d.key + "start" || busyKey === d.key + "restart") && (
                        <span className="text-muted-foreground text-xs">
                          {busyKey === d.key + "start" ? "starting…" : "restarting…"}
                        </span>
                      )}
                    </div>

                    <div className="mt-3 grid grid-cols-[1fr_auto] items-end gap-2">
                      <div className="flex flex-col gap-1">
                        <Label htmlFor={"port-" + d.key} className="text-xs">
                          Port (butuh restart daemon)
                        </Label>
                        <Input
                          id={"port-" + d.key}
                          type="number"
                          value={ports[d.key] ?? ""}
                          onChange={(e) =>
                            setPort(d.key, e.target.value === "" ? "" : Number(e.target.value))
                          }
                        />
                      </div>
                      <Button size="sm" variant="secondary" disabled={!dirty} onClick={() => savePort(d.key)}>
                        Simpan port
                      </Button>
                    </div>
                  </>
                )}
              </CardHeader>
            </Card>
          )
        })}

        {/* SQLite — zero setup, always available */}
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle>SQLite</CardTitle>
              <Badge variant="secondary">selalu aktif</Badge>
            </div>
            <CardDescription>
              Tanpa daemon — file database di <code>data/sabdopalon.db</code>, langsung dipakai PHP.
            </CardDescription>
          </CardHeader>
        </Card>
      </div>

      <p className="text-muted-foreground mt-4 px-1 text-xs">
        Semua database bisa hidup bersamaan — situs kamu bebas memakai koneksi mana pun
        (env <code>SABDOPALON_MARIADB_*</code> dan <code>SABDOPALON_PG_*</code> tersedia untuk semua situs).
      </p>
    </>
  )
}

/* --------------------------------- Backups --------------------------------- */

function BackupsTab({
  backups,
  busyBackup,
  doBackup,
}: {
  backups: Backup[]
  busyBackup: boolean
  doBackup: () => void
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <div className="flex flex-col gap-1.5">
          <CardTitle>Backups</CardTitle>
          <CardDescription>Backup lama dipangkas otomatis (simpan 5).</CardDescription>
        </div>
        <Button onClick={doBackup} disabled={busyBackup}>
          <HardDriveDownload /> Backup Sekarang
        </Button>
      </CardHeader>
      <div className="px-4 pb-4 lg:px-6">
        {backups.length === 0 ? (
          <p className="text-muted-foreground border-dashed rounded-xl border p-8 text-center text-sm">
            Belum ada backup — klik “Backup Sekarang”.
          </p>
        ) : (
          <div className="bg-card overflow-hidden rounded-xl border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Nama</TableHead>
                  <TableHead>Ukuran</TableHead>
                  <TableHead>Waktu</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {backups.map((b) => (
                  <TableRow key={b.name}>
                    <TableCell className="font-mono text-xs">{b.name}</TableCell>
                    <TableCell>{(b.size / 1024).toFixed(0)} KB</TableCell>
                    <TableCell className="text-muted-foreground">{b.time}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
    </Card>
  )
}

/* --------------------------------- Terminal -------------------------------- */

function TerminalTab({
  cfg,
  status,
}: {
  cfg: ConfigPayload
  status: ReturnType<typeof useLive>["status"]
}) {
  const cards = daemonCards(cfg)
  // Default to the first running engine, else the first installed one.
  const firstRunning = cards.find((d) => status?.db_states?.[d.key])?.key
  const firstInstalled = cards.find((d) => d.installed)?.key
  const [engine, setEngine] = useState<Engine>(firstRunning ?? firstInstalled ?? "mariadb")
  const [termStatus, setTermStatus] = useState<TermStatus>("connecting")
  const panelRef = useRef<{ clear: () => void; restart: () => void }>(null)

  const card = cards.find((d) => d.key === engine)!
  const running = status?.db_states?.[engine] ?? cfg.db_states?.[engine] ?? false
  const installed = card.installed

  return (
    <div className="flex h-full min-h-0 flex-col gap-3">
      {/* Engine selector + connection status */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="inline-flex rounded-lg border bg-muted/40 p-0.5">
          {cards.map((d) => {
            const dRunning = status?.db_states?.[d.key] ?? cfg.db_states?.[d.key] ?? false
            const active = d.key === engine
            return (
              <button
                key={d.key}
                type="button"
                disabled={!d.installed}
                onClick={() => setEngine(d.key)}
                title={!d.installed ? "Pasang dulu di halaman Packages" : dRunning ? "berjalan" : "berhenti"}
                className={`inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-40 ${
                  active ? "bg-background shadow-sm" : "text-muted-foreground hover:text-foreground"
                }`}
              >
                <span className={`size-1.5 rounded-full ${dRunning ? "bg-emerald-500" : "bg-zinc-400 dark:bg-zinc-600"}`} />
                {d.label}
              </button>
            )
          })}
        </div>
        <div className="flex items-center gap-2 text-xs text-muted-foreground">
          {running ? (
            <>
              <span className={`size-2 rounded-full ${termStatus === "connected" ? "bg-emerald-500" : termStatus === "connecting" ? "bg-amber-500 animate-pulse" : "bg-red-400"}`} />
              {termStatus}
            </>
          ) : (
            <span>{card.label} berhenti</span>
          )}
        </div>
      </div>

      {/* Terminal or not-running notice */}
      {!installed ? (
        <Card>
          <CardContent className="flex items-center gap-3 py-6 text-sm text-muted-foreground">
            <TerminalIcon className="size-4 shrink-0" />
            {card.label} belum terpasang — pasang dulu di halaman Packages.
          </CardContent>
        </Card>
      ) : !running ? (
        <Card>
          <CardContent className="flex flex-col gap-2 py-6 text-sm">
            <span className="flex items-center gap-2">
              <TerminalIcon className="size-4 shrink-0 text-muted-foreground" />
              {card.label} belum berjalan.
            </span>
            <span className="text-muted-foreground">
              Nyalakan daemon-nya di tab <b>Daemons</b> dulu, lalu kembali ke sini.
            </span>
          </CardContent>
        </Card>
      ) : (
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border bg-background">
          <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2">
            <TerminalIcon className="size-4 shrink-0" />
            <span className="text-sm font-medium">{card.label}</span>
            <span className={`size-2 rounded-full ${termStatus === "connected" ? "bg-emerald-500" : termStatus === "connecting" ? "bg-amber-500 animate-pulse" : "bg-red-400"}`} />
            <span className="text-muted-foreground text-xs">{termStatus}</span>
            <div className="ml-auto flex items-center gap-1.5">
              <Button size="sm" variant="ghost" onClick={() => panelRef.current?.clear()}>
                Clear
              </Button>
              <Button size="sm" variant="ghost" onClick={() => panelRef.current?.restart()}>
                <RotateCw /> Restart
              </Button>
            </div>
          </div>
          <div className="relative min-h-0 flex-1 overflow-hidden">
            <TerminalPanel
              ref={panelRef}
              cmd={dbClientCmd(engine)}
              sessionKey={`database:${engine}`}
              className="absolute inset-0 h-auto border-0 rounded-none"
              onStatus={setTermStatus}
            />
          </div>
        </div>
      )}
    </div>
  )
}
