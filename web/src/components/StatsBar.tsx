import { Card } from "@/components/ui/card"
import type { Block, Status } from "@/lib/api"

function avgBlockTime(blocks: Block[]): number | null {
  const b = blocks.slice(0, 11) // newest first
  if (b.length < 2) return null
  return (b[0].timestamp - b[b.length - 1].timestamp) / (b.length - 1)
}

export function StatsBar({
  status,
  blocks,
  mempool,
}: {
  status: Status | null
  blocks: Block[]
  mempool: number
}) {
  const avg = avgBlockTime(blocks)
  const items = [
    { label: "HEIGHT", value: status ? `#${status.latestIndex}` : "—", color: "text-primary text-glow-primary", bar: "bg-primary" },
    { label: "DIFFICULTY", value: status ? String(status.difficulty) : "—", color: "text-accent text-glow-accent", bar: "bg-accent" },
    { label: "MEMPOOL", value: status ? String(mempool) : "—", color: "text-[#ffe600]", bar: "bg-[#ffe600]" },
    { label: "SUPPLY", value: status ? status.totalSupply.toLocaleString() : "—", color: "text-[#00e5ff]", bar: "bg-[#00e5ff]" },
    { label: "AVG BLOCK TIME", value: avg === null ? "—" : `${avg.toFixed(1)}s`, color: "text-foreground", bar: "bg-foreground/60" },
  ]
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
      {items.map((i) => (
        <Card key={i.label} className="cyber-card-sm gap-2 rounded-none px-4 py-3">
          <div className={`h-0.5 w-8 ${i.bar}`} />
          <div className="text-[10px] tracking-[0.25em] text-muted-foreground">{i.label}</div>
          <div className={`font-sans text-3xl font-semibold tabular-nums ${i.color}`}>{i.value}</div>
        </Card>
      ))}
    </div>
  )
}