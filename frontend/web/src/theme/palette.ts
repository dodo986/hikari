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

/**
 * Paints the resolved Material roles onto the document, the way Tokens.qml
 * resolves them: a named palette wins, otherwise the wallpaper's roles apply,
 * otherwise the compiled defaults stay.
 */
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
  // Greyscale wallpapers hand back an error role indistinguishable from body text,
  // so failures keep the house red there instead of disappearing into the page.
  const error = palette.error
  if (error && ink && Math.abs(LUMINANCE(error) - LUMINANCE(ink)) > 0.08) {
    root.setProperty('--alert', error)
  }
  root.setProperty('--light', paper && LUMINANCE(paper) > 0.5 ? '1' : '0')
  document.documentElement.dataset.light = paper && LUMINANCE(paper) > 0.5 ? 'true' : 'false'
}
