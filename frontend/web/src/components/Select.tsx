import type { ReactNode } from 'react'
import styles from './Select.module.css'

interface SelectProps {
  value: string
  onChange: (value: string) => void
  label: string
  children: ReactNode
  disabled?: boolean
}

export function Select({ value, onChange, label, children, disabled = false }: SelectProps) {
  return (
    <span className={styles.wrap}>
      <select
        className={styles.select}
        value={value}
        aria-label={label}
        disabled={disabled}
        onChange={(event) => onChange(event.currentTarget.value)}
      >
        {children}
      </select>
    </span>
  )
}
