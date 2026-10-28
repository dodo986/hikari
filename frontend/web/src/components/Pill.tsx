import type { ReactNode } from 'react'
import styles from './Pill.module.css'

interface PillProps {
  on: boolean
  onSelect: () => void
  children: ReactNode
  title?: string
}

export function Pill({ on, onSelect, children, title }: PillProps) {
  return (
    <button type="button" className={styles.pill} data-on={on} aria-pressed={on} title={title} onClick={onSelect}>
      {children}
    </button>
  )
}
