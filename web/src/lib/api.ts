export type Tx = {
  id: string
  from: string
  to: string
  amount: number
  timestamp: number // nanoseconds: exceeds JS safe-integer range, never use it for math
  pubKey?: string
  signature?: string
}

export type Block = {
  index: number
  timestamp: number // unix seconds
  prevHash: string
  merkleRoot: string
  difficulty: number
  nonce: number
  hash: string
  transactions: Tx[] | null // genesis block has null
}

export type Status = {
  latestIndex: number
  latestHash: string
  difficulty: number
  mempool: number
  blockReward: number
  totalSupply: number
  nodeAddress: string
  faucetAmount: number
}

export type TxResp = {
  tx: Tx
  status: "pending" | "confirmed"
  blockIndex: number
  confirmations?: number
}

export type AddrResp = {
  address: string
  balance: number
  txs: { tx: Tx; blockIndex: number }[] | null
}

export async function api<T>(
  path: string,
  init?: { method?: string; body?: unknown }
): Promise<T> {
  const res = await fetch(path, {
    method: init?.method ?? "GET",
    headers: init?.body ? { "Content-Type": "application/json" } : undefined,
    body: init?.body ? JSON.stringify(init.body) : undefined,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data?.error ?? `request failed (${res.status})`)
  return data as T
}