import TerminalPanel from "@/components/terminal-panel"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

// TerminalPage is the full-page terminal — the same TerminalPanel used per
// site, opened at the Sabdopalon sites root.
export default function TerminalPage() {
  return (
    <div className="flex flex-col gap-4 px-4 lg:px-6">
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Terminal</CardTitle>
          <CardDescription>
            Shell with Sabdopalon&apos;s bin/ on PATH — run php, mysql, composer, anything.
          </CardDescription>
          <div className="mt-2">
            <TerminalPanel heightClass="h-[calc(100vh-16rem)] min-h-[24rem]" />
          </div>
        </CardHeader>
      </Card>
    </div>
  )
}
