import { useEffect, useRef, useState } from "react"
import { toast } from "sonner"
import { CheckCircle2, LoaderCircle, Rocket } from "lucide-react"

import api, { type SetupJob } from "@/lib/api"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Label } from "@/components/ui/label"
import { Progress } from "@/components/ui/progress"
import { Switch } from "@/components/ui/switch"

export default function SetupPage() {
  const [installMariaDB, setInstallMariaDB] = useState(true)
  const [installPostgres, setInstallPostgres] = useState(false)
  const [createSample, setCreateSample] = useState(true)
  const [job, setJob] = useState<SetupJob | null>(null)
  const [progress, setProgress] = useState(10)
  const logRef = useRef<HTMLPreElement>(null)

  // Poll the setup job while it runs.
  useEffect(() => {
    if (!job?.running) {
      setProgress((p) => (job?.done ? 100 : p))
      return
    }
    const t = setInterval(() => {
      setProgress((p) => Math.min(p + 1.5 + Math.random() * 4, 95))
      api.setupJob().then(setJob).catch(() => {})
    }, 800)
    return () => clearInterval(t)
  }, [job?.running])

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight })
    if (job?.done && !job?.running) {
      if (job.error) {
        toast.error(job.error)
      } else {
        toast.success("Sabdopalon is ready! Reloading…")
        setTimeout(() => window.location.reload(), 1200)
      }
    }
  }, [job?.done, job?.running])

  async function start() {
    setProgress(5)
    const r = await api.runSetup({
      install_mariadb: installMariaDB,
      install_postgres: installPostgres,
      create_sample_site: createSample,
      db_engine: installMariaDB ? "mariadb" : "sqlite",
      http_port: 8080,
      https_port: 8443,
      tld: "localhost",
    })
    if (r.error) {
      toast.error(r.error)
      return
    }
    setJob({ running: true, done: false, output: "" })
  }

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <div className="max-w-2xl">
        <h2 className="text-2xl font-semibold">Welcome to Sabdopalon 🐫</h2>
        <p className="text-muted-foreground mt-1 text-sm">
          Your local PHP development server — everything lives inside one folder, nothing touches
          your system. Pick your stack and we&apos;ll install it in a few minutes.
        </p>
      </div>

      {!job ? (
        <div className="flex max-w-2xl flex-col gap-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Core stack</CardTitle>
              <CardDescription>
                PHP + MariaDB are the default — the classic local dev combo for WordPress and
                Laravel. All optional.
              </CardDescription>
              <div className="mt-3 flex flex-col gap-3">
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <Label htmlFor="setup-mariadb" className="font-normal">
                    MariaDB (MySQL-compatible database)
                  </Label>
                  <Switch
                    id="setup-mariadb"
                    checked={installMariaDB}
                    onCheckedChange={setInstallMariaDB}
                  />
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <Label htmlFor="setup-postgres" className="font-normal">
                    PostgreSQL (optional)
                  </Label>
                  <Switch
                    id="setup-postgres"
                    checked={installPostgres}
                    onCheckedChange={setInstallPostgres}
                  />
                </div>
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <Label htmlFor="setup-sample" className="font-normal">
                    Create a sample site to get started
                  </Label>
                  <Switch
                    id="setup-sample"
                    checked={createSample}
                    onCheckedChange={setCreateSample}
                  />
                </div>
              </div>
            </CardHeader>
          </Card>

          <Button size="lg" className="w-fit" onClick={start}>
            <Rocket /> Install &amp; Start
          </Button>
          <p className="text-muted-foreground text-xs">
            Downloads are verified (SHA-256) and kept inside the Sabdopalon folder.
          </p>
        </div>
      ) : (
        <Card className="max-w-2xl">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">
                {job.done && !job.error ? (
                  <span className="inline-flex items-center gap-2 text-emerald-500">
                    <CheckCircle2 /> Ready!
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-2">
                    {job.running && <LoaderCircle className="size-4 animate-spin" />}
                    Setting up Sabdopalon…
                  </span>
                )}
              </CardTitle>
              <Progress value={progress} className="w-40" />
            </div>
            <pre
              ref={logRef}
              className="bg-background text-muted-foreground max-h-72 overflow-y-auto rounded-lg border p-3 font-mono text-xs whitespace-pre-wrap"
            >
              {job.output || "starting…"}
            </pre>
            {job.error && (
              <p className="text-destructive text-sm">
                Setup failed: {job.error} — you can retry from the Packages page later.
              </p>
            )}
          </CardHeader>
        </Card>
      )}
    </div>
  )
}
