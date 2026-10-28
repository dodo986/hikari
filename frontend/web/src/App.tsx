import { useEffect, useMemo, useState } from 'react'
import { useDaemon } from './daemon/store'
import { refreshStatus, startDaemonLink } from './daemon/link'
import { useT } from './i18n/t'
import { Button } from './components/Button'
import { Rail } from './components/Rail'
import { About } from './pages/About'
import { Accounts } from './pages/Accounts'
import { Downloads } from './pages/Downloads'
import { GameDetail } from './pages/GameDetail'
import { Mods } from './pages/Mods'
import { useHotkeys } from './lib/hotkeys'
import { Settings } from './pages/Settings'
import { Library } from './pages/Library'
import { parseRoute } from './router'
import type { Route } from './router'
import { startDaemon } from './host/bridge'
import styles from './App.module.css'

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
  const installs = useDaemon((state) => state.installs)
  const running = useDaemon((state) => state.running)
  const playing = useMemo(() => Object.values(running), [running])
  const mods = useDaemon((state) => state.mods.games)
  const installed = useMemo(() => games.filter((game) => game.installed).length, [games])
  const titles = useMemo(
    () => Object.fromEntries(games.map((game) => [`${game.store}/${game.id}`, game.title])),
    [games],
  )

  useEffect(() => {
    startDaemonLink()
    void refreshStatus().catch(() => {})
  }, [])

  return (
    <div className={styles.app}>
      <Rail
        route={route}
        games={games.length}
        titles={titles}
        installed={installed}
        playing={playing}
        mods={mods}
        installs={Object.keys(installs).length}
        connection={connection}
      />
      <main className={styles.main} key={`${route.page}/${route.store ?? ''}/${route.id ?? ''}`}>
        {connection === 'offline' ? (
          <div className={styles.offline}>
            <h1 className={styles.offlineTitle}>{t('The Hikari daemon is not running')}</h1>
            <p className={styles.offlineText}>
              {t(
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
                    setStartNote(error instanceof Error ? error.message : String(error)),
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
