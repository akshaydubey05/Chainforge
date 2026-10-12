import { cn } from "@/lib/utils"

export function Hash({
  value,
  short = false,
  className,
}: {
  value: string
  short?: boolean
  className?: string
}) {
  const text = short ? value.slice(0, 16) + "…" : value
  const zeros = text.match(/^0*/)![0]
  return (
    <span className={cn("font-mono break-all", className)}>
      <span className="text-accent text-glow-accent">{zeros}</span>
      <span>{text.slice(zeros.length)}</span>
    </span>
  )
}