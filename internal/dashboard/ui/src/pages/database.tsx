import { useEffect, useState } from "react"
import { toast } from "sonner"
import { HardDriveDownload } from "lucide-react"

import api, { poll, type Backup, type Status } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

export default function DatabasePage() {
  const engine = "sqlite"
  const [status, setStatus] = useState<Status | null>(null)
  const [backups, setBackups] = useState<Backup[]>([])
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    const t = poll(async () => {
      api.status().then(setStatus).catch(() => {})
      const b = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(b) ? b : [])
    }, 6000)
    return () => clearInterval(t)
  }, [])

  async function doBackup() {
    setBusy(true)
    try {
      const r = await api.backupNow()
      if (r.error) toast.error(r.error)
      else toast.success(r.message ?? `Backup created: ${r.backup}`)
      const list = await api.listBackups().catch(() => [])
      setBackups(Array.isArray(list) ? list : [])
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div className="flex flex-col gap-1.5">
              <CardTitle>Engine</CardTitle>
              <CardDescription>
                {engine === "sqlite"
                  ? "SQLite works with zero setup — the file lives in data/sabdopalon.db."
                  : `The ${engine} daemon starts and stops together with Sabdopalon.`}
              </CardDescription>
            </div>
            <Badge variant={engine === "sqlite" ? "secondary" : "default"}>
              {status?.database ?? engine}
            </Badge>
          </div>
        </CardHeader>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <div className="flex flex-col gap-1.5">
            <CardTitle>Backups</CardTitle>
            <CardDescription>Oldest backups are pruned automatically (keep 5).</CardDescription>
          </div>
          <Button onClick={doBackup} disabled={busy}>
            <HardDriveDownload /> Backup Now
          </Button>
        </CardHeader>
        <div className="px-4 pb-4 lg:px-6">
          {backups.length === 0 ? (
            <p className="text-muted-foreground border-dashed rounded-xl border p-8 text-center text-sm">
              No backups yet — click “Backup Now”.
            </p>
          ) : (
            <div className="bg-card overflow-hidden rounded-xl border">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>Name</TableHead>
                    <TableHead>Size</TableHead>
                    <TableHead>Created</TableHead>
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
