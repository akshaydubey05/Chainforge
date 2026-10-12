import { useCallback, useEffect, useRef, useState } from "react"
import { RotateCcw, ShieldAlert, ShieldCheck, Hammer } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Slider } from "@/components/ui/slider"
import { Hash } from "@/components/Hash"
import { sha256Hex } from "@/lib/hash"
import { cn } from "@/lib/utils"

type LabBlock = { index: number; data: string; prevHash: string; nonce: number }

const ZERO = "0".repeat(64)
const SAMPLE = [
  "Genesis: the lab begins",
  "Alice pays Bob 20 CFG",
  "Bob pays Carol 5 CFG",
  "Carol pays Dave 2 CFG",
  "Dave pays Alice 1 CFG",
]

const header = (b: LabBlock) => `${b.index}|${b.prevHash}|${b.data}|${b.nonce}`

async function mineNonce(
  base: Omit<LabBlock, "nonce">,
  difficulty: number,
  onTick: (hash: string) => void,
  stale: () => boolean
) {
  const target = "0".repeat(difficulty)
  let nonce = 0
  for (;;) {
    const hash = await sha256Hex(header({ ...base, nonce }))
    if (hash.startsWith(target)) return { nonce, hash }
    nonce++
    if (nonce % 400 === 0) {
      onTick(hash)
      await new Promise((r) => setTimeout(r, 0)) // let the UI paint
      if (stale()) throw new Error("stale")
    }
  }
}

