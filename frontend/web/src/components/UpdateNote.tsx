import { useEffect, useState } from 'react'
import { Button } from './Button'
import { applyUpdate, checkUpdate } from '../daemon/api'
import { useDaemon } from '../daemon/store'
import { useT } from '../i18n/t'
import styles from './UpdateNote.module.css'

export function UpdateNote() {
  const t = useT()
  const connection = useDaemon((state) => state.connection)
  const [version, setVersion] = useState('')
  const [busy, setBusy] = useState(false)
  const [note, setNote] = useState('')

  useEffect(() => {
    if (connection !== 'live') return
    let live = true
    void checkUpdate()
      .then((result) => {
        if (live && result.available) setVersion(result.version ?? '')
      })
      .catch(() => {})
    return () => {
      live = false
    }
  }, [connection])

  if (!version && !note) return null

  return (
    <div className={styles.note}>
      <span className={styles.text}>
        {note || (version ? `${t('Hikari')} ${version} ${t('is available')}` : '')}
      </span>
      {version && !note ? (
        <Button
          size="small"
          busy={busy}
          onClick={async () => {
            setBusy(true)
            try {
              await applyUpdate()
              setNote(t('Updating, the window restarts when it lands'))
              setVersion('')
            } catch (error) {
              setNote(error instanceof Error ? error.message : String(error))
            } finally {
              setBusy(false)
            }
          }}
        >
          {t('Update')}
        </Button>
      ) : null}
    </div>
  )
}
