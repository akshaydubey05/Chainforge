import { useState } from "react"
import { Boxes, FlaskConical, ScanSearch, Network, Wallet } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { NetworkMap } from "@/components/NetworkMap"
import { BlockSheet } from "@/components/BlockSheet"
import { Dashboard } from "@/components/Dashboard"
import { Explorer } from "@/components/Explorer"
import { StatsBar } from "@/components/StatsBar"
import { TamperLab } from "@/components/TamperLab"
import { WalletPanel } from "@/components/WalletPanel"
import { useChain } from "@/hooks/useChain"
import type { Block } from "@/lib/api"
import { cn } from "@/lib/utils"

const TABS = [
  { id: "live", label: "LIVE", Icon: Boxes },
  { id: "explorer", label: "EXPLORER", Icon: ScanSearch },
  { id: "tamper", label: "TAMPER LAB", Icon: FlaskConical },
  { id: "wallet", label: "WALLET", Icon: Wallet },
  { id: "network", label: "NETWORK", Icon: Network },
] as const

function initialTab(): string {
  const h = location.hash.slice(1)
  return TABS.some((t) => t.id === h) ? h : "live"
}

export default function App() {
  const chain = useChain()
  const [tab, setTab] = useState(initialTab)
  const [selected, setSelected] = useState<Block | null>(null)

  const onTab = (id: string) => {
    setTab(id)
    history.replaceState(null, "", `#${id}`) // shareable deep link, e.g. /#tamper
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-6xl flex-col gap-6 px-4 py-8">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1
            className="glitch font-display text-3xl font-black tracking-[0.2em] text-primary text-glow-primary sm:text-5xl"
            data-text="CHAINFORGE"
          >
            CHAINFORGE
          </h1>
          <p className="mt-2 text-xs tracking-[0.35em] text-muted-foreground">
            LIVE BLOCKCHAIN LAB // WRITTEN IN GO
          </p>
        </div>
        {chain.connected ? (
          <Badge className="rounded-none bg-accent/15 px-3 py-1 text-accent">
            <span className="pulse-neon">●</span> NODE ONLINE
          </Badge>
        ) : (
          <Badge className="rounded-none bg-destructive/20 px-3 py-1 text-destructive">
            ○ OFFLINE · RETRYING
          </Badge>
        )}
      </header>

      <StatsBar status={chain.status} blocks={chain.blocks} mempool={chain.mempool.length} />

      <nav role="tablist" className="flex flex-wrap gap-2 border-b border-border pb-4">
        {TABS.map(({ id, label, Icon }) => {
          const active = tab === id
          return (
            <button
              key={id}
              role="tab"
              aria-selected={active}
              onClick={() => onTab(id)}
              className={cn(
                "flex items-center gap-2 border px-5 py-2.5 font-display text-xs tracking-widest transition",
                active
                  ? "border-primary bg-primary/15 text-primary shadow-[0_0_18px_color-mix(in_srgb,var(--primary)_45%,transparent)]"
                  : "border-border text-muted-foreground hover:border-primary/60 hover:text-foreground"
              )}
            >
              <Icon className="size-4" />
              {label}
            </button>
          )
        })}
      </nav>

      <main className="flex-1">
        {tab === "live" && <Dashboard chain={chain} onSelect={setSelected} />}
        {tab === "explorer" && <Explorer blocks={chain.blocks} onSelect={setSelected} />}
        {/* always mounted, only hidden, so the lab keeps its state between tabs */}
        <div className={tab === "tamper" ? "" : "hidden"}>
          <TamperLab />
        </div>
        {tab === "wallet" && <WalletPanel latestIndex={chain.status?.latestIndex} />}
        {tab === "network" && <NetworkMap />}
      </main>

      <BlockSheet block={selected} onClose={() => setSelected(null)} />

      <footer className="border-t border-border pt-4 text-center text-[11px] tracking-widest text-muted-foreground/70">
        DEMO NETWORK · COINS HAVE NO VALUE · PROOF OF WORK · ECDSA · MERKLE TREES
      </footer>
    </div>
  )
}