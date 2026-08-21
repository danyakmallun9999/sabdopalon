import { useEffect, useState } from "react"
import { useSearchParams } from "react-router-dom"
import { ScrollText } from "lucide-react"

import { Switch } from "@/components/ui/switch"

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
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <Tabs value={current} onValueChange={setCurrent}>
        <div className="flex items-center justify-between gap-2">
          <TabsList className="flex-wrap">
            {sites.map((n) => (
              <TabsTrigger key={n} value={n}>
                <ScrollText className="size-3.5" />
                {n}
              </TabsTrigger>
            ))}
          </TabsList>
          <label className="text-muted-foreground flex items-center gap-2 text-xs">
            auto-refresh
            <Switch checked={auto} onCheckedChange={setAuto} id="log-auto" />
          </label>
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
