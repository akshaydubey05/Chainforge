import { useEffect, useState } from "react"
import { ArrowRightLeft, Box, Cpu, GitFork, Radio } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Hash } from "@/components/Hash"
import { useTick, type Chain, type MiningJob } from "@/hooks/useChain"
import type { Block } from "@/lib/api"
import { ago, randomHex, short } from "@/lib/format"

const panel = "cyber-card gap-3 rounded-none border-border bg-card/80"
const title = "font-display text-xs tracking-[0.25em] text-muted-foreground"

function MiningCard({ mining, difficulty }: { mining: MiningJob | null; difficulty: number }) {
  const [elapsed, setElapsed] = useState(0)
  const [glyph, setGlyph] = useState("")

  useEffect(() => {
    if (!mining) return
    const id = setInterval(() => {
      setElapsed((Date.now() - mining.startedAt) / 1000)
      setGlyph(randomHex(32))
    }, 60)
    return () => clearInterval(id)
  }, [mining])

  return (
    <Card className={`${panel} ${mining ? "border-primary" : ""}`}>
      <CardHeader className="flex-row items-center justify-between">
        <CardTitle className={title}>MINER // PROOF OF WORK</CardTitle>
        {mining ? (
          <Badge className="pulse-neon rounded-none bg-primary text-primary-foreground">
            <Cpu className="size-3" /> MINING
          </Badge>
        ) : (
          <Badge variant="outline" className="rounded-none text-muted-foreground">
            <Radio className="size-3" /> IDLE
          </Badge>
        )}
      </CardHeader>
      <CardContent className="space-y-2">
        {mining ? (
          <>
            <div className="font-display text-4xl font-black text-primary text-glow-primary">
              {elapsed.toFixed(1)}s
            </div>
            <div className="text-xs text-muted-foreground">
              searching for a hash starting with{" "}
              <span className="text-accent">{"0".repeat(mining.difficulty)}</span>… · miner {short(mining.miner, 6, 4)}
            </div>
            <div className="truncate font-mono text-xs text-primary/70" aria-hidden>
              {glyph}
            </div>
            <div className="text-[10px] tracking-widest text-muted-foreground/60">
              HASH STREAM IS A VISUALISATION · REAL HASHING RUNS IN THE GO NODE
            </div>
          </>
        ) : (
          <div className="cursor text-sm text-muted-foreground">
            waiting for transactions · difficulty {difficulty} · idle block every 30s
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function BlockRow({ b, onSelect }: { b: Block; onSelect: (b: Block) => void }) {
  const txCount = (b.transactions ?? []).length
  return (
    <button
      onClick={() => onSelect(b)}
      className="block-in cyber-card-sm w-full space-y-1 border border-border bg-muted/40 p-3 text-left transition hover:border-primary hover:bg-primary/5"
    >
      <div className="flex items-center justify-between">
        <span className="font-display text-sm font-bold text-primary text-glow-primary">BLOCK #{b.index}</span>
        <span className="text-xs text-muted-foreground">{ago(b.timestamp)}</span>
      </div>
      <Hash value={b.hash} short className="block text-xs" />
      <div className="flex items-center gap-2 text-[11px] text-muted-foreground">
        <Badge variant="outline" className="rounded-none px-1.5 py-0 text-[10px]">
          {txCount} tx
        </Badge>
        <span>nonce {b.nonce.toLocaleString()}</span>
      </div>
    </button>
  )
}

export function Dashboard({ chain, onSelect }: { chain: Chain; onSelect: (b: Block) => void }) {
  useTick(1000)
  const { blocks, mempool, feed, mining, status } = chain

  const icon = { block: Box, tx: ArrowRightLeft, mining: Cpu, reorg: GitFork } as const
  const color = { block: "text-primary", tx: "text-accent", mining: "text-[#ffe600]", reorg: "text-destructive" } as const

  return (
    <div className="grid gap-4 lg:grid-cols-3">
      <div className="space-y-4 lg:col-span-2">
        <MiningCard mining={mining} difficulty={status?.difficulty ?? 0} />
        <Card className={panel}>
          <CardHeader>
            <CardTitle className={title}>CHAIN // LATEST BLOCKS</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {blocks.length === 0 && <div className="cursor text-sm text-muted-foreground">connecting to node</div>}
            {blocks.map((b) => (
              <BlockRow key={b.index} b={b} onSelect={onSelect} />
            ))}
          </CardContent>
        </Card>
      </div>

      <div className="space-y-4">
        <Card className={panel}>
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle className={title}>MEMPOOL</CardTitle>
            <Badge variant="outline" className="rounded-none text-[#ffe600]">
              {mempool.length}
            </Badge>
          </CardHeader>
          <CardContent className="max-h-64 space-y-1 overflow-y-auto">
            {mempool.length === 0 && <div className="text-sm text-muted-foreground">no pending transactions</div>}
            {mempool.map((t) => (
              <div key={t.id} className="flex justify-between gap-2 border-b border-border/50 py-1 font-mono text-xs">
                <span className="text-muted-foreground">
                  {short(t.from, 6, 4)} → {short(t.to, 6, 4)}
                </span>
                <span className="text-accent">{t.amount}</span>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card className={panel}>
          <CardHeader>
            <CardTitle className={title}>EVENT FEED</CardTitle>
          </CardHeader>
          <CardContent className="max-h-80 space-y-1 overflow-y-auto">
            {feed.length === 0 && <div className="text-sm text-muted-foreground">listening…</div>}
            {feed.map((f) => {
              const Icon = icon[f.kind]
              return (
                <div key={f.id} className="flex items-start gap-2 text-xs">
                  <Icon className={`mt-0.5 size-3.5 shrink-0 ${color[f.kind]}`} />
                  <span className="font-mono text-muted-foreground">{f.text}</span>
                </div>
              )
            })}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}