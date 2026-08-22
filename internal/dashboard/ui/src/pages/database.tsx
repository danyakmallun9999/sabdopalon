import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { HardDriveDownload, Play, RotateCw, Square } from "lucide-react"

import api, { poll, type Backup, type ConfigPayload, type Status } from "@/lib/api"
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

type DbAction = "start" | "stop" | "restart"

// Engines offered in the picker. MySQL is deliberately absent — MariaDB is a
// drop-in replacement; a legacy config value still renders correctly below.
const ENGINES = [
  { id: "sqlite", label: "SQLite" },
  { id: "mariadb", label: "MariaDB" },
  { id: "postgresql", label: "PostgreSQL" },
]

export default function DatabasePage() {
  const [cfg, setCfg] = useState<ConfigPayload>({})
  const [status, setStatus] = useState<Status | null>(null)
  const [backups, setBackups] = useState<Backup[]>([])
  const [busyAction, setBusyAction] = useState<DbAction | null>(null)
  const [busyBackup, setBusyBackup] = useState(false)
  const [saving, setSaving] = useState(false)

  // Draft edits are saved explicitly (engine/port need a restart anyway).
  const [draftEngine, setDraftEngine] = useState("sqlite")
  const [draftPort, setDraftPort] = useState<number | "">("")
  // Sync drafts only once per fresh page load — never while the user edits.
  const syncedRef = useRef(false)

  useEffect(() => {
    async function first() {
      const c = await api.getConfig().catch(() => null)
      if (c) {
        setCfg(c)
        if (!syncedRef.current) {
          setDraftEngine(c.db_engine || "sqlite")
          setDraftPort(c.db_port ?? "")
          syncedRef.current = true
        }
      }
      setStatus(await api.status().catch(() => null))
      const b = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(b) ? b : [])
    }
    first()
    const t = poll(async () => {
      api.getConfig().then(setCfg).catch(() => {})
      api.status().then(setStatus).catch(() => {})
      const b = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(b) ? b : [])
    }, 6000)
    return () => clearInterval(t)
  }, [])

  const engine = cfg.db_engine || "sqlite"
  const running =
    engine === "sqlite" || engine === "" ? true : (status?.db_running ?? false)

  async function save() {
    setSaving(true)
    try {
      await api.saveConfig({
        db_engine: draftEngine,
        db_port: draftPort === "" ? undefined : Number(draftPort),
      })
      toast.success("Pengaturan database disimpan", {
        description:
          draftEngine !== engine
            ? `Engine ${engine} → ${draftEngine}. Restart Sabdopalon untuk menerapkan.`
            : "Perubahan port berlaku setelah restart.",
      })
    } catch {
      toast.error("Gagal menyimpan pengaturan")
    } finally {
      setSaving(false)
    }
  }

  async function doAction(a: DbAction) {
    setBusyAction(a)
    try {
      const r = await api.databaseControl(a)
      if (r.error) toast.error(`${a}: ${r.error}`)
      else toast.success(`Database ${a} — OK`)
      setStatus(await api.status().catch(() => null))
    } finally {
      setBusyAction(null)
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

  const dirty =
    draftEngine !== engine || Number(draftPort) !== (cfg.db_port ?? -1)

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <Card>
        <CardHeader>
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="flex flex-col gap-1.5">
              <CardTitle>Engine</CardTitle>
              <CardDescription>
                {engine === "sqlite"
                  ? "SQLite siap pakai tanpa setup — filenya di data/sabdopalon.db."
                  : `Daemon ${engine} berjalan sebagai bagian dari Sabdopalon.`}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              <Badge variant={running ? "default" : "outline"}>
                {running ? "berjalan" : "berhenti"}
              </Badge>
              <span className="text-muted-foreground text-sm font-medium">{engine}</span>
            </div>
          </div>

          {/* Lifecycle controls for daemon engines */}
          {engine !== "sqlite" && engine !== "" && (
            <div className="mt-3 flex items-center gap-2">
              {!running && (
                <Button size="sm" disabled={busyAction !== null} onClick={() => doAction("start")}>
                  <Play /> Start
                </Button>
              )}
              {running && (
                <Button size="sm" variant="outline" disabled={busyAction !== null} onClick={() => doAction("stop")}>
                  <Square /> Stop
                </Button>
              )}
              <Button
                size="sm"
                variant="outline"
                disabled={busyAction !== null || !running}
                onClick={() => doAction("restart")}
              >
                <RotateCw /> Restart
              </Button>
              {busyAction && (
                <span className="text-muted-foreground text-xs">{busyAction}…</span>
              )}
            </div>
          )}
          {cfg.db_error && (
            <p className="text-destructive mt-2 text-xs">Gagal start: {cfg.db_error}</p>
          )}

          {/* Engine + port picker — saved explicitly; needs restart */}
          <div className="mt-4 grid grid-cols-1 items-end gap-3 @xl/main:grid-cols-[1fr_10rem_auto]">
            <div className="flex flex-col gap-2">
              <Label>Engine database</Label>
              <Select value={ENGINES.some((e) => e.id === draftEngine) ? draftEngine : "__legacy"} onValueChange={(v) => { if (v) setDraftEngine(v) }}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {ENGINES.map((e) => (
                    <SelectItem key={e.id} value={e.id}>
                      {e.label}
                      {cfg.db_installed?.[e.id] === false ? " — belum terpasang (Packages)" : ""}
                    </SelectItem>
                  ))}
                  {!ENGINES.some((e) => e.id === draftEngine) && (
                    <SelectItem value="__legacy">{draftEngine}</SelectItem>
                  )}
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="dbport">Port</Label>
              <Input
                id="dbport"
                type="number"
                disabled={draftEngine === "sqlite"}
                value={draftPort}
                onChange={(e) =>
                  setDraftPort(e.target.value === "" ? "" : Number(e.target.value))
                }
              />
            </div>
            <Button onClick={save} disabled={!dirty || saving}>
              {saving ? "Menyimpan…" : "Simpan"}
            </Button>
          </div>
          <p className="text-muted-foreground text-xs">
            Perubahan engine/port berlaku setelah restart Sabdopalon. Engine belum terpasang?
            Pasang dulu di halaman Packages.
          </p>
        </CardHeader>
      </Card>

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
