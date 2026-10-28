import { useEffect, useRef, useState } from 'react'
import { Button } from './Button'
import { Sheet } from './Sheet'
import type { Key } from '../daemon/api'
import { runLog } from '../daemon/api'
import type { RunLog } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './TextSheet.module.css'

interface LaunchLogSheetProps {
  open: boolean
  game: Key
  initial: RunLog | null
  onNote: (note: string) => void
  onClose: () => void
}

const EMPTY = 'No output was recorded. The game may have failed before it started; see the command the launcher built under Launch command.'

export function LaunchLogSheet({ open, game, initial, onNote, onClose }: LaunchLogSheetProps) {
  const t = useT()
  const [current, setCurrent] = useState<RunLog | null>(initial)
  const body = useRef<HTMLPreElement>(null)

  useEffect(() => {
    if (open) setCurrent(initial)
  }, [open, initial])

  const summary = (() => {
    if (!current) return t('No launch recorded yet.')
    const state = current.running ? t('running') : current.state || t('ended')
    return current.exit_code ? `${state}  ·  ${t('exit')} ${current.exit_code}` : state
  })()

  return (
    <Sheet
      open={open}
      title={t('Launch log')}
      closeLabel={t('Close')}
      onClose={onClose}
      footer={
        <>
          <Button
            size="small"
            onClick={async () => {
              try {
                setCurrent(await runLog(game))
              } catch {
                onNote(t('No launch recorded yet.'))
              }
            }}
          >
            {t('Refresh')}
          </Button>
          <Button
            size="small"
            onClick={() => {
              void navigator.clipboard?.writeText(current?.lines.join('\n') ?? '')
              onNote(t('Copied'))
            }}
          >
            {t('Copy')}
          </Button>
        </>
      }
    >
      <p className={styles.subtitle}>{t('The last launch of this game, its output and the reason it stopped.')}</p>
      <p className={styles.note}>{summary}</p>
      {current?.error ? <p className={styles.error}>{current.error}</p> : null}
      {current?.mod_stop ? (
        <div className={styles.error}>
          <p>{current.mod_stop}</p>
        </div>
      ) : null}
      <pre className={styles.log} ref={body}>
        {current?.lines?.length ? current.lines.join('\n') : t(EMPTY)}
      </pre>
    </Sheet>
  )
}
