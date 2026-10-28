import { Button } from './Button'
import type { ModUpdate } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './UpdateBanner.module.css'

interface UpdateBannerProps {
  updates: ModUpdate[]
  disabled: boolean
  onUpdate: (row: ModUpdate) => void
}

export function UpdateBanner({ updates, disabled, onUpdate }: UpdateBannerProps) {
  const t = useT()
  const label = `${updates.length} ${updates.length === 1 ? t('update') : t('updates')} ${t('on Nexus')}`
  return (
    <section className={styles.banner}>
      <p className={styles.head}>{label}</p>
      {updates.map((row) => (
        <div className={styles.row} key={row.mod}>
          <span className={styles.name}>{row.mod}</span>
          <span className={styles.version}>
            {`${row.installed_version || '?'}  ->  ${row.latest_version || '?'}`}
          </span>
          <Button size="small" disabled={disabled} onClick={() => onUpdate(row)}>
            {t('Update')}
          </Button>
        </div>
      ))}
    </section>
  )
}
