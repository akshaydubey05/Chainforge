import { useCallback, useEffect, useState } from "react"
import { Check, Copy, Droplets, Hammer, Send, Trash2, Wallet } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { useWallet } from "@/hooks/useWallet"
import { api, type AddrResp } from "@/lib/api"
import { short } from "@/lib/format"
import { sendTransfer } from "@/lib/sign"

const panel = "cyber-card gap-3 rounded-none border-border bg-card/80"
const title = "font-display text-xs tracking-[0.25em] text-muted-foreground"

export function WalletPanel({ latestIndex }: { latestIndex: number | undefined }) {
  const { wallet, create, destroy } = useWallet()
  const [info, setInfo] = useState<AddrResp | null>(null)
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [busy, setBusy] = useState<"faucet" | "mine" | "send" | null>(null)
  const [copied, setCopied] = useState(false)
  const [to, setTo] = useState("")
  const [amount, setAmount] = useState("5")

  const refresh = useCallback(async () => {
    if (!wallet) return
    try {
      setInfo(await api<AddrResp>(`/api/address/${wallet.address}`))
    } catch {
      /* ignore */
    }
  }, [wallet])

  // refresh on mount and whenever a new block lands
  useEffect(() => {
    refresh()
  }, [refresh, latestIndex])

  async function run(kind: "faucet" | "mine" | "send", fn: () => Promise<string>) {
    setBusy(kind)
    setMsg(null)
    try {
      setMsg({ ok: true, text: await fn() })
    } catch (e) {
      setMsg({ ok: false, text: (e as Error).message })
    } finally {
      setBusy(null)
      refresh()
    }
  }

  if (!wallet) {
    return (
      <Card className={panel}>
        <CardHeader>
          <CardTitle className={title}>WALLET</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Generate an ECDSA P-256 key pair right in your browser. Your private key never leaves this device.
          </p>
          <Button onClick={create} className="rounded-none bg-primary font-display tracking-widest">
            <Wallet className="size-4" /> GENERATE WALLET
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      <Card className={`${panel} border-primary/60`}>
        <CardHeader>
          <CardTitle className={title}>YOUR ADDRESS</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center gap-2">
            <code className="break-all text-xs text-accent">{wallet.address}</code>
            <Button
              size="icon"
              variant="ghost"
              onClick={() => {
                navigator.clipboard.writeText(wallet.address)
                setCopied(true)
                setTimeout(() => setCopied(false), 1500)
              }}
            >
              {copied ? <Check className="size-4 text-accent" /> : <Copy className="size-4" />}
            </Button>
          </div>

          <div className="font-sans text-4xl font-semibold tabular-nums text-primary text-glow-primary">
            {(info?.balance ?? 0).toLocaleString()} <span className="text-base text-muted-foreground">CFG</span>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              disabled={busy !== null}
              onClick={() =>
                run("faucet", async () => {
                  await api("/api/faucet", { method: "POST", body: { address: wallet.address } })
                  return "Faucet transaction queued. Coins arrive with the next block."
                })
              }
              className="rounded-none bg-accent font-display tracking-widest text-accent-foreground hover:bg-accent/80"
            >
              <Droplets className="size-4" /> {busy === "faucet" ? "SENDING…" : "FAUCET"}
            </Button>
            <Button
              disabled={busy !== null}
              onClick={() =>
                run("mine", async () => {
                  const b = await api<{ index: number; nonce: number }>("/api/mine", {
                    method: "POST",
                    body: { miner: wallet.address },
                  })
                  return `You mined block #${b.index} (nonce ${b.nonce.toLocaleString()}) and earned the block reward.`
                })
              }
              className="rounded-none bg-primary font-display tracking-widest"
            >
              <Hammer className="size-4" /> {busy === "mine" ? "MINING…" : "MINE A BLOCK"}
            </Button>
            <Button
              variant="outline"
              onClick={() => confirm("Delete this wallet from your browser? The key cannot be recovered.") && destroy()}
              className="rounded-none border-destructive/60 text-destructive hover:bg-destructive/10"
            >
              <Trash2 className="size-4" /> BURN WALLET
            </Button>
          </div>

          {msg && (
            <div
              className={`border p-3 text-sm ${msg.ok ? "border-accent/60 text-accent" : "border-destructive text-destructive"}`}
            >
              {msg.text}
            </div>
          )}
          <p className="text-[11px] text-muted-foreground/70">
            TESTNET ONLY. The key is stored unencrypted in this browser's localStorage. Coins have no value.
          </p>
        </CardContent>
      </Card>

      <Card className={panel}>
        <CardHeader>
          <CardTitle className={title}>SEND COINS // SIGNED IN YOUR BROWSER</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Input
            value={to}
            onChange={(e) => setTo(e.target.value.trim().toLowerCase())}
            placeholder="recipient address (40 hex characters)"
            className="rounded-none bg-background/60 font-mono text-xs"
          />
          <div className="flex gap-2">
            <Input
              type="number"
              min={1}
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              className="w-32 rounded-none bg-background/60 font-mono"
            />
            <Button
              disabled={busy !== null}
              onClick={() =>
                run("send", async () => {
                  const n = Number(amount)
                  if (!/^[0-9a-f]{40}$/.test(to)) throw new Error("Recipient must be a 40-character hex address.")
                  if (to === wallet.address) throw new Error("You can't send coins to yourself.")
                  if (!Number.isInteger(n) || n < 1) throw new Error("Amount must be a whole number of at least 1.")
                  const id = await sendTransfer(wallet, to, n)
                  return `Signed locally and broadcast. Transaction ${id.slice(0, 12)}… is pending until the next block.`
                })
              }
              className="rounded-none bg-primary font-display tracking-widest"
            >
              <Send className="size-4" /> {busy === "send" ? "SIGNING…" : "SIGN & SEND"}
            </Button>
          </div>
          <p className="text-[11px] text-muted-foreground/70">
            The node never sees your private key. It only verifies the ECDSA signature.
          </p>
        </CardContent>
      </Card>

      <Card className={panel}>
        <CardHeader>
          <CardTitle className={title}>HISTORY</CardTitle>
        </CardHeader>
        <CardContent className="max-h-80 space-y-1 overflow-y-auto">
          {(info?.txs ?? []).length === 0 && <div className="text-sm text-muted-foreground">no activity yet</div>}
          {(info?.txs ?? []).map(({ tx, blockIndex }) => (
            <div key={tx.id} className="flex justify-between gap-2 border-b border-border/50 py-1 font-mono text-xs">
              <span className="text-muted-foreground">
                {tx.from === "COINBASE" ? "REWARD" : short(tx.from, 6, 4)} → {short(tx.to, 6, 4)}
              </span>
              <span>
                <span className="text-accent">{tx.amount}</span>{" "}
                <span className="text-muted-foreground">{blockIndex < 0 ? "pending" : `#${blockIndex}`}</span>
              </span>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}