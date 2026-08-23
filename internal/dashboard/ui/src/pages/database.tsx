import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { HardDriveDownload, Play, RotateCw, Square } from "lucide-react"

import api, { poll, type Backup, type ConfigPayload } from "@/lib/api"
import { useLive } from "@/lib/live"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type DbAction = "start" | "stop" | "restart"

type DaemonCfg = {
  key: "mariadb" | "postgresql"
  label: string
  enabled?: boolean
  port?: number
  installed?: boolean
}

// Every daemon gets its own card and can run at the same time — each with
// its own port ("default aktif semua").
function daemonCards(cfg: ConfigPayload): DaemonCfg[] {
  return [
    {
      key: "mariadb",
      label: "MariaDB",
      enabled: cfg.db_mariadb_enabled ?? true,
      port: cfg.db_mariadb_port ?? 3306,
      installed: cfg.db_installed?.mariadb ?? false,
    },
    {
      key: "postgresql",
      label: "PostgreSQL",
      enabled: cfg.db_pg_enabled ?? true,
      port: cfg.db_pg_port ?? 5433,
      installed: cfg.db_installed?.postgresql ?? false,
    },
  ]
}

export default function DatabasePage() {
  const { status } = useLive()
  const [cfg, setCfg] = useState<ConfigPayload>({})
  const [backups, setBackups] = useState<Backup[]>([])
  const [busyKey, setBusyKey] = useState<string | null>(null)
  const [busyBackup, setBusyBackup] = useState(false)
  const [ports, setPorts] = useState<Record<string, number | "">>({})
  // Port drafts sync once per load; polling never clobbers edits.
  const syncedRef = useRef(false)

  useEffect(() => {
    async function first() {
      const c = await api.getConfig().catch(() => null)
      if (c) {
        setCfg(c)
        if (!syncedRef.current) {
          setPorts({
            mariadb: c.db_mariadb_port ?? 3306,
            postgresql: c.db_pg_port ?? 5433,
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
    <div className="flex flex-col gap-4 px-4 lg:px-6">
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

      <p className="text-muted-foreground text-xs -mt-2 px-1">
        Semua database bisa hidup bersamaan — situs kamu bebas memakai koneksi mana pun
        (env <code>SABDOPALON_MARIADB_*</code> dan <code>SABDOPALON_PG_*</code> tersedia untuk semua situs).
      </p>

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
    </div>
  )
}
