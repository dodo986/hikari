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
  onOpen: () => void
}

export function Tile({ game, running, progress, index, onOpen }: TileProps) {
  const t = useT()
  const facts = [game.store.toUpperCase()]
  if (game.favorite === true) facts.unshift('★')
  facts.push(game.installed ? game.platform || t('Installed') : t('Not installed'))
  const footer = game.installed ? game.install_size || game.version || '' : ''

  return (
    <button
      type="button"
      className={styles.tile}
      style={{ animationDelay: `${Math.min(index * 14, 260)}ms` } as CSSProperties}
      onClick={onOpen}
      aria-label={game.title}
      data-testid="tile"
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
            <div className={styles.progressFill} style={{ width: `${Math.min(100, Math.max(3, progress))}%` }} />
          </div>
        ) : null}
      </div>
      <span className={styles.title}>{game.title}</span>
      <span className={styles.facts}>{facts.join('  ·  ')}</span>
      {footer ? <span className={styles.facts}>{footer}</span> : null}
    </button>
  )
}
