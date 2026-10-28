import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { FactsPanel } from '../components/FactsPanel'
import { GameHero } from '../components/GameHero'
import { GameSettingsSheet } from '../components/GameSettingsSheet'
import { InstallPanel } from '../components/InstallPanel'
import { LaunchCommandSheet } from '../components/LaunchCommandSheet'
import { LaunchLogSheet } from '../components/LaunchLogSheet'
import { OptiPanel } from '../components/OptiPanel'
import { stopGame } from '../daemon/api'
import { launchWith, setFavorite, updateGame } from '../daemon/detail'
import { gameKey, useDaemon } from '../daemon/store'
import type { GameSettings, LaunchOption } from '../daemon/types'
import { useT } from '../i18n/t'
import { navigate } from '../router'
import { useGameDetail } from './game/useGameDetail'
import styles from './GameDetail.module.css'

interface GameDetailProps {
  store: string
  id: string
}

export function GameDetail({ store, id }: GameDetailProps) {
  const t = useT()
  const key = useMemo(() => ({ store, id }), [store, id])
  const detail = useGameDetail(key)
  const running = useDaemon((state) => state.running[gameKey(store, id)])
  const installs = useDaemon((state) => state.installs[gameKey(store, id)])
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [commandOpen, setCommandOpen] = useState(false)
  const [logOpen, setLogOpen] = useState(false)
  const [note, setNote] = useState('')
  const [error, setError] = useState('')
  const [option, setOption] = useState<LaunchOption | null>(null)
  const [busy, setBusy] = useState(false)
  const crashedAt = useRef(0)

  const game = detail.game
  const isRunning = Boolean(running)
  const launching = running?.state === 'launching'
  const crashed = detail.crashed

  const say = useCallback((text: string) => {
    setError('')
    setNote(text)
  }, [])
  const fail = useCallback((message: string) => {
    setNote('')
    setError(message)
  }, [])

  useEffect(() => {
    if (!note) return
    const timer = window.setTimeout(() => setNote(''), 4000)
    return () => window.clearTimeout(timer)
  }, [note])

  useEffect(() => {
    if (!crashed || detail.crashed === false) return
    crashedAt.current = Date.now()
    void detail.reloadLog()
  }, [crashed, detail])

  const play = async () => {
    if (!game) return
    setBusy(true)
    try {
      await launchWith(key, option ?? undefined)
      say(t('Launching'))
    } catch (failure) {
      fail(failure instanceof Error ? failure.message : String(failure))
    } finally {
      setBusy(false)
    }
  }

  const stop = async () => {
    try {
      await stopGame(key)
      say(t('Stopped'))
    } catch (failure) {
      fail(failure instanceof Error ? failure.message : String(failure))
    }
  }

  const update = async () => {
    try {
      await updateGame(key)
      say(t('Updating'))
    } catch (failure) {
      fail(failure instanceof Error ? failure.message : String(failure))
    }
  }

  const pin = async () => {
    if (!game) return
    const next = !game.favorite
    try {
      await setFavorite(key, next)
      detail.setGame({ ...game, favorite: next })
      say(next ? t('Pinned') : t('Unpinned'))
    } catch (failure) {
      fail(failure instanceof Error ? failure.message : String(failure))
    }
  }

  const installed = game?.installed ?? false
  const size = game?.install_size ?? ''

  const status = launching
    ? t('Starting the game')
    : isRunning
      ? t('Session active')
      : crashed
        ? detail.log?.error || t('The last session crashed')
        : installed
          ? t('Ready when you are')
          : t('Choose your install options')

  return (
    <div className={styles.page}>
      <header className={styles.bar}>
        <Button size="small" onClick={() => navigate({ page: 'library' })}>
          {`← ${t('Library')}`}
        </Button>
        <span className={styles.crumb}>{`${t('LIBRARY')}  /  ${store.toUpperCase()}`}</span>
        <span className={styles.spacer} />
        {game?.update_available ? <Badge accent>{t('UPDATE AVAILABLE')}</Badge> : null}
      </header>

      <div className={styles.body}>
        <GameHero
          game={game}
          running={isRunning}
          launching={launching}
          crashed={crashed}
          size={size}
        />

        <div className={styles.actions}>
          {installed ? (
            isRunning ? (
              <Button variant="primary" onClick={() => void stop()}>
                {`■ ${t('Stop playing')}`}
              </Button>
            ) : (
              <Button variant="primary" busy={busy} onClick={() => void play()}>
                {launching ? `… ${t('Launching…')}` : `▶ ${t('Play now')}`}
              </Button>
            )
          ) : (
            <Button variant="primary" disabled>
              {`↓ ${t('Install game')}`}
            </Button>
          )}
          {installed && game?.update_available ? (
            <Button onClick={() => void update()}>{t('Update')}</Button>
          ) : null}
          {installed ? (
            <Button onClick={() => setCommandOpen(true)}>{t('Launch command')}</Button>
          ) : null}
          <Button data-testid="open-settings" onClick={() => setSettingsOpen(true)}>
            {t('Settings')}
          </Button>
          {installed ? <Button onClick={() => setLogOpen(true)}>{t('Log')}</Button> : null}
          <Button onClick={() => void pin()}>{game?.favorite ? `★ ${t('Pinned')}` : `☆ ${t('Pin')}`}</Button>
          <span className={styles.spacer} />
          {status || error ? (
            <span className={styles.status} data-crashed={crashed}>
              <span className={styles.statusLine}>{status}</span>
              {error ? <span className={styles.error}>{error}</span> : null}
              {note ? <span className={styles.note}>{note}</span> : null}
            </span>
          ) : null}
        </div>

        {!installed && game ? (
          <InstallPanel
            game={game}
            info={detail.install}
            progress={installs}
            onStarted={say}
            onFailed={fail}
          />
        ) : null}

        <FactsPanel game={game} settings={detail.settings} size={size} />

        <OptiPanel
          key={`${store}/${id}`}
          gameKey_={key}
          status={detail.opti}
          installed={installed}
          onNote={say}
          onFailed={fail}
          onChanged={detail.reloadOpti}
        />
      </div>

      {game ? (
        <>
          <GameSettingsSheet
            open={settingsOpen}
            game={game}
            settings={detail.settings}
            options={detail.options}
            onClose={() => setSettingsOpen(false)}
            onSaved={(settings: GameSettings, text: string) => {
              detail.setSettings(settings)
              say(text)
            }}
            onNote={say}
            onFailed={fail}
            onRemoved={() => navigate({ page: 'library' })}
            onLaunchOption={setOption}
          />
          <LaunchCommandSheet open={commandOpen} game={key} onClose={() => setCommandOpen(false)} />
          <LaunchLogSheet
            open={logOpen}
            game={key}
            record={detail.log}
            initial={detail.log}
            onNote={say}
            onClose={() => setLogOpen(false)}
          />
        </>
      ) : null}

    </div>
  )
}
