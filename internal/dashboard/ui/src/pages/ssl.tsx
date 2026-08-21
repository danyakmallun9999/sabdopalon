import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Copy, KeyRound, Lock, ShieldCheck } from "lucide-react"

import api, { poll, type SslAction, type SslStatus } from "@/lib/api"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

type Step = {
  n: number
  title: string
  desc: string
  cta: string
  action: () => Promise<SslAction>
  stateKey: "ca_exists" | "wildcard_cert" | "trusted"
}

export default function SslPage() {
  const [st, setSt] = useState<SslStatus | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const [suAction, setSuAction] = useState<SslAction | null>(null)

  useEffect(() => {
    const t = poll(() => api.sslStatus().then(setSt).catch(() => {}), 6000)
    return () => clearInterval(t)
  }, [])

  const trusted = !!st?.installed && !!st?.fingerprint_match
  const staleTrust = !!st?.installed && !st?.fingerprint_match

  async function run(key: string, fn: () => Promise<SslAction>) {
    setBusy(key)
    try {
      const r = await fn()
      if (r.error) toast.error(r.error)
      else if (r.needs_su) setSuAction(r)
      else if (r.ok) toast.success(r.message ?? "Done")
      api.sslStatus().then(setSt).catch(() => {})
    } finally {
      setBusy(null)
    }
  }

  async function copyCommand(cmd?: string) {
    if (!cmd) return
    await navigator.clipboard.writeText(cmd).catch(() => {})
    toast.success("Command copied")
  }

  const steps: Step[] = [
    {
      n: 1,
      title: "Root CA",
      desc: "Create your personal local Certificate Authority. Needed once per machine.",
      cta: "Generate CA",
      action: api.sslCa,
      stateKey: "ca_exists",
    },
    {
      n: 2,
      title: "Wildcard certificate",
      desc: `Issue *.${st?.tld ?? "localhost"} so every site gets HTTPS in one go.`,
      cta: `Issue *.${st?.tld ?? ""}`,
      action: api.sslWildcard,
      stateKey: "wildcard_cert",
    },
    {
      n: 3,
      title: "Trust",
      desc: "Install the CA into your OS trust store so browsers accept it silently.",
      cta: "Trust CA",
      action: api.sslTrust,
      stateKey: "trusted",
    },
  ]

  function stepBadge(s: Step) {
    if (s.stateKey === "trusted") {
      if (!st?.installed) return <Badge variant="outline">not trusted</Badge>
      if (staleTrust) return <Badge className="bg-amber-500 text-black">stale — re-trust</Badge>
      return <Badge>trusted ✓</Badge>
    }
    const ok = s.stateKey === "ca_exists" ? st?.ca_exists : st?.wildcard_cert
    return <Badge variant={ok ? "default" : "outline"}>{ok ? "ready" : "pending"}</Badge>
  }

  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <p className="text-muted-foreground text-sm">
        Local HTTPS with a Sabdopalon root certificate — the mkcert approach, no third-party tools.
      </p>

      <div className="grid grid-cols-1 gap-4 @xl/main:grid-cols-3">
        {steps.map((s) => (
          <Card key={s.n}>
            <CardHeader>
              <div className="flex items-center gap-2">
                {s.n === 1 ? (
                  <KeyRound className="text-primary size-4" />
                ) : s.n === 2 ? (
                  <Lock className="text-primary size-4" />
                ) : (
                  <ShieldCheck className="text-primary size-4" />
                )}
                <CardTitle className="text-base">
                  {s.n} · {s.title}
                </CardTitle>
              </div>
              <CardDescription>{s.desc}</CardDescription>
              <div className="mt-3 flex items-center justify-between">
                <Button
                  size="sm"
                  disabled={
                    busy === String(s.n) ||
                    (s.n >= 2 && !st?.ca_exists) ||
                    // Already done states disable the button
                    (s.stateKey === "ca_exists" && st?.ca_exists) ||
                    (s.stateKey === "wildcard_cert" && st?.wildcard_cert) ||
                    (s.stateKey === "trusted" && trusted)
                  }
                  onClick={() => run(String(s.n), s.action)}
                >
                  {busy === String(s.n) ? "Working…" : s.cta}
                </Button>
                {stepBadge(s)}
              </div>
            </CardHeader>
          </Card>
        ))}
      </div>

      <Alert>
        <ShieldCheck />
        <AlertTitle>Firefox keeps its own certificate store</AlertTitle>
        <AlertDescription>
          Import <code>certs/sabdopalon-rootCA.crt</code> via Settings → Privacy → Certificates →
          View Certificates → Authorities.
        </AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle>Status</CardTitle>
        </CardHeader>
        <div className="px-4 pb-4 lg:px-6">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="w-1/3">Check</TableHead>
                <TableHead>Result</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow>
                <TableCell>Wildcard cert</TableCell>
                <TableCell>{st?.wildcard_cert ? "✓ present" : "— missing"}</TableCell>
              </TableRow>
              <TableRow>
                <TableCell>OS trust store</TableCell>
                <TableCell>{st?.installed ? "✓ installed" : "— not installed"}</TableCell>
              </TableRow>
              <TableRow>
                <TableCell>Fingerprint match</TableCell>
                <TableCell>
                  {!st?.installed ? "—" : st?.fingerprint_match ? "✓ matches" : "⚠ mismatch"}
                </TableCell>
              </TableRow>
              <TableRow>
                <TableCell>HTTPS port</TableCell>
                <TableCell>{st?.https_port}</TableCell>
              </TableRow>
              {st?.detail && (
                <TableRow>
                  <TableCell>Detail</TableCell>
                  <TableCell className="text-amber-500">{st.detail}</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </Card>

      <Dialog open={!!suAction} onOpenChange={(v) => !v && setSuAction(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Elevation required</DialogTitle>
            <DialogDescription>{suAction?.message}</DialogDescription>
          </DialogHeader>
          <div className="flex items-center gap-2">
            <code className="bg-muted flex-1 rounded-md p-3 font-mono text-xs break-all">
              {suAction?.command}
            </code>
            <Button size="icon" variant="outline" onClick={() => copyCommand(suAction?.command)}>
              <Copy />
              <span className="sr-only">Copy command</span>
            </Button>
          </div>
          {suAction?.note && (
            <p className="text-muted-foreground text-xs">{suAction.note}</p>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
