import type { PaletteRoles } from '../daemon/types'

const LUMINANCE = (hex: string): number => {
  const value = hex.replace('#', '')
  const full =
    value.length === 3
      ? value
          .split('')
          .map((part) => part + part)
          .join('')
      : value
  const r = parseInt(full.slice(0, 2), 16) / 255
  const g = parseInt(full.slice(2, 4), 16) / 255
  const b = parseInt(full.slice(4, 6), 16) / 255
  return 0.299 * r + 0.587 * g + 0.114 * b
}

// A named palette wins, then the wallpaper's roles, then the compiled defaults.
export function applyPalette(palette: PaletteRoles | null | undefined): void {
  if (!palette) return
  const root = document.documentElement.style
  const paper = palette.surface
  const ink = palette.onSurface
  const inkDim = palette.onSurfaceVariant ?? ink
  const accent = palette.primary
  const lift = palette.surfaceContainerLow
  if (paper) root.setProperty('--paper', paper)
  if (lift) root.setProperty('--paper-lift', lift)
  if (ink) root.setProperty('--ink', ink)
  if (inkDim) root.setProperty('--ink-dim', inkDim)
  if (accent) {
    root.setProperty('--sun', accent)
    root.setProperty('--sun-deep', `color-mix(in srgb, ${accent} 70%, black)`)
  }
  if (palette.inverseSurface) root.setProperty('--bone', palette.inverseSurface)
  if (palette.inverseOnSurface) root.setProperty('--ink-on-bone', palette.inverseOnSurface)
  // CSS only needs the boolean; the palette decides every surface colour itself.
  document.documentElement.dataset.light = paper && LUMINANCE(paper) > 0.5 ? 'true' : 'false'
}
