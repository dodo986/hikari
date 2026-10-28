export function compact(value: number): string {
  if (value >= 10_000_000) return `${Math.round(value / 1_000_000)}M`
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`
  if (value >= 10_000) return `${Math.round(value / 1000)}K`
  if (value >= 1000) return `${(value / 1000).toFixed(1)}K`
  return String(value)
}

export function formatBytes(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) return ''
  const gib = bytes / 1024 ** 3
  if (gib >= 1) return `${gib.toFixed(1)} GiB`
  return `${Math.round(bytes / 1024 ** 2)} MiB`
}

export function formatPercent(value: number | null | undefined): string {
  return typeof value === 'number' ? `${value.toFixed(1)}%` : ''
}

export function formatSpeed(value: number | null | undefined): string {
  return typeof value === 'number' ? `${value.toFixed(1)} MiB/s` : ''
}

export function day(value: string | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}
