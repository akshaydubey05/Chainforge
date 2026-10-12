export const short = (s: string, a = 8, b = 6) =>
  s.length <= a + b + 1 ? s : `${s.slice(0, a)}…${s.slice(-b)}`

export function ago(unixSeconds: number): string {
  const s = Math.max(0, Math.floor(Date.now() / 1000 - unixSeconds))
  if (s < 5) return "just now"
  if (s < 60) return `${s}s ago`
  if (s < 3600) return `${Math.floor(s / 60)}m ago`
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`
  return `${Math.floor(s / 86400)}d ago`
}

export function randomHex(bytes: number): string {
  const a = crypto.getRandomValues(new Uint8Array(bytes))
  return Array.from(a, (b) => b.toString(16).padStart(2, "0")).join("")
}