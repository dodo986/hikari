import { useEffect, useState } from 'react'
import { Button } from './Button'
import { Panel } from './Panel'
import { report, tailLog } from '../daemon/api'
import { useDaemon } from '../daemon/store'
import { useT } from '../i18n/t'
import { middle } from '../lib/text'
import styles from './DiagnosticsPanel.module.css'
import { errorMessage } from '../lib/errors'

export function DiagnosticsPanel() {
  const t = useT()
  const [lines, setLines] = useState<string[]>([])
  const [path, setPath] = useState('')
  const [status, setStatus] = useState('')
  const connection = useDaemon((state) => state.connection)

  const load = async () => {
    try {
      const result = await tailLog(200)
      setLines(result.lines ?? [])
      setPath(result.path ?? '')
    } catch (error) {
      setStatus(errorMessage(error))
    }
  }

  useEffect(() => {
    if (connection !== 'live') return
    void load()
  }, [connection])

  const copy = async () => {
    try {
      const { text } = await report()
      await navigator.clipboard?.writeText(text)
      setStatus(t('Report copied to the clipboard'))
    } catch (error) {
      setStatus(error instanceof Error ? error.message : t('no report available'))
    }
  }

  return (
    <Panel
      title={t('Diagnostics')}
      note={t(
        "The daemon's own log. Copy a report when something fails; it has versions, engine paths and the recent log, no credentials.",
      )}
    >
      <div className={styles.bar}>
        <span className={styles.path} title={path}>
          {middle(path, 64)}
        </span>
        <Button size="small" onClick={() => void load()}>
          {t('Refresh')}
        </Button>
        <Button size="small" onClick={() => void copy()}>
          {t('Copy report')}
        </Button>
      </div>
      {status ? <p className={styles.status}>{status}</p> : null}
      <pre className={styles.log}>{lines.join('\n') || t('no log yet')}</pre>
    </Panel>
  )
}
