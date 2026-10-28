import { useState } from 'react'
import type { CSSProperties } from 'react'
import { artUrl } from '../daemon/endpoint'
import type { Artwork } from '../daemon/types'
import styles from './GameArt.module.css'

interface GameArtProps {
  title: string
  artwork?: Artwork
  artworkLocal?: Artwork
  className?: string
  style?: CSSProperties
  showTitle?: boolean
  /** Cover art is portrait; a square slot crops it less from the top. */
  role?: 'cover' | 'square'
  /** Painted width in CSS pixels; the bridge serves a cached copy at this size. */
  width?: number
}

function pick(artwork: Artwork | undefined, local: Artwork | undefined, role: 'cover' | 'square') {
  const other = role === 'cover' ? 'square' : 'cover'
  return (
    local?.[role] ||
    artwork?.[role] ||
    local?.[other] ||
    artwork?.[other] ||
    ''
  )
}

export function GameArt({
  title,
  artwork,
  artworkLocal,
  className,
  style,
  showTitle = true,
  role = 'square',
  width = 260,
}: GameArtProps) {
  const [loaded, setLoaded] = useState(false)
  const [wide, setWide] = useState(false)
  // Ask for roughly what the slot paints, doubled for a high density screen.
  const requested =
    typeof window === 'undefined' ? 0 : Math.round(width * Math.min(2, window.devicePixelRatio || 1))
  const source = artUrl(pick(artwork, artworkLocal, role), requested)
  const logo = artUrl(artworkLocal?.logo ?? artwork?.logo)

  return (
    <div className={[styles.frame, className].filter(Boolean).join(' ')} style={style}>
      <div className={styles.placeholder} data-testid="art-placeholder" style={{ opacity: loaded ? 0 : 1 }}>
        <span className={styles.initial} style={{ fontSize: 'min(64px, 3rem)' }}>
          {(title || '?').charAt(0).toUpperCase()}
        </span>
        {showTitle && title ? <span className={styles.placeholderTitle}>{title}</span> : null}
      </div>
      {source ? (
        <img
          className={[styles.image, wide ? styles.wide : '', loaded ? styles.shown : ''].join(' ')}
          src={source}
          alt=""
          loading="lazy"
          decoding="async"
          onLoad={(event) => {
            const image = event.currentTarget
            setWide(image.naturalWidth / Math.max(1, image.naturalHeight) > 2.6)
            setLoaded(true)
          }}
          onError={() => setLoaded(false)}
        />
      ) : null}
      {loaded && logo ? <img className={styles.logo} src={logo} alt="" /> : null}
    </div>
  )
}
