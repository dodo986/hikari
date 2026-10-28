/** Nexus counts: 1234 becomes 1.2K, 1234567 becomes 1.2M. */
export function compact(value: number): string {
  if (value >= 10_000_000) return `${Math.round(value / 1_000_000)}M`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 10_000) return `${Math.round(value / 1000)}K`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return String(value)
}

/** Nexus dates arrive as RFC 3339 or as a bare date; unparsable input reads empty. */
export function day(value: string | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}
