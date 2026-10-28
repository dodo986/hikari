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
  const connection = useDaemon((state) => state.connection)
  const games = useDaemon((state) => state.games)
  const status = useDaemon((state) => state.status)
  const installed = useMemo(() => games.filter((game) => game.installed).length, [games])

  useEffect(() => {
    startDaemonLink()
    void refreshStatus().catch(() => {})
  }, [])

  const selection = status ? `hikarid ${status.version}` : ''

  return (
    <div className={styles.app}>
      <Rail
        route={route}
        games={games.length}
        installed={installed}
        connection={connection}
        selection={selection}
      />
      <main className={styles.main}>
        {connection === 'offline' ? (
          <div className={styles.offline}>
            <h1 className={styles.offlineTitle}>{t('The Hikari daemon is not running')}</h1>
            <p className={styles.offlineText}>
              {t(
                'Games, logins and installs all come from hikarid. Start it, or run scripts/dev.sh from the repository.',
              )}
            </p>
            <code className={styles.code}>systemctl --user start hikarid</code>
            <Button variant="primary" onClick={() => void startDaemon().catch(() => {})}>
              {t('Start daemon')}
            </Button>
          </div>
        ) : route.page === 'library' ? (
          <Library />
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
