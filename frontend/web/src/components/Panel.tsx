import type { ReactNode } from 'react'
import styles from './Panel.module.css'

interface PanelProps {
  title: string
  note?: string
  action?: ReactNode
  children: ReactNode
}

export function Panel({ title, note, action, children }: PanelProps) {
  return (
    <section className={styles.panel}>
      <header className={styles.head}>
        <h2 className={styles.title}>{title}</h2>
        {action ?? (note ? <span className={styles.note}>{note}</span> : null)}
      </header>
      {children}
    </section>
  )
}
