import { memo } from 'react'
import type { CSSProperties } from 'react'
import type { Game } from '../daemon/types'
import { useT } from '../i18n/t'
import { Badge } from './Badge'
import { GameArt } from './GameArt'
import styles from './Tile.module.css'

interface TileProps {
  game: Game
  running: boolean
  progress?: number | null
  index: number
  leaving?: boolean
  onOpen: () => void
}

// Install progress repaints one tile per tick; keep the props comparable.
export const Tile = memo(function Tile({
  game,
  running,
  progress,
  index,
  leaving = false,
  onOpen,
}: TileProps) {
  const t = useT()
  const facts = [game.store.toUpperCase()]
  if (game.favorite === true) facts.unshift('★')
  facts.push(game.installed ? game.platform || t('Installed') : t('Not installed'))

  return (
    <button
      type="button"
      className={styles.tile}
      style={{ animationDelay: `${Math.min(index * 14, 260)}ms` } as CSSProperties}
      onClick={onOpen}
      aria-label={game.title}
      data-testid={leaving ? 'tile-leaving' : 'tile'}
      aria-hidden={leaving || undefined}
      tabIndex={leaving ? -1 : undefined}
      data-store={game.store}
      data-title={game.title}
    >
      <div className={styles.card}>
        <GameArt
          className={styles.art}
          title={game.title}
          artwork={game.artwork}
          artworkLocal={game.artwork_local}
          role="square"
        />
        <div className={styles.scrim} />
        <div className={`${styles.corner} ${styles.topLeft}`}>
          <Badge>{game.store}</Badge>
        </div>
        {game.update_available ? (
          <div className={`${styles.corner} ${styles.topRight}`}>
            <Badge accent>{t('UPDATE')}</Badge>
          </div>
        ) : null}
        {running ? (
          <div className={`${styles.corner} ${styles.bottomLeft}`}>
            <Badge solid>{t('PLAYING')}</Badge>
          </div>
        ) : (
          <div className={`${styles.corner} ${styles.bottomRight} ${styles.view}`}>
            {t('VIEW')} →
          </div>
        )}
        {progress != null ? (
          <div className={styles.progress}>
            <div className={styles.progressFill} style={{ transform: `scaleX(${Math.min(1, Math.max(0.03, progress / 100))})` }} />
          </div>
        ) : null}
      </div>
      <span className={styles.title}>{game.title}</span>
      <span className={styles.facts}>{facts.join('  ·  ')}</span>
    </button>
  )
})
