import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Save } from "lucide-react"

import api, {
  type ConfigPayload as Config,
  type MailpitStatus,
  type Profile,
} from "@/lib/api"
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
  const [mailpit, setMailpit] = useState<MailpitStatus | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.getConfig().then(setCfg).catch(() => {})
    api.listProfiles().then((p) => setProfiles(Array.isArray(p) ? p : [])).catch(() => {})
    api.services().then((s) => setMailpit(s.mailpit)).catch(() => {})
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
          <CardTitle>Database</CardTitle>
          <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2">
            <div className="flex flex-col gap-2">
              <Label>Engine</Label>
              <Select value={cfg.db_engine ?? "sqlite"} onValueChange={(v) => set("db_engine", v ?? "sqlite")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="sqlite">SQLite — zero setup</SelectItem>
                  <SelectItem value="mariadb">MariaDB (install from Packages)</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="dbport">Port (mariadb/mysql)</Label>
              <Input
                id="dbport"
                type="number"
                value={cfg.db_port ?? ""}
                onChange={(e) => set("db_port", Number(e.target.value))}
              />
            </div>
          </div>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Dashboard &amp; services</CardTitle>
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
            <div className="flex flex-row items-center justify-between rounded-lg border p-3">
              <Label htmlFor="mailpit" className="font-normal">
                Mailpit catcher
                {mailpit?.running && (
                  <span className="text-muted-foreground block text-xs">running now</span>
                )}
              </Label>
              <Switch
                id="mailpit"
                checked={!!cfg.mailpit_enabled}
                onCheckedChange={async (v) => {
                  set("mailpit_enabled", v)
                  await api.toggleMailpit(v)
                }}
              />
            </div>
          </div>
          <Button className="mt-3 w-fit" onClick={save} disabled={saving}>
            <Save /> Save settings
          </Button>
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
