import { useMemo, useState } from 'react'
import { GameArt } from '../components/GameArt'
import { PageHeader } from '../components/PageHeader'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { EmptyState } from '../components/EmptyState'
import { Card } from '../components/Card'
import { cancelInstall, stopGame } from '../daemon/api'
import { gameKey, useDaemon } from '../daemon/store'
import { errorMessage } from '../lib/errors'
import { useT } from '../i18n/t'
import { formatPercent, formatSpeed } from '../lib/format'
import styles from './Downloads.module.css'

const FINISHED = new Set(['error', 'canceled', 'failed', 'cancelled'])

export function Downloads() {
  const t = useT()
  const installs = useDaemon((state) => state.installs)
  const clearProgress = useDaemon((state) => state.clearProgress)
  const [error, setError] = useState('')
  const running = useDaemon((state) => state.running)
  const games = useDaemon((state) => state.games)

  const rows = useMemo(() => Object.values(installs), [installs])
  const playing = useMemo(() => Object.values(running), [running])
  const byKey = useMemo(
    () => new Map(games.map((game) => [gameKey(game.store, game.id), game])),
    [games],
  )

  const title = (store: string, id: string): string =>
    byKey.get(gameKey(store, id))?.title ?? id

  return (
    <div className={styles.page}>
      <PageHeader
        title={t('Downloads')}
        subtitle={`${rows.length} ${t('in flight')}  ·  ${playing.length} ${t('playing')}`}
        actions={rows.length + playing.length > 0 ? <Badge accent>{t('ACTIVE')}</Badge> : null}
      />

      {error ? <p className={styles.error}>{error}</p> : null}

      {rows.length === 0 && playing.length === 0 ? (
        <EmptyState
          title={t('Nothing downloading')}
          note={t('Installs and running games appear here.')}
        />
      ) : null}

      {playing.length > 0 ? <p className={styles.mark}>{t('PLAYING NOW')}</p> : null}
      {playing.map((event) => {
        const game = byKey.get(gameKey(event.store, event.id))
        return (
          <Card key={`run-${gameKey(event.store, event.id)}`}>
            <div className={styles.row}>
              <GameArt
                title={game?.title ?? title(event.store, event.id)}
                artwork={game?.artwork}
                artworkLocal={game?.artwork_local}
                role="square"
                showTitle={false}
                className={styles.artPlay}
              />
              <div className={styles.text}>
                <span className={styles.name}>{title(event.store, event.id)}</span>
                <span className={styles.meta}>
                  {event.store.toUpperCase()}  · {' '}
                  {event.state === 'launching' ? t('starting') : t('running')}
                  {event.pid ? `  ·  pid ${event.pid}` : ''}
                </span>
              </div>
              <Button
                size="small"
                onClick={() =>
                  void stopGame({ store: event.store, id: event.id }).catch((failure: unknown) =>
                    setError(errorMessage(failure)),
                  )
                }
              >
                {t('Stop')}
              </Button>
            </div>
          </Card>
        )
      })}

      {rows.length > 0 ? <p className={styles.mark}>{t('DOWNLOADS')}</p> : null}
      {rows.map((progress) => {
        const game = byKey.get(gameKey(progress.store, progress.id))
        const percent = typeof progress.percent === 'number' ? progress.percent : null
        const finished = FINISHED.has(progress.state ?? '')
        const meta = [
          formatPercent(percent),
          progress.bytes ?? '',
          progress.eta ? `${t('eta')} ${progress.eta}` : '',
          formatSpeed(progress.down_speed),
        ]
          .filter(Boolean)
          .join('  ·  ')
        return (
          <Card key={`job-${gameKey(progress.store, progress.id)}`}>
            <div className={styles.row}>
              <GameArt
                title={game?.title ?? title(progress.store, progress.id)}
                artwork={game?.artwork}
                artworkLocal={game?.artwork_local}
                role="square"
                showTitle={false}
                className={styles.artJob}
              />
              <div className={styles.text}>
                <span className={styles.head}>
                  <span className={styles.name}>{title(progress.store, progress.id)}</span>
                  <Badge accent>{progress.state}</Badge>
                </span>
                {finished ? null : (
                  <span
                    className={styles.bar}
                    role="progressbar"
                    aria-label={game?.title ?? progress.id}
                    aria-valuemin={0}
                    aria-valuemax={100}
                    aria-valuenow={percent ?? 0}
                  >
                    <span className={styles.fill} style={{ width: `${percent ?? 0}%` }} />
                  </span>
                )}
                <span className={styles.meta}>{meta}</span>
              </div>
              <Button
                size="small"
                onClick={() =>
                  finished
                    ? clearProgress(gameKey(progress.store, progress.id))
                    : void cancelInstall({ store: progress.store, id: progress.id }).catch((error: unknown) =>
                        setError(errorMessage(error)),
                      )
                }
              >
                {finished ? t('Dismiss') : t('Cancel')}
              </Button>
            </div>
          </Card>
        )
      })}
    </div>
  )
}
