import type { ReactNode } from 'react'
import styles from './EmptyState.module.css'

interface EmptyStateProps {
  title: string
  note?: string
  action?: ReactNode
}

export function EmptyState({ title, note, action }: EmptyStateProps) {
  return (
    <div className={styles.empty}>
      <p className={styles.title}>{title}</p>
      {note ? <p className={styles.note}>{note}</p> : null}
      {action ? <div className={styles.action}>{action}</div> : null}
    </div>
  )
}
