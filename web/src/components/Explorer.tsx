import { useState } from "react"
import { Search } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Hash } from "@/components/Hash"
import { useTick } from "@/hooks/useChain"
import { api, type AddrResp, type Block, type TxResp } from "@/lib/api"
import { ago, short } from "@/lib/format"

type Result = { kind: "tx"; data: TxResp } | { kind: "address"; data: AddrResp }

const panel = "cyber-card gap-3 rounded-none border-border bg-card/80"
const title = "font-display text-xs tracking-[0.25em] text-muted-foreground"

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[90px_1fr] gap-2">
      <span className="text-[10px] tracking-[0.2em] text-muted-foreground">{label}</span>
      <span className="min-w-0 text-sm">{children}</span>
    </div>
  )
}

export function Explorer({ blocks, onSelect }: { blocks: Block[]; onSelect: (b: Block) => void }) {
  useTick(1000)
  const [q, setQ] = useState("")
  const [res, setRes] = useState<Result | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function openBlock(n: number) {
    onSelect(await api<Block>(`/api/blocks/${n}`))
  }

  async function run(raw: string) {
    const v = raw.trim().toLowerCase()
    setQ(v)
    setErr(null)
    setRes(null)
    if (!v) return
    setLoading(true)
    try {
      if (/^\d+$/.test(v)) await openBlock(Number(v))
      else if (/^[0-9a-f]{64}$/.test(v)) setRes({ kind: "tx", data: await api<TxResp>(`/api/tx/${v}`) })
      else if (/^[0-9a-f]{40}$/.test(v)) setRes({ kind: "address", data: await api<AddrResp>(`/api/address/${v}`) })
      else setErr("Enter a block number, a 64-char transaction ID, or a 40-char address.")
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const AddrLink = ({ a }: { a: string }) =>
    a === "COINBASE" ? (
      <span className="text-[#ffe600]">COINBASE (block reward)</span>
    ) : (
      <button onClick={() => run(a)} className="break-all text-left font-mono text-xs text-accent hover:underline">
        {a}
      </button>
    )

  return (
    <div className="space-y-4">
      <form
        onSubmit={(e) => {
          e.preventDefault()
          run(q)
        }}
        className="flex gap-2"
      >
        <Input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="block number · tx id · address"
          className="rounded-none border-primary/50 bg-card font-mono focus-visible:ring-primary/40"
        />
        <Button type="submit" disabled={loading} className="rounded-none bg-primary font-display tracking-widest">
          <Search className="size-4" /> SCAN
        </Button>
      </form>

      {err && <div className="border border-destructive bg-destructive/10 p-3 text-sm text-destructive">{err}</div>}

      {res?.kind === "tx" && (
        <Card className={`${panel} border-accent/50`}>
          <CardHeader className="flex-row items-center justify-between">
            <CardTitle className={title}>TRANSACTION</CardTitle>
            <Badge
              className={`rounded-none ${res.data.status === "confirmed" ? "bg-accent text-accent-foreground" : "bg-[#ffe600] text-black"}`}
            >
              {res.data.status}
              {res.data.confirmations ? ` · ${res.data.confirmations} conf` : ""}
            </Badge>
          </CardHeader>
          <CardContent className="space-y-2">
            <Row label="ID"><Hash value={res.data.tx.id} className="text-xs" /></Row>
            <Row label="FROM"><AddrLink a={res.data.tx.from} /></Row>
            <Row label="TO"><AddrLink a={res.data.tx.to} /></Row>
            <Row label="AMOUNT"><span className="text-accent">{res.data.tx.amount}</span></Row>
            {res.data.blockIndex >= 0 && (
              <Row label="BLOCK">
                <button onClick={() => openBlock(res.data.blockIndex)} className="text-primary hover:underline">
                  #{res.data.blockIndex}
                </button>
              </Row>
            )}
          </CardContent>
        </Card>
      )}

      {res?.kind === "address" && (
        <Card className={`${panel} border-accent/50`}>
          <CardHeader>
            <CardTitle className={title}>ADDRESS</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="break-all font-mono text-xs">{res.data.address}</div>
            <div className="font-display text-3xl font-black text-accent text-glow-accent">
              {res.data.balance.toLocaleString()} <span className="text-sm text-muted-foreground">CFG</span>
            </div>
            <div className="max-h-72 space-y-1 overflow-y-auto">
              {(res.data.txs ?? []).map(({ tx, blockIndex }) => (
                <div key={tx.id} className="flex justify-between gap-2 border-b border-border/50 py-1 font-mono text-xs">
                  <button onClick={() => run(tx.id)} className="text-left text-muted-foreground hover:text-primary">
                    {tx.from === "COINBASE" ? "COINBASE" : short(tx.from, 6, 4)} → {short(tx.to, 6, 4)}
                  </button>
                  <span>
                    <span className="text-accent">{tx.amount}</span>{" "}
                    <span className="text-muted-foreground">{blockIndex < 0 ? "pending" : `#${blockIndex}`}</span>
                  </span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      <Card className={panel}>
        <CardHeader>
          <CardTitle className={title}>RECENT BLOCKS</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          {blocks.map((b) => (
            <button
              key={b.index}
              onClick={() => onSelect(b)}
              className="grid w-full grid-cols-[70px_1fr_60px_80px] items-center gap-2 border-b border-border/50 py-2 text-left text-xs transition hover:bg-primary/5"
            >
              <span className="font-display font-bold text-primary">#{b.index}</span>
              <Hash value={b.hash} short />
              <span className="text-muted-foreground">{(b.transactions ?? []).length} tx</span>
              <span className="text-right text-muted-foreground">{ago(b.timestamp)}</span>
            </button>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}