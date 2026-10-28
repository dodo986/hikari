import { Button } from './Button'
import { Panel } from './Panel'
import { useDaemon } from '../daemon/store'
import { modsCancel } from '../daemon/mods'
import { openUrl } from '../host/bridge'
import { useT } from '../i18n/t'
import styles from './JobPanel.module.css'

export function JobPanel() {
  const t = useT()
  const job = useDaemon((state) => state.mods.job)
  const log = useDaemon((state) => state.mods.log)
  const authUrl = useDaemon((state) => state.mods.authUrl)

  const title = job ? `${job.phase} ${job.current}/${job.total}` : t('Working…')
  const ratio = job && job.total > 0 ? Math.min(1, job.current / job.total) : 0
  const last = log[log.length - 1] ?? ''

  return (
    <Panel title={title} note={job?.message ?? ''}>
      <div className={styles.track} aria-hidden="true">
        <div className={styles.fill} style={{ transform: `scaleX(${ratio})` }} />
      </div>
      <div className={styles.row}>
        <span className={styles.tail} title={last}>
          {last}
        </span>
        {authUrl ? (
          <Button size="small" onClick={() => void openUrl(authUrl)}>
            {t('Open the Nexus page')}
          </Button>
        ) : null}
        <Button size="small" onClick={() => void modsCancel()}>
          {t('Cancel')}
        </Button>
      </div>
    </Panel>
  )
}
