import { useEffect, useMemo, useRef, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { api } from "@/lib/api"
import { short } from "@/lib/format"

type NetResp = {
  self: { nodeId: string; name: string; url?: string; height: number; tip: string }
  peers: { nodeId: string; name: string; url: string; height: number; tip: string; online: boolean }[]
}
type Row = { id: string; name: string; url: string; height: number; tip: string; online: boolean; self: boolean }
type Pulse = { id: number; x1: number; y1: number; x2: number; y2: number }
type NodeState = "offline" | "syncing" | "fork" | "synced"

const W = 560
const H = 360
const COLOR: Record<NodeState, string> = {
  synced: "var(--accent)",
  syncing: "#ffe600",
  fork: "var(--destructive)",
  offline: "#5b6078",
}
const LABEL: Record<NodeState, string> = {
  synced: "IN SYNC",
  syncing: "SYNCING",
  fork: "ON A FORK",
  offline: "OFFLINE",
}

function layout(n: number) {
  if (n === 1) return [{ x: W / 2, y: H / 2 }]
  return Array.from({ length: n }, (_, i) => {
    const a = (i / n) * Math.PI * 2 - Math.PI / 2
    return { x: W / 2 + Math.cos(a) * 125, y: H / 2 + Math.sin(a) * 125 }
  })
}

const hostOf = (u: string) => {
  try {
    return new URL(u).hostname
  } catch {
    return u
  }
}

export function NetworkMap() {
  const [net, setNet] = useState<NetResp | null>(null)
  const [pulses, setPulses] = useState<Pulse[]>([])
  const prevTips = useRef<Record<string, string>>({})
  const pulseId = useRef(0)

  useEffect(() => {
    let off = false
    const tick = async () => {
      try {
        const r = await api<NetResp>("/api/network")
        if (!off) setNet(r)
      } catch {
        /* node unreachable */
      }
    }
    tick()
    const id = setInterval(tick, 1500)
    return () => {
      off = true
      clearInterval(id)
    }
  }, [])

  const rows: Row[] = useMemo(() => {
    if (!net) return []
    const self: Row = {
      id: net.self.nodeId,
      name: net.self.name || "this node",
      url: net.self.url ?? "",
      height: net.self.height,
      tip: net.self.tip,
      online: true,
      self: true,
    }
    const peers: Row[] = net.peers
      .map((p) => ({
        id: p.nodeId || p.url,
        name: p.name || hostOf(p.url),
        url: p.url,
        height: p.height,
        tip: p.tip,
        online: p.online,
        self: false,
      }))
      .sort((a, b) => a.name.localeCompare(b.name))
    return [self, ...peers]
  }, [net])

  // pulses: when a node's tip changes, animate gossip from it to the others
  useEffect(() => {
    if (rows.length === 0) return
    const pos = layout(rows.length)
    const first = Object.keys(prevTips.current).length === 0
    const fresh: Pulse[] = []
    rows.forEach((r, i) => {
      const prev = prevTips.current[r.id]
      if (!first && prev !== undefined && prev !== r.tip && r.online) {
        rows.forEach((o, j) => {
          if (j !== i && o.online) {
            fresh.push({ id: pulseId.current++, x1: pos[i].x, y1: pos[i].y, x2: pos[j].x, y2: pos[j].y })
          }
        })
      }
      prevTips.current[r.id] = r.tip
    })
    if (fresh.length) {
      setPulses((p) => [...p, ...fresh])
      const ids = new Set(fresh.map((f) => f.id))
      setTimeout(() => setPulses((p) => p.filter((x) => !ids.has(x.id))), 1400)
    }
  }, [rows])

  const pos = layout(Math.max(rows.length, 1))
  const online = rows.filter((r) => r.online)
  const maxH = Math.max(0, ...online.map((r) => r.height))
  const atMax = online.filter((r) => r.height === maxH)
  const tipCount = new Map<string, number>()
  atMax.forEach((r) => tipCount.set(r.tip, (tipCount.get(r.tip) ?? 0) + 1))
  const fork = tipCount.size > 1
  const mainTip = [...tipCount.entries()].sort((a, b) => b[1] - a[1])[0]?.[0]

  const stateOf = (r: Row): NodeState => {
    if (!r.online) return "offline"
    if (r.height < maxH) return "syncing"
    if (fork && r.tip !== mainTip) return "fork"
    return "synced"
  }

  const banner = fork
    ? { text: `FORK DETECTED · nodes disagree on block #${maxH - 1}. The longest valid chain wins.`, cls: "border-destructive bg-destructive/15 text-destructive" }
    : rows.some((r) => stateOf(r) === "syncing")
      ? { text: "SYNCING · a node is catching up with the network", cls: "border-[#ffe600]/60 bg-[#ffe600]/10 text-[#ffe600]" }
      : rows.some((r) => !r.online)
        ? { text: "NODE OFFLINE · the rest of the network keeps producing blocks", cls: "border-[#ffe600]/60 bg-[#ffe600]/10 text-[#ffe600]" }
        : { text: "ALL NODES IN SYNC", cls: "border-accent/60 bg-accent/10 text-accent" }

  return (
    <div className="space-y-4">
      <div className={`border p-3 font-display text-sm tracking-widest ${banner.cls}`}>{banner.text}</div>

      <Card className="cyber-card gap-3 rounded-none">
        <CardHeader>
          <CardTitle className="font-display text-xs tracking-[0.25em] text-muted-foreground">
            P2P NETWORK // EVERY NODE VERIFIES EVERY BLOCK
          </CardTitle>
        </CardHeader>
        <CardContent>
          {rows.length <= 1 && (
            <p className="mb-2 text-sm text-muted-foreground">
              Single-node mode. Start more nodes with <code>-peers</code> to form a network.
            </p>
          )}
          <svg viewBox={`0 0 ${W} ${H}`} className="mx-auto w-full max-w-3xl">
            {rows.map((a, i) =>
              rows.slice(i + 1).map((b, k) => {
                const j = i + 1 + k
                const live = a.online && b.online
                return (
                  <line
                    key={`${a.id}-${b.id}`}
                    x1={pos[i].x}
                    y1={pos[i].y}
                    x2={pos[j].x}
                    y2={pos[j].y}
                    strokeDasharray={live ? undefined : "4 6"}
                    style={{ stroke: "var(--primary)", strokeOpacity: live ? 0.35 : 0.12, strokeWidth: 1 }}
                  />
                )
              })
            )}

            {pulses.map((p) => (
              <circle key={p.id} r="4" style={{ fill: "var(--primary)", filter: "drop-shadow(0 0 4px var(--primary))" }}>
                <animateMotion dur="1s" fill="freeze" path={`M ${p.x1} ${p.y1} L ${p.x2} ${p.y2}`} />
              </circle>
            ))}

            {rows.map((r, i) => {
              const st = stateOf(r)
              const c = COLOR[st]
              return (
                <g key={r.id} transform={`translate(${pos[i].x} ${pos[i].y})`} opacity={st === "offline" ? 0.6 : 1}>
                  {r.self && (
                    <circle r="36" fill="none" strokeDasharray="3 4" style={{ stroke: "var(--primary)" }} />
                  )}
                  <circle
                    r="26"
                    style={{ fill: "var(--card)", stroke: c, strokeWidth: 2, filter: `drop-shadow(0 0 6px ${c})` }}
                  />
                  <text textAnchor="middle" dy="5" fontSize="15" fontWeight="600" style={{ fill: c }}>
                    {r.height}
                  </text>
                  <text textAnchor="middle" dy="50" fontSize="11" style={{ fill: "var(--foreground)" }}>
                    {r.name}
                    {r.self ? " (you)" : ""}
                  </text>
                </g>
              )
            })}
          </svg>
          <p className="text-center text-[11px] text-muted-foreground/70">
            Number inside a node = blocks it holds. Dashed ring = the node serving this page.
          </p>
        </CardContent>
      </Card>

      <Card className="cyber-card gap-3 rounded-none">
        <CardHeader>
          <CardTitle className="font-display text-xs tracking-[0.25em] text-muted-foreground">NODES</CardTitle>
        </CardHeader>
        <CardContent className="space-y-1">
          {rows.map((r) => {
            const st = stateOf(r)
            return (
              <div
                key={r.id}
                className="grid grid-cols-[1fr_auto] items-center gap-2 border-b border-border/50 py-2 text-xs sm:grid-cols-[140px_110px_80px_1fr]"
              >
                <span className="font-display tracking-wider">{r.name}</span>
                <Badge variant="outline" className="w-fit rounded-none" style={{ color: COLOR[st], borderColor: COLOR[st] }}>
                  {LABEL[st]}
                </Badge>
                <span className="font-mono text-muted-foreground">{r.height} blocks</span>
                <span className="hidden font-mono text-muted-foreground sm:block">tip {short(r.tip, 10, 4)}</span>
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}