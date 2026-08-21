import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { CheckCircle2, Download } from "lucide-react"

import api, { poll, type InstallJob, type Package } from "@/lib/api"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"

const LABELS: Record<string, string> = {
  php: "Default PHP (auto-used by all sites)",
  php81: "PHP 8.1",
  php82: "PHP 8.2",
  php83: "PHP 8.3",
  php84: "PHP 8.4",
  php85: "PHP 8.5",
  mariadb: "MariaDB server (auto-managed daemon)",
  mailpit: "Mailpit — local e-mail catcher",
}

export default function PackagesPage() {
  const [pkgs, setPkgs] = useState<Package[]>([])
  const [job, setJob] = useState<InstallJob | null>(null)
  const [progress, setProgress] = useState(15)
  const logRef = useRef<HTMLPreElement>(null)

  const load = () => api.listPackages().then(setPkgs).catch(() => {})

  useEffect(() => {
    const t = poll(load, 6000)
    return () => clearInterval(t)
  }, [])

  // Poll the install job only while it runs; animate a soft progress bar.
  useEffect(() => {
    if (!job?.running) {
      setProgress((p) => (job?.done ? 100 : p))
      if (!job?.running) return
    }
    const t = setInterval(() => {
      setProgress((p) => Math.min(p + 2 + Math.random() * 6, 95))
      api.installJob().then(setJob).catch(() => {})
    }, 700)
    return () => clearInterval(t)
  }, [job?.running])

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
    if (job?.done && !job?.running) {
      if (job.error) toast.error(job.error)
      else toast.success(`Installed ✓`)
      load()
    }
  }, [job?.done, job?.running])

  async function install(name: string) {
    setProgress(10)
    const r = await api.installPackage(name)
    if (r.error) {
      toast.error(r.error)
      return
    }
    setJob({ name, running: true, done: false, output: "" })
  }

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <p className="text-muted-foreground text-sm">
        Optional components, downloaded and managed inside{" "}
        <code className="bg-muted rounded px-1.5 py-0.5">bin/</code> — nothing touches your OS.
      </p>

      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-2 @5xl/main:grid-cols-3">
        {pkgs.map((p) => (
          <Card key={p.name}>
            <CardHeader>
              <div className="flex items-start justify-between gap-2">
                <div className="flex flex-col gap-1.5">
                  <CardTitle className="text-base">{LABELS[p.name] ?? p.name}</CardTitle>
                  <CardDescription>
                    v{p.version}
                    {p.license ? ` · ${p.license}` : ""}
                  </CardDescription>
                </div>
                <Badge variant={p.installed ? "default" : "outline"}>
                  {p.installed ? "installed" : "not installed"}
                </Badge>
              </div>
              <Button
                className="mt-3 w-fit"
                variant={p.installed ? "secondary" : "default"}
                disabled={p.installed || !!job?.running}
                onClick={() => install(p.name)}
              >
                {p.installed ? <CheckCircle2 /> : <Download />}
                {p.installed ? "Installed" : "Install"}
              </Button>
            </CardHeader>
          </Card>
        ))}
      </div>

      {job && job.running && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">Installing {job.name}…</CardTitle>
              <Progress value={progress} className="w-40" />
            </div>
            <pre
              ref={logRef}
              className="bg-background text-muted-foreground max-h-56 overflow-y-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap"
            >
              {job.output || "starting…"}
            </pre>
          </CardHeader>
        </Card>
      )}
    </div>
  )
}
