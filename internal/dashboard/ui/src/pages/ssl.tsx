import { useEffect, useState } from "react"
import { toast } from "sonner"
import { Copy, KeyRound, Lock, ShieldCheck } from "lucide-react"

import api, { poll, type SslAction, type SslStatus } from "@/lib/api"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { ToneBadge } from "@/components/status"
import { PageHeader } from "@/components/page-header"
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
      if (!st?.installed) return <ToneBadge tone="muted">not trusted</ToneBadge>
      if (staleTrust) return <ToneBadge tone="warning">stale — re-trust</ToneBadge>
      return <ToneBadge tone="success">trusted ✓</ToneBadge>
    }
    const ok = s.stateKey === "ca_exists" ? st?.ca_exists : st?.wildcard_cert
    return <ToneBadge tone={ok ? "success" : "muted"}>{ok ? "ready" : "pending"}</ToneBadge>
  }

  return (
    <div className="flex flex-col gap-5 px-5 py-7 lg:px-7">
      <PageHeader
        title="SSL / HTTPS"
        description="A local certificate authority makes every site trusted in the browser — without warnings."
      />
      <div className="grid grid-cols-1 gap-3.5 @xl/main:grid-cols-3">
        {steps.map((s) => (
          <Card key={s.n}>
            <CardHeader>
              <div className="flex items-center gap-2">
                {s.n === 1 ? (
                  <KeyRound className="size-[15px] text-brand" />
                ) : s.n === 2 ? (
                  <Lock className="size-[15px] text-brand" />
                ) : (
                  <ShieldCheck className="size-[15px] text-brand" />
                )}
                <CardTitle>
                  {s.n} · {s.title}
                </CardTitle>
              </div>
              <CardDescription>{s.desc}</CardDescription>
              <div className="mt-3 flex items-center justify-between">
                <Button
                  size="sm"
                  variant={
                    (s.stateKey === "ca_exists" && st?.ca_exists) ||
                    (s.stateKey === "wildcard_cert" && st?.wildcard_cert)
                      ? "secondary"
                      : "default"
                  }
                  title="Click to regenerate / re-apply"
                  disabled={busy === String(s.n) || (s.n >= 2 && !st?.ca_exists)}
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
        <div className="px-(--card-pad-x) pb-(--card-pad-y)">
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
                <TableCell>Trust store</TableCell>
                <TableCell>
                  {st?.installed
                    ? `✓ ${st.source === "user" ? "user (no admin)" : "system"}`
                    : "— not installed"}
                </TableCell>
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
                  <TableCell className="text-warning">{st.detail}</TableCell>
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
            <code className="flex-1 rounded-xl border border-ring-inner bg-surface-inset p-3 font-mono text-[11.5px] break-all text-text-secondary">
              {suAction?.command}
            </code>
            <Button size="icon" variant="outline" onClick={() => copyCommand(suAction?.command)}>
              <Copy />
              <span className="sr-only">Copy command</span>
            </Button>
          </div>
          {suAction?.note && (
            <p className="text-[11px] leading-relaxed text-text-muted">{suAction.note}</p>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
