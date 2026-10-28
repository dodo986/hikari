import { useCallback } from 'react'
import { DiagnosticsPanel } from '../components/DiagnosticsPanel'
import { EngineGrid } from '../components/EngineGrid'
import { PageHeader } from '../components/PageHeader'
import { Panel } from '../components/Panel'
import { SecretPanel } from '../components/SecretPanel'
import { WinePanel } from '../components/WinePanel'
import { refreshStatus } from '../daemon/link'
import { useDaemon } from '../daemon/store'
import { useT } from '../i18n/t'
import { middle } from '../lib/text'
import styles from './Settings.module.css'

function uptime(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return '0s'
  if (seconds < 60) return `${Math.floor(seconds)}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

export function Settings() {
  const t = useT()
  const status = useDaemon((state) => state.status)
  const onEnginesChanged = useCallback(() => void refreshStatus(), [])

  return (
    <div className={styles.page}>
      <PageHeader title={t('Settings')} subtitle={t('The daemon and the engines behind the library.')} />
      <div className={styles.body}>
        <Panel
          title={t('Daemon')}
          subtitle={t('One process owns the socket, the library and every engine run.')}
        >
          <div className={styles.daemon}>
            <div className={styles.stack}>
              <span className={styles.value}>{status ? `hikari ${status.version}` : 'hikari'}</span>
              {status ? <span className={styles.faint}>{`up ${uptime(status.uptime_seconds)}`}</span> : null}
            </div>
            <span className={styles.spacer} />
            <div className={styles.right}>
              <span className={styles.mark}>{t('SOCKET')}</span>
              <span className={styles.socket} title={status?.socket}>
                {status?.socket ? middle(status.socket, 48) : ''}
              </span>
            </div>
          </div>
        </Panel>

        <WinePanel onEnginesChanged={onEnginesChanged} />
        <EngineGrid engines={status?.engines} />
        <SecretPanel />
        <DiagnosticsPanel />
      </div>
    </div>
  )
}
