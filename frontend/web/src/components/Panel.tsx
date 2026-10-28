import type { ReactNode } from 'react'
import styles from './Panel.module.css'

interface PanelProps {
  title: string
  subtitle?: string
  note?: string
  action?: ReactNode
  children: ReactNode
}

export function Panel({ title, subtitle, note, action, children }: PanelProps) {
  return (
    <section className={styles.panel}>
      <header className={styles.head}>
        <div className={styles.text}>
          <h2 className={styles.title}>{title}</h2>
          {subtitle ?? note ? <p className={styles.subtitle}>{subtitle ?? note}</p> : null}
        </div>
        {action ? <div className={styles.action}>{action}</div> : null}
      </header>
      <div className={styles.body}>{children}</div>
    </section>
  )
}
