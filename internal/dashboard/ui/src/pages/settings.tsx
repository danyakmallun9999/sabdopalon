import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Save } from "lucide-react"

import api, { type ConfigPayload as Config, type Profile } from "@/lib/api"
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

export default function SettingsPage() {
  const [cfg, setCfg] = useState<Config>({})
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.getConfig().then(setCfg).catch(() => {})
    api.listProfiles().then((p) => setProfiles(Array.isArray(p) ? p : [])).catch(() => {})
  }, [])

  function set<K extends keyof Config>(key: K, value: Config[K]) {
    setCfg((c) => ({ ...c, [key]: value }))
  }

  async function save() {
    setSaving(true)
    try {
      const r = await api.saveConfig(cfg)
      if (r.error) toast.error(r.error)
      else toast.success(r.message)
    } finally {
      setSaving(false)
    }
  }

  async function applyProfile(name: string) {
    if (!confirm(`Apply profile "${name}"? Running sites will restart.`)) return
    const r = await api.applyProfile(name)
    if (r.error) toast.error(r.error)
    else toast.success(r.message ?? "Applied")
  }

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <Card>
        <CardHeader>
          <CardTitle>General</CardTitle>
          <CardDescription>
            Stored in config/engine.toml. Port/TLD/database changes apply after a restart.
          </CardDescription>
          <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="tld">Domain suffix (TLD)</Label>
              <Input id="tld" value={cfg.tld ?? ""} onChange={(e) => set("tld", e.target.value)} />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="http">HTTP port (fallback)</Label>
              <Input
                id="http"
                type="number"
                value={cfg.http_port ?? ""}
                onChange={(e) => set("http_port", Number(e.target.value))}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="https">HTTPS port (fallback)</Label>
              <Input
                id="https"
                type="number"
                value={cfg.https_port ?? ""}
                onChange={(e) => set("https_port", Number(e.target.value))}
              />
            </div>
          </div>
          <p className="text-muted-foreground text-xs">
            Sabdopalon automatically tries ports 80/443 for clean URLs first — these are fallbacks.
            Enable permanently: <code>sabdopalon enable-ports</code>.
          </p>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Dashboard</CardTitle>
          <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-3">
            <div className="flex flex-col gap-2">
              <Label htmlFor="dashport">Dashboard port</Label>
              <Input
                id="dashport"
                type="number"
                value={cfg.dashboard_port ?? ""}
                onChange={(e) => set("dashboard_port", Number(e.target.value))}
              />
            </div>
            <div className="flex flex-row items-center justify-between rounded-lg border p-3">
              <Label htmlFor="autoopen" className="font-normal">
                Open dashboard on start
              </Label>
              <Switch
                id="autoopen"
                checked={!!cfg.auto_open}
                onCheckedChange={(v) => set("auto_open", v)}
              />
            </div>
          </div>
          <Button className="mt-3 w-fit" onClick={save} disabled={saving}>
            <Save /> Save settings
          </Button>
        </CardHeader>
      </Card>

      {/* Keamanan: situs PHP mengeksekusi kode — akses LAN harus opt-in */}
      <Card>
        <CardHeader>
          <CardTitle>Akses jaringan (LAN)</CardTitle>
          <div className="mt-2 flex flex-row items-center justify-between gap-4 rounded-lg border p-3">
            <div className="flex flex-col gap-1">
              <Label htmlFor="lan" className="font-normal">
                Izinkan perangkat lain di jaringan membuka situs kamu
              </Label>
              <p className="text-muted-foreground text-xs">
                Nonaktif (default): situs hanya bisa dibuka dari komputer ini
                (127.0.0.1). Mengaktifkan ini membuka port 8080/8443 ke
                jaringan — pastikan Wi-Fi kamu terpercaya. Berlaku setelah
                restart.
              </p>
            </div>
            <Switch
              id="lan"
              checked={!!cfg.lan}
              onCheckedChange={(v) => set("lan", v)}
            />
          </div>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Profiles</CardTitle>
          <CardDescription>
            Named PHP/database presets. Applying stops running sites so they pick up the new PHP
            binary.
          </CardDescription>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead>Name</TableHead>
                <TableHead>PHP</TableHead>
                <TableHead>DB</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {profiles.map((p) => (
                <TableRow key={p.Name}>
                  <TableCell className="font-medium">
                    {p.Name}
                    {p.Description && (
                      <span className="text-muted-foreground ml-2 text-xs">{p.Description}</span>
                    )}
                  </TableCell>
                  <TableCell>{p.PHP || "(default)"}</TableCell>
                  <TableCell>{p.DBEngine || "(default)"}</TableCell>
                  <TableCell className="text-right">
                    {p.Name !== "default" && (
                      <Button size="sm" variant="outline" onClick={() => applyProfile(p.Name)}>
                        Apply
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardHeader>
      </Card>
    </div>
  )
}
