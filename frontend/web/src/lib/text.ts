/** Middle elision, the way paths are shown in a narrow slot. */
export function middle(text: string, max: number): string {
  if (!text || text.length <= max) return text
  const keep = Math.max(2, Math.floor((max - 1) / 2))
  return `${text.slice(0, keep)}…${text.slice(text.length - keep)}`
}

export function bytes(value: number | undefined): string {
  if (!value || value <= 0) return ''
  if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(1)} GiB`
  return `${Math.round(value / 1024 ** 2)} MiB`
}
