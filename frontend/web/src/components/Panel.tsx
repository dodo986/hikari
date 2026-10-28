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
        <div className={styles.text}>
          <h2 className={styles.title}>{title}</h2>
          {note ? <p className={styles.subtitle}>{note}</p> : null}
        </div>
        {action ? <div className={styles.action}>{action}</div> : null}
      </header>
      <div className={styles.body}>{children}</div>
    </section>
  )
}
