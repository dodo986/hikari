import { useEffect, useMemo, useRef, useState } from 'react'
import { gameKey, useDaemon } from './daemon/store'
import { refreshStatus, startDaemonLink } from './daemon/link'
import { startModsLink } from './daemon/modsLink'
import { useT } from './i18n/t'
import { Button } from './components/Button'
import { Rail } from './components/Rail'
import { About } from './pages/About'
import { Accounts } from './pages/Accounts'
import { Downloads } from './pages/Downloads'
import { GameDetail } from './pages/GameDetail'
import { Mods } from './pages/Mods'
import { useHotkeys } from './lib/hotkeys'
import { handlePendingLink } from './lib/pendingLink'
import { Settings } from './pages/Settings'
import { Library } from './pages/Library'
import { parseRoute } from './router'
import type { Route } from './router'
import { startDaemon } from './host/bridge'
import styles from './App.module.css'
import { errorMessage } from './lib/errors'

function useRoute(): Route {
  const [hash, setHash] = useState(() => window.location.hash)
  useEffect(() => {
    const onChange = () => setHash(window.location.hash)
    window.addEventListener('hashchange', onChange)
    return () => window.removeEventListener('hashchange', onChange)
  }, [])
  return useMemo(() => parseRoute(hash), [hash])
}

export function App() {
  const t = useT()
  const route = useRoute()
  useHotkeys()
  const [starting, setStarting] = useState(false)
  const [startNote, setStartNote] = useState('')
  const connection = useDaemon((state) => state.connection)
  const linkError = useDaemon((state) => state.error)
  const games = useDaemon((state) => state.games)
  const installs = useDaemon((state) => Object.keys(state.installs).length)
  const running = useDaemon((state) => state.running)
  const playing = useMemo(() => Object.values(running), [running])
  const mods = useDaemon((state) => state.mods.games)
  const installed = useMemo(() => games.filter((game) => game.installed).length, [games])
  const titles = useMemo(
    () => Object.fromEntries(games.map((game) => [gameKey(game.store, game.id), game.title])),
    [games],
  )

  useEffect(() => {
    const stopDaemon = startDaemonLink()
    const stopMods = startModsLink()
    void refreshStatus().catch(() => {})
    return () => {
      stopMods()
      stopDaemon()
    }
  }, [])

  // A browser hands nxm:// links to this window; the install starts once the
  // daemon is reachable, and the mods page reports what happened.
  const linkHandled = useRef(false)
  useEffect(() => {
    if (connection !== 'live' || linkHandled.current) return
    if (!window.__HIKARI_LINK__) return
    linkHandled.current = true
    handlePendingLink()
      .then((note) => {
        if (note) useDaemon.getState().setMods({ error: note })
      })
      .catch((error: unknown) => {
        useDaemon
          .getState()
          .setMods({ error: errorMessage(error) })
      })
  }, [connection])

  return (
    <div className={styles.app}>
      <Rail
        route={route}
        games={games.length}
        titles={titles}
        installed={installed}
        playing={playing}
        mods={mods}
        installs={installs}
        connection={connection}
      />
      <main className={styles.main} key={`${route.page}/${route.store ?? ''}/${route.id ?? ''}/${route.game ?? ''}`}>
        {connection === 'offline' || connection === 'unauthorized' ? (
          <div className={styles.offline}>
            <h1 className={styles.offlineTitle}>
              {connection === 'unauthorized'
                ? t('The daemon refused this window')
                : t('The Hikari daemon is not running')}
            </h1>
            <p className={styles.offlineText}>
              {connection === 'unauthorized'
                ? t('Its token changed since this window opened. Reopen Hikari to pick up the new one.')
                : t(
                    'Games, logins and installs all come from hikarid. Start it, or run scripts/dev.sh from the repository.',
                  )}
            </p>
            <Button
              variant="primary"
              busy={starting}
              onClick={() => {
                setStarting(true)
                startDaemon()
                  .then(() => setStartNote(t('Starting hikarid')))
                  .catch((error: unknown) =>
                    setStartNote(errorMessage(error)),
                  )
                  .finally(() => setStarting(false))
              }}
            >
              {t('Start daemon')}
            </Button>
            {startNote || linkError ? (
              <p className={styles.offlineNote}>{startNote || linkError}</p>
            ) : null}
          </div>
        ) : route.page === 'library' ? (
          <Library />
        ) : route.page === 'store' ? (
          <Library storeMode />
        ) : route.page === 'game' && route.store && route.id ? (
          <GameDetail store={route.store} id={route.id} />
        ) : route.page === 'mods' ? (
          <Mods />
        ) : route.page === 'settings' ? (
          <Settings />
        ) : route.page === 'downloads' ? (
          <Downloads />
        ) : route.page === 'accounts' ? (
          <Accounts />
        ) : route.page === 'about' ? (
          <About />
        ) : (
          <div className={styles.stub}>{t('Not ported yet')}</div>
        )}
      </main>
    </div>
  )
}
