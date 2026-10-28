import { useState } from 'react'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { GameArt } from '../../components/GameArt'
import { modsConfigure } from '../../daemon/mods'
import { refreshMods } from '../../daemon/modsLink'
import { useDaemon } from '../../daemon/store'
import type { Game, ModsGame } from '../../daemon/types'
import { pickFolder } from '../../host/bridge'
import { useT } from '../../i18n/t'
import styles from './GameList.module.css'

interface GameListProps {
  onOpen: (game: ModsGame) => void
  onNote: (message: string) => void
  onFailed: (message: string) => void
}

export function profileOf(game: ModsGame, chosen: Record<string, string>): string {
  return chosen[game.id] ?? game.deployed_profile ?? game.profiles[0] ?? ''
}

export function GameList({ onOpen, onNote, onFailed }: GameListProps) {
  const t = useT()
  const games = useDaemon((state) => state.mods.games)
  const status = useDaemon((state) => state.mods.status)
  const library = useDaemon((state) => state.games)
  const [showAll, setShowAll] = useState(false)

  const ready = games.filter((game) => game.configured)
  const rest = games.filter((game) => !game.configured)
  const art = (id: string): Game | undefined => library.find((game) => game.id === id)

  const setFolder = async (game: ModsGame) => {
    const path = await pickFolder(t('Choose the game folder'))
    if (!path) return
    try {
      const result = await modsConfigure(game.id, path)
      if (result.problems?.length) onFailed(result.problems.join('; '))
      else onNote(`${game.name}: ${t('folder saved')}`)
      await refreshMods()
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <div className={styles.list}>
      {ready.length === 0 && rest.length === 0 ? (
        <div className={styles.plate}>
          <EmptyState
            title={
              status?.available
                ? t('The backend knows no games yet.')
                : t('The modding backend is not available.')
            }
          />
        </div>
      ) : null}

      {ready.map((game) => {
        const cover = art(game.id)
        return (
          <button type="button" className={styles.row} key={game.id} onClick={() => onOpen(game)}>
            <span className={styles.art}>
              <GameArt
                title={game.name}
                artwork={cover?.artwork}
                artworkLocal={cover?.artwork_local}
                role="square"
                showTitle={false}
                className={styles.cover}
              />
            </span>
            <span className={styles.text}>
              <span className={styles.name}>{game.name}</span>
              <span className={styles.meta}>
                {[profileOf(game, {}), game.nexus_domain || t('no Nexus domain'), game.store]
                  .filter(Boolean)
                  .join('  ·  ')}
              </span>
            </span>
            {game.deploy_active ? <Badge accent>{t('DEPLOYED')}</Badge> : null}
            <span className={styles.chevron}>›</span>
          </button>
        )
      })}

      {rest.length > 0 ? (
        <div className={styles.showAll}>
          <Button size="small" onClick={() => setShowAll((value) => !value)}>
            {showAll ? t('Hide them') : `${t('Show all')} ${rest.length}`}
          </Button>
          <span className={styles.hint}>
            {t('Hikari knows these games too; point one at its folder and it becomes moddable here.')}
          </span>
        </div>
      ) : null}

      {showAll
        ? rest.map((game) => (
            <div className={styles.plain} key={game.id}>
              <span className={styles.plainName}>{game.name}</span>
              <Button size="small" onClick={() => void setFolder(game)}>
                {t('Set folder')}
              </Button>
            </div>
          ))
        : null}
    </div>
  )
}