export function TamperLab() {
  const [difficulty, setDifficulty] = useState(3)
  const [blocks, setBlocks] = useState<LabBlock[]>([])
  const [hashes, setHashes] = useState<string[]>([])
  const [busy, setBusy] = useState<number | null>(null)
  const [ticker, setTicker] = useState("")
  const [stats, setStats] = useState<{ blocks: number; tries: number; ms: number } | null>(null)
  const runId = useRef(0)

  const forge = useCallback(async (d: number) => {
    const id = ++runId.current
    const stale = () => runId.current !== id
    setStats(null)
    setBlocks([])
    setHashes([])
    const out: LabBlock[] = []
    let prev = ZERO
    try {
      for (let i = 0; i < SAMPLE.length; i++) {
        setBusy(i)
        const base = { index: i, data: SAMPLE[i], prevHash: prev }
        const { nonce, hash } = await mineNonce(base, d, setTicker, stale)
        out.push({ ...base, nonce })
        prev = hash
        setBlocks([...out])
      }
    } catch {
      return // superseded by a newer run
    }
    setBusy(null)
  }, [])

  useEffect(() => {
    forge(3)
  }, [forge])

  // live hashes of whatever is currently in the blocks
  useEffect(() => {
    let off = false
    Promise.all(blocks.map((b) => sha256Hex(header(b)))).then((hs) => {
      if (!off) setHashes(hs)
    })
    return () => {
      off = true
    }
  }, [blocks])

  const target = "0".repeat(difficulty)
  const ready = hashes.length === blocks.length
  const ok = blocks.map((b, i) => {
    if (!ready) return true
    const powOk = hashes[i].startsWith(target)
    const linkOk = i === 0 ? b.prevHash === ZERO : b.prevHash === hashes[i - 1]
    return powOk && linkOk
  })
  const firstBad = ok.indexOf(false)

  async function remineFrom(i: number) {
    const id = ++runId.current
    const stale = () => runId.current !== id
    const t0 = performance.now()
    let tries = 0
    const next = [...blocks]
    try {
      for (let j = i; j < next.length; j++) {
        setBusy(j)
        const prev = j === 0 ? ZERO : await sha256Hex(header(next[j - 1]))
        const base = { index: next[j].index, data: next[j].data, prevHash: prev }
        const { nonce } = await mineNonce(base, difficulty, setTicker, stale)
        tries += nonce + 1
        next[j] = { ...base, nonce }
        setBlocks([...next])
      }
    } catch {
      return
    }
    setBusy(null)
    setStats({ blocks: next.length - i, tries, ms: performance.now() - t0 })
  }

  return (
    <div className="space-y-4">
      {/* controls */}
      <div className="cyber-card space-y-4 border border-border bg-card/80 p-4">
        <div>
          <h2 className="font-display text-lg font-bold tracking-widest text-primary text-glow-primary">
            TAMPER LAB
          </h2>
          <p className="text-sm text-muted-foreground">
            A 5-block chain, mined in your browser right now. Edit any block's data and watch the damage spread.
            Then try to repair it.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-4">
          <div className="w-60 space-y-2">
            <div className="text-[10px] tracking-[0.25em] text-muted-foreground">
              DIFFICULTY {difficulty} · ~{(16 ** difficulty).toLocaleString()} hashes / block
            </div>
            <Slider
              min={1}
              max={4}
              step={1}
              value={[difficulty]}
              disabled={busy !== null}
              onValueChange={(v) => setDifficulty(v[0])}
              onValueCommit={(v) => forge(v[0])}
            />
          </div>
          <Button
            variant="outline"
            onClick={() => forge(difficulty)}
            className="rounded-none border-accent/60 text-accent hover:bg-accent/10"
          >
            <RotateCcw className="size-4" /> RESET CHAIN
          </Button>
        </div>
      </div>

      {/* verdict banner */}
      <div
        className={cn(
          "flex items-center gap-3 border p-3 font-display text-sm tracking-widest",
          firstBad === -1
            ? "border-accent/60 bg-accent/10 text-accent"
            : "border-destructive bg-destructive/15 text-destructive"
        )}
      >
        {firstBad === -1 ? <ShieldCheck className="size-5" /> : <ShieldAlert className="size-5" />}
        {busy !== null
          ? `MINING BLOCK #${busy}…`
          : firstBad === -1
            ? "CHAIN VALID"
            : `CHAIN BROKEN AT BLOCK #${firstBad} · ${blocks.length - firstBad} BLOCKS INVALIDATED`}
      </div>

      {busy !== null && <div className="truncate font-mono text-xs text-primary/70">{ticker}</div>}

      {stats && (
        <div className="border border-[#ffe600]/50 bg-[#ffe600]/5 p-3 text-sm">
          <span className="font-display text-[#ffe600]">REPAIRED.</span> Re-mined{" "}
          <b>{stats.blocks}</b> block{stats.blocks > 1 ? "s" : ""} in <b>{(stats.ms / 1000).toFixed(1)}s</b> (
          {stats.tries.toLocaleString()} hashes). An attacker would have to do this faster than the whole honest
          network keeps adding new blocks.
        </div>
      )}

      {/* blocks */}
      <div className="space-y-1">
        {blocks.map((b, i) => {
          const state = firstBad === -1 || i < firstBad ? "valid" : i === firstBad ? "tampered" : "orphaned"
          const hash = hashes[i] ?? "…"
          return (
            <div key={b.index}>
              <div
                className={cn(
                  "cyber-card border p-4 transition-colors",
                  state === "valid" && "border-accent/40 bg-accent/5",
                  state === "tampered" && "border-destructive bg-destructive/15",
                  state === "orphaned" && "border-destructive/60 bg-destructive/5"
                )}
              >
                <div className="mb-3 flex items-center justify-between">
                  <span className="font-display text-sm font-bold tracking-widest">BLOCK #{b.index}</span>
                  {state === "valid" && (
                    <Badge className="rounded-none bg-accent text-accent-foreground">VALID</Badge>
                  )}
                  {state === "tampered" && (
                    <Badge className="rounded-none bg-destructive text-destructive-foreground">TAMPERED</Badge>
                  )}
                  {state === "orphaned" && (
                    <Badge variant="outline" className="rounded-none border-destructive text-destructive">
                      INVALIDATED
                    </Badge>
                  )}
                </div>

                <div className="space-y-2 text-xs">
                  <div>
                    <div className="mb-1 text-[10px] tracking-[0.25em] text-muted-foreground">DATA (editable)</div>
                    <Input
                      value={b.data}
                      disabled={busy !== null}
                      onChange={(e) =>
                        setBlocks((bs) => bs.map((x, j) => (j === i ? { ...x, data: e.target.value } : x)))
                      }
                      className="rounded-none bg-background/60 font-mono"
                    />
                  </div>
                  <div>
                    <span className="text-[10px] tracking-[0.25em] text-muted-foreground">PREV </span>
                    <Hash value={b.prevHash} short className={state !== "valid" ? "text-destructive" : ""} />
                  </div>
                  <div>
                    <span className="text-[10px] tracking-[0.25em] text-muted-foreground">NONCE </span>
                    <span className="font-mono">{b.nonce.toLocaleString()}</span>
                  </div>
                  <div>
                    <span className="text-[10px] tracking-[0.25em] text-muted-foreground">HASH </span>
                    <Hash value={hash} className={state !== "valid" ? "text-destructive" : ""} />
                  </div>
                </div>

                {state === "tampered" && busy === null && (
                  <Button
                    onClick={() => remineFrom(i)}
                    className="mt-3 rounded-none bg-primary font-display tracking-widest"
                  >
                    <Hammer className="size-4" /> RE-MINE FROM #{i}
                  </Button>
                )}
              </div>
              {i < blocks.length - 1 && (
                <div className="flex justify-center">
                  <div className={cn("h-5 w-px", ok[i + 1] && state === "valid" ? "bg-accent" : "bg-destructive")} />
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}