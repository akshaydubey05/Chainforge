import { sha256Hex } from "@/lib/hash"

const hex = (b: Uint8Array) => Array.from(b, (x) => x.toString(16).padStart(2, "0")).join("")

function b64uToBytes(s: string): Uint8Array {
  const b = atob(s.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(s.length / 4) * 4, "="))
  return Uint8Array.from(b, (c) => c.charCodeAt(0))
}

function derInt(b: Uint8Array): number[] {
  let i = 0
  while (i < b.length - 1 && b[i] === 0) i++
  let v = Array.from(b.slice(i))
  if (v[0] & 0x80) v = [0, ...v]
  return [0x02, v.length, ...v]
}

/** WebCrypto gives raw r||s (64 bytes); Go's VerifyASN1 wants DER. */
function rawSigToDer(sig: Uint8Array): Uint8Array {
  const body = [...derInt(sig.slice(0, 32)), ...derInt(sig.slice(32, 64))]
  return Uint8Array.from([0x30, body.length, ...body])
}

export async function sendTransfer(
  w: { address: string; jwk: JsonWebKey },
  to: string,
  amount: number
): Promise<string> {
  const key = await crypto.subtle.importKey("jwk", w.jwk, { name: "ECDSA", namedCurve: "P-256" }, false, ["sign"])

  // public key as 64 bytes X||Y, same as the Go node expects
  const x = b64uToBytes(w.jwk.x!)
  const y = b64uToBytes(w.jwk.y!)
  const pub = new Uint8Array(64)
  pub.set(x, 32 - x.length)
  pub.set(y, 64 - y.length)

  // nanosecond timestamp: needs BigInt (too large for a JS number)
  const ts = BigInt(Date.now()) * BigInt(1000000) + BigInt(Math.floor(Math.random() * 1000000))

  // identical to core.Transaction.SigningHash input: "from|to|amount|timestamp"
  const msg = `${w.address}|${to}|${amount}|${ts.toString()}`
  const id = await sha256Hex(msg)
  const raw = new Uint8Array(
    await crypto.subtle.sign({ name: "ECDSA", hash: "SHA-256" }, key, new TextEncoder().encode(msg))
  )
  const signature = hex(rawSigToDer(raw))

  // hand-built JSON so the BigInt timestamp stays an exact integer
  const body = `{"id":"${id}","from":"${w.address}","to":"${to}","amount":${amount},"timestamp":${ts.toString()},"pubKey":"${hex(pub)}","signature":"${signature}"}`

  const res = await fetch("/api/tx", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data?.error ?? `request failed (${res.status})`)
  return id
}