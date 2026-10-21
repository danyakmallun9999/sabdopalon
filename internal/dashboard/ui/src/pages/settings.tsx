import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Save, FileText } from "lucide-react"

import api, { type ConfigPayload as Config, type Profile, type PhpIni } from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
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
  const [ini, setIni] = useState<PhpIni | null>(null)
  const [iniContent, setIniContent] = useState("")
  const [savingIni, setSavingIni] = useState(false)

  useEffect(() => {
    api.getConfig().then(setCfg).catch(() => {})
    api.listProfiles().then((p) => setProfiles(Array.isArray(p) ? p : [])).catch(() => {})
    api.getPhpIni().then((d) => { setIni(d); setIniContent(d.content) }).catch(() => {})
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

  async function savePhpIni() {
    setSavingIni(true)
    try {
      const r = await api.savePhpIni(iniContent)
      if (r.error) toast.error(r.error)
      else toast.success(r.message)
    } finally {
      setSavingIni(false)
    }
  }

  return (
    <div className="flex flex-col gap-5 px-5 py-7 lg:px-7">
      {/*
        One card, one form, one button.

        The TLD, the fallback ports, the dashboard port/auto-open and the LAN
        switch are all fields of the same ConfigPayload, and a single save()
        writes all of them. Splitting them across three cards made the button
        look like it belonged to whichever card happened to hold it (Dashboard),
        while the LAN switch sat below that button and looked like it saved
        itself. One card with the button in its footer is what the code does.
      */}
      
      <div className="grid grid-cols-1 gap-4 @4xl/main:grid-cols-12">
        <Card className="min-w-0 @4xl/main:col-span-5">
        <CardHeader>
          <CardTitle>General</CardTitle>
          <CardDescription>
            Stored in config/engine.toml. Port, TLD and database changes apply after a restart.
          </CardDescription>
        </CardHeader>

        <CardContent className="flex flex-col gap-5">
          <section className="flex flex-col gap-3 border-b border-border-soft pb-5 last:border-0 last:pb-0">
            <h3 className="text-[13.5px] font-semibold text-text-primary">Domain and ports</h3>
            <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2">
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="tld">Domain suffix (TLD)</Label>
                <Input id="tld" value={cfg.tld ?? ""} onChange={(e) => set("tld", e.target.value)} />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="dashport">Dashboard port</Label>
                <Input
                  id="dashport"
                  type="number"
                  value={cfg.dashboard_port ?? ""}
                  onChange={(e) => set("dashboard_port", Number(e.target.value))}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="http">HTTP port (fallback)</Label>
                <Input
                  id="http"
                  type="number"
                  value={cfg.http_port ?? ""}
                  onChange={(e) => set("http_port", Number(e.target.value))}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="https">HTTPS port (fallback)</Label>
                <Input
                  id="https"
                  type="number"
                  value={cfg.https_port ?? ""}
                  onChange={(e) => set("https_port", Number(e.target.value))}
                />
              </div>
            </div>
            <p className="text-[13.5px] leading-relaxed text-text-muted">
              Sabdopalon automatically tries ports 80/443 for clean URLs first; these are fallbacks.
              Enable permanently: <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[13.5px] text-text-secondary">sabdopalon enable-ports</code>.
            </p>
          </section>

          <section className="flex flex-col gap-3 border-b border-border-soft pb-5 last:border-0 last:pb-0">
            <h3 className="text-[13.5px] font-semibold text-text-primary">Dashboard</h3>
            <div className="flex flex-row items-center justify-between gap-4 rounded-md border border-ring-inner bg-surface-inset px-3 py-2.5">
              <div className="flex flex-col gap-1">
                <Label htmlFor="autoopen" className="font-normal">
                  Open dashboard on start
                </Label>
                <p className="text-[13.5px] leading-relaxed text-text-muted">
                  Opens the dashboard in your browser when Sabdopalon starts.
                </p>
              </div>
              <Switch
                id="autoopen"
                checked={!!cfg.auto_open}
                onCheckedChange={(v) => set("auto_open", v)}
              />
            </div>
          </section>

          {/* Security: PHP sites execute code, so LAN access must be opt-in. */}
          <section className="flex flex-col gap-3 border-b border-border-soft pb-5 last:border-0 last:pb-0">
            <h3 className="text-[13.5px] font-semibold text-text-primary">Network access</h3>
            <div className="flex flex-row items-center justify-between gap-4 rounded-md border border-ring-inner bg-surface-inset px-3 py-2.5">
              <div className="flex flex-col gap-1">
                <Label htmlFor="lan" className="font-normal">
                  Allow other devices on the network to open your sites
                </Label>
                <p className="text-[13.5px] leading-relaxed text-text-muted">
                  Exposes the proxy ports above to your network; only enable this on a Wi-Fi you
                  trust. Applies after restart.
                </p>
              </div>
              <Switch
                id="lan"
                checked={!!cfg.lan}
                onCheckedChange={(v) => set("lan", v)}
              />
            </div>
          </section>
        </CardContent>

        <CardFooter className="justify-end border-t border-border-soft">
          <Button onClick={save} disabled={saving}>
            <Save /> Save settings
          </Button>
        </CardFooter>
      </Card>

      <Card className="min-w-0 @4xl/main:col-span-7">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <FileText className="size-4 text-text-muted" /> PHP configuration (php.ini)
          </CardTitle>
          <CardDescription>
            This file applies to all sites (passed via PHPRC).{" "}
            {ini?.version && (
              <>Active PHP: <code className="rounded bg-surface-raised px-1 py-0.5 font-mono text-[13.5px] text-text-secondary">{ini.version}</code>. </>
            )}
            Stored at <code className="break-all rounded bg-surface-raised px-1 py-0.5 font-mono text-[13.5px] text-text-secondary">{ini?.path ?? "config/php.ini"}</code>.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-1 flex-col gap-2">
          <Textarea
            className="min-h-[300px] flex-1 font-mono text-[13.5px] leading-relaxed"
            value={iniContent}
            onChange={(e) => setIniContent(e.target.value)}
            spellCheck={false}
            placeholder="memory_limit = 256M&#10;upload_max_filesize = 64M&#10;..."
          />
          <div className="flex items-center gap-2">
            <Button className="w-fit" variant="outline" onClick={savePhpIni} disabled={savingIni}>
              <Save /> Save php.ini
            </Button>
            <span className="text-[13.5px] text-text-muted">
              Saving will restart running sites.
            </span>
          </div>
        </CardContent>
      </Card>


      <Card className="min-w-0 @4xl/main:col-span-12">
        <CardHeader>
          <CardTitle>Profiles</CardTitle>
          <CardDescription>
            Named PHP/database presets. Applying stops running sites so they pick up the new PHP
            binary.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-hidden rounded-xl border border-ring-inner">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>Name</TableHead>
                  <TableHead>PHP</TableHead>
                  <TableHead>DB</TableHead>
                  <TableHead className="w-24 text-right">
                    <span className="sr-only">Actions</span>
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {profiles.map((p) => (
                  <TableRow key={p.Name}>
                    <TableCell className="font-medium">
                      {p.Name}
                      {p.Description && (
                        <span className="ml-2 text-[13.5px] text-text-muted">{p.Description}</span>
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
          </div>
        </CardContent>
      </Card>
      </div>
    </div>
  )
}
