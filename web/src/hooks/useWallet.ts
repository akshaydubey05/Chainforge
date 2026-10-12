import { useCallback, useState } from "react"
import { sha256HexBytes } from "@/lib/hash"

const KEY = "chainforge.wallet.v1"
type StoredWallet = { address: string; jwk: JsonWebKey }

/**
 * Browser wallet: ECDSA P-256, the same curve the Go node verifies.
 * Address = first 40 hex chars of sha256(X||Y), identical to core.AddressFromKey.
 * The private key (JWK) stays in this browser's localStorage. Testnet only.
 */
export function useWallet() {
  const [wallet, setWallet] = useState<StoredWallet | null>(() => {
    try {
      const raw = localStorage.getItem(KEY)
      return raw ? (JSON.parse(raw) as StoredWallet) : null
    } catch {
      return null
    }
  })

  const create = useCallback(async () => {
    const pair = await crypto.subtle.generateKey(
      { name: "ECDSA", namedCurve: "P-256" },
      true,
      ["sign", "verify"]
    )
    // raw export = 0x04 || X(32) || Y(32); drop the 0x04 prefix to get the node's 64-byte format
    const raw = new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey))
    const address = (await sha256HexBytes(raw.slice(1))).slice(0, 40)
    const jwk = await crypto.subtle.exportKey("jwk", pair.privateKey)
    const w = { address, jwk }
    localStorage.setItem(KEY, JSON.stringify(w))
    setWallet(w)
  }, [])

  const destroy = useCallback(() => {
    localStorage.removeItem(KEY)
    setWallet(null)
  }, [])

  return { wallet, create, destroy }
}