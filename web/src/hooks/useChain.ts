import { useCallback, useEffect, useRef, useState } from "react"
import { api, type Block, type Status, type Tx } from "@/lib/api"
import { short } from "@/lib/format"

export type FeedItem = { id: number; kind: "block" | "tx" | "mining" | "reorg"; text: string; ts: number }
export type MiningJob = { miner: string; difficulty: number; startedAt: number }

type WsEvent =
  | { type: "hello"; data: Status }
  | { type: "mining_started"; data: { miner: string; difficulty: number; mempool: number } }
  | { type: "new_block"; data: Block }
  | { type: "new_tx"; data: Tx }
  | { type: "reorg"; data: { peer: string; dropped: number; newHeight: number } }

/** Re-render every `ms` so "x seconds ago" labels stay fresh. */
export function useTick(ms = 1000) {
  const [, set] = useState(0)
  useEffect(() => {
    const id = setInterval(() => set((n) => n + 1), ms)
    return () => clearInterval(id)
  }, [ms])
}

export function useChain() {
  const [status, setStatus] = useState<Status | null>(null)
  const [blocks, setBlocks] = useState<Block[]>([])
  const [mempool, setMempool] = useState<Tx[]>([])
  const [feed, setFeed] = useState<FeedItem[]>([])
  const [connected, setConnected] = useState(false)
  const [mining, setMining] = useState<MiningJob | null>(null)
  const feedId = useRef(0)

  const pushFeed = useCallback((kind: FeedItem["kind"], text: string) => {
    setFeed((f) => [{ id: feedId.current++, kind, text, ts: Date.now() }, ...f].slice(0, 40))
  }, [])

  useEffect(() => {
    let ws: WebSocket | null = null
    let timer: number | undefined
    let closed = false

    const load = async () => {
      try {
        const [s, b, m] = await Promise.all([
          api<Status>("/api/status"),
          api<{ blocks: Block[] }>("/api/blocks?limit=30"),
          api<Tx[]>("/api/mempool"),
        ])
        setStatus(s)
        setBlocks(b.blocks)
        setMempool(m ?? [])
      } catch {
        /* node not reachable yet; the ws reconnect loop will retry */
      }
    }

    const connect = () => {
      const proto = location.protocol === "https:" ? "wss" : "ws"
      ws = new WebSocket(`${proto}://${location.host}/ws`)
      ws.onopen = () => {
        setConnected(true)
        load()
      }
      ws.onmessage = (m) => {
        const e = JSON.parse(m.data) as WsEvent
        switch (e.type) {
          case "hello":
            setStatus(e.data)
            break
          case "reorg":
            pushFeed("reorg", `chain reorganised · ${e.data.dropped} block(s) replaced by a longer chain`)
            load()
            break
          case "mining_started":
            setMining({ ...e.data, startedAt: Date.now() })
            pushFeed("mining", `mining started · difficulty ${e.data.difficulty} · ${e.data.mempool} pending`)
            break
          case "new_block": {
            const b = e.data
            const txs = b.transactions ?? []
            setBlocks((p) => [b, ...p.filter((x) => x.index !== b.index)].slice(0, 30))
            setMining(null)
            const ids = new Set(txs.map((t) => t.id))
            setMempool((p) => p.filter((t) => !ids.has(t.id)))
            setStatus((s) =>
              s
                ? { ...s, latestIndex: b.index, latestHash: b.hash, totalSupply: b.index * s.blockReward }
                : s
            )
            pushFeed("block", `block #${b.index} sealed · ${txs.length} tx · nonce ${b.nonce.toLocaleString()}`)
            break
          }
          case "new_tx":
            setMempool((p) => (p.some((t) => t.id === e.data.id) ? p : [...p, e.data]))
            pushFeed("tx", `${short(e.data.from, 6, 4)} → ${short(e.data.to, 6, 4)} · ${e.data.amount}`)
            break
        }
      }
      ws.onclose = () => {
        if (closed) return
        setConnected(false)
        timer = window.setTimeout(connect, 2000)
      }
    }

    connect()
    load()
    return () => {
      closed = true
      clearTimeout(timer)
      ws?.close()
    }
  }, [pushFeed])

  return { status, blocks, mempool, feed, connected, mining }
}

export type Chain = ReturnType<typeof useChain>