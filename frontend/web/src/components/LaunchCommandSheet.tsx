import { useEffect, useState } from 'react'
import { Sheet } from './Sheet'
import { dryRun } from '../daemon/api'
import type { Key, LaunchPlan } from '../daemon/api'
import { useT } from '../i18n/t'
import styles from './TextSheet.module.css'

interface LaunchCommandSheetProps {
  open: boolean
  game: Key
  onClose: () => void
}

export function LaunchCommandSheet({ open, game, onClose }: LaunchCommandSheetProps) {
  const t = useT()
  const [plan, setPlan] = useState<LaunchPlan | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) return
    setPlan(null)
    setError('')
    setLoading(true)
    void dryRun(game)
      .then((result) => {
        if (result.plan?.error) setError(result.plan.error)
        else setPlan(result.plan ?? null)
      })
      .catch((failure: Error) => setError(failure.message))
      .finally(() => setLoading(false))
  }, [open, game])

  const text = plan
    ? [
        ...Object.entries(plan.env ?? {}).map(([name, value]) => `${name}=${value}`),
        '',
        (plan.argv ?? []).join(' '),
      ].join('\n')
    : ''

  return (
    <Sheet
      open={open}
      title={t('Launch command')}
      closeLabel={t('Close')}
      onClose={onClose}
    >
      <p className={styles.subtitle}>
        {t('Exactly what the daemon would execute, including its environment diff.')}
      </p>
      {loading ? <p className={styles.note}>{t('Loading…')}</p> : null}
      {error ? <p className={styles.error}>{error}</p> : null}
      {!loading && !error && !plan ? <p className={styles.error}>{t('no plan')}</p> : null}
      {plan ? <pre className={styles.code}>{text}</pre> : null}
    </Sheet>
  )
}
