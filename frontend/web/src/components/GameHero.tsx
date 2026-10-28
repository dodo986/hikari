import { Badge } from './Badge'
import { GameArt } from './GameArt'
import type { Game } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './GameHero.module.css'

interface GameHeroProps {
  game: Game | null
  running: boolean
  launching: boolean
  crashed: boolean
  size: string
}

const STORE_NAMES: Record<string, string> = {
  epic: 'EPIC',
  gog: 'GOG',
  amazon: 'AMAZON',
  steam: 'STEAM',
  local: 'LOCAL',
}

export function GameHero({ game, running, launching, crashed, size }: GameHeroProps) {
  const t = useT()
  const installed = game?.installed ?? false
  const badges: string[] = []
  if (game?.store) badges.push(STORE_NAMES[game.store] ?? game.store.toUpperCase())
  if (launching) badges.push(t('LAUNCHING'))
  else if (running) badges.push(t('PLAYING'))
  else if (crashed) badges.push(t('CRASHED'))
  if (!launching && !running && !crashed) badges.push(installed ? t('INSTALLED') : t('NOT INSTALLED'))
  if (game?.update_available) badges.push(t('UPDATE'))

  const facts: string[] = []
  if (game?.platform) facts.push(game.platform)
  if (size) facts.push(size)
  if (game?.version) facts.push(game.version)
  if (game?.can_run_offline) facts.push(t('OFFLINE READY'))
  if (game?.cloud_saves) facts.push(t('CLOUD SAVES'))

  return (
    <section className={styles.hero}>
      <div className={styles.art}>
        <GameArt
          title={game?.title ?? ''}
          artwork={game?.artwork}
          artworkLocal={game?.artwork_local}
          role="cover"
          showTitle={false}
          width={1200}
          className={styles.cover}
        />
      </div>
      <div className={styles.scrim} />
      <div className={styles.content}>
        <div className={styles.badges}>
          {badges.map((label, index) => (
            <Badge
              key={label}
              accent={index === 0}
              solid={label === t('PLAYING')}
            >
              {label}
            </Badge>
          ))}
        </div>
        <h1 className={styles.title}>{game?.title ?? ''}</h1>
        {game?.developer ? <p className={styles.developer}>{game.developer}</p> : null}
        <div className={styles.facts}>
          {facts.map((fact) => (
            <Badge key={fact}>{fact}</Badge>
          ))}
        </div>
      </div>
      <div className={styles.square}>
        <GameArt
          title={game?.title ?? ''}
          artwork={game?.artwork}
          artworkLocal={game?.artwork_local}
          role="square"
          showTitle={false}
          width={180}
          className={styles.squareArt}
        />
      </div>
    </section>
  )
}
