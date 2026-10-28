export function middle(text: string, max: number): string {
  if (!text || text.length <= max) return text
  const keep = Math.max(2, Math.floor((max - 1) / 2))
  return `${text.slice(0, keep)}…${text.slice(text.length - keep)}`
}

