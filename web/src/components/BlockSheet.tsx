import { useEffect, useState } from "react"
import { ShieldAlert, ShieldCheck } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Hash } from "@/components/Hash"
import type { Block } from "@/lib/api"
import { short } from "@/lib/format"
import { sha256Hex } from "@/lib/hash"

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-[10px] tracking-[0.25em] text-muted-foreground">{label}</div>
      <div className="text-sm">{children}</div>
    </div>
  )
}

export function BlockSheet({ block, onClose }: { block: Block | null; onClose: () => void }) {
  const [verified, setVerified] = useState<boolean | null>(null)

  useEffect(() => {
    if (!block) return
    let off = false
    setVerified(null)
    // identical to core.Block.ComputeHash in Go
    sha256Hex(
      `${block.index}|${block.timestamp}|${block.prevHash}|${block.merkleRoot}|${block.difficulty}|${block.nonce}`
    ).then((h) => {
      if (!off) setVerified(h === block.hash && h.startsWith("0".repeat(block.difficulty)))
    })
    return () => {
      off = true
    }
  }, [block])

  const txs = block?.transactions ?? []

  return (
    <Sheet open={!!block} onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full overflow-y-auto border-l border-primary/50 bg-card sm:max-w-xl">
        {block && (
          <>
            <SheetHeader>
              <SheetTitle className="font-display tracking-widest text-primary text-glow-primary">
                BLOCK #{block.index}
              </SheetTitle>
              <SheetDescription asChild>
                <div>
                  {verified === null ? (
                    <Badge variant="outline" className="rounded-none">verifying…</Badge>
                  ) : verified ? (
                    <Badge className="rounded-none bg-accent text-accent-foreground">
                      <ShieldCheck className="size-3" /> hash + PoW verified in your browser
                    </Badge>
                  ) : (
                    <Badge className="rounded-none bg-destructive text-destructive-foreground">
                      <ShieldAlert className="size-3" /> verification failed
                    </Badge>
                  )}
                </div>
              </SheetDescription>
            </SheetHeader>

            <div className="space-y-4 px-4 pb-6">
              <Field label="HASH"><Hash value={block.hash} className="text-xs" /></Field>
              <Field label="PREVIOUS HASH"><Hash value={block.prevHash} className="text-xs" /></Field>
              <Field label="MERKLE ROOT"><span className="break-all font-mono text-xs">{block.merkleRoot}</span></Field>
              <div className="grid grid-cols-3 gap-3">
                <Field label="NONCE">{block.nonce.toLocaleString()}</Field>
                <Field label="DIFFICULTY">{block.difficulty}</Field>
                <Field label="TIME">{new Date(block.timestamp * 1000).toLocaleTimeString()}</Field>
              </div>

              <div>
                <div className="mb-2 text-[10px] tracking-[0.25em] text-muted-foreground">
                  TRANSACTIONS ({txs.length})
                </div>
                <div className="space-y-2">
                  {txs.length === 0 && <div className="text-sm text-muted-foreground">none (genesis block)</div>}
                  {txs.map((t) => (
                    <div key={t.id} className="cyber-card-sm border border-border bg-muted/40 p-2 font-mono text-xs">
                      <div className="flex justify-between">
                        <span className={t.from === "COINBASE" ? "text-[#ffe600]" : "text-muted-foreground"}>
                          {t.from === "COINBASE" ? "COINBASE" : short(t.from, 8, 6)} → {short(t.to, 8, 6)}
                        </span>
                        <span className="text-accent">{t.amount}</span>
                      </div>
                      <div className="mt-1 truncate text-[10px] text-muted-foreground/70">id {t.id}</div>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </>
        )}
      </SheetContent>
    </Sheet>
  )
}