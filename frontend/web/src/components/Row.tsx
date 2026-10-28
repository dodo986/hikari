import type { ReactNode } from 'react'
import styles from './Row.module.css'

interface RowProps {
  label: string
  note?: string
  children?: ReactNode
  align?: 'center' | 'start'
}

export function Row({ label, note, children, align = 'center' }: RowProps) {
  return (
    <div className={styles.row} data-align={align}>
      <div className={styles.text}>
        <span className={styles.label}>{label}</span>
        {note ? <span className={styles.note}>{note}</span> : null}
      </div>
      {children ? <div className={styles.control}>{children}</div> : null}
    </div>
  )
}

interface FieldProps {
  label: string
  note?: string
  children: ReactNode
}

export function Field({ label, note, children }: FieldProps) {
  return (
    <label className={styles.field}>
      <span className={styles.label}>{label}</span>
      {children}
      {note ? <span className={styles.note}>{note}</span> : null}
    </label>
  )
}
