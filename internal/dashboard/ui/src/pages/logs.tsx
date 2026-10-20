import { useEffect, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { ScrollText } from "lucide-react"

import { Switch } from "@/components/ui/switch"
import { PageHeader } from "@/components/page-header"

import api, { type LogResponse } from "@/lib/api"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"

export default function LogsPage() {
  const [sites, setSites] = useState<string[]>([])
  const [current, setCurrent] = useState<string>("")
  const [log, setLog] = useState<LogResponse | null>(null)
  const [auto, setAuto] = useState(true)
  const [params, setParams] = useSearchParams()

  useEffect(() => {
    api
      .listSites()
      .then((list) => {
        const names = Array.isArray(list) ? list.map((s) => s.name).concat(["database", "mailpit"]) : []
        setSites(names)
        const fromUrl = params.get("site")
        const initial = fromUrl && names.includes(fromUrl) ? fromUrl : (names[0] ?? "")
        setCurrent((c) => c || initial)
      })
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!current) return
    if (params.get("site") !== current) {
      setParams(current === "database" || current === "mailpit" ? {} : { site: current }, { replace: true })
    }
    const tick = () => api.logs(current).then(setLog).catch(() => setLog(null))
    tick()
    if (!auto) return
    const t = setInterval(tick, 2500)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current, auto])

  return (
    <div className="flex flex-col gap-5 px-5 py-7 lg:px-7">
      <PageHeader
        title="Logs"
        description="Follow a site, the database, or Mailpit without opening a terminal."
        actions={
          <label className="flex items-center gap-2 text-[11px] text-text-muted">
            auto-refresh
            <Switch checked={auto} onCheckedChange={setAuto} id="log-auto" />
          </label>
        }
      />
      <Tabs value={current} onValueChange={setCurrent}>
        <div className="max-w-full overflow-x-auto">
          <TabsList>
            {sites.map((n) => (
              <TabsTrigger key={n} value={n} className="flex-none">
                <ScrollText className="size-3.5" />
                {n}
              </TabsTrigger>
            ))}
          </TabsList>
        </div>

        {sites.map((n) => (
          <TabsContent key={n} value={n}>
            {current === n && (
              <>
                <p className="text-muted-foreground mb-2 font-mono text-xs">
                  logs/{log?.file ?? "…"}
                </p>
                <pre className="bg-background text-muted-foreground max-h-[60vh] min-h-56 overflow-y-auto rounded-lg border p-3 font-mono text-xs leading-relaxed">
                  {log?.error ?? log?.lines.join("\n") ?? "No logs yet."}
                </pre>
              </>
            )}
          </TabsContent>
        ))}
      </Tabs>
    </div>
  )
}
