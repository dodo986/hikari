import type { ReactNode } from 'react'
import styles from './Badge.module.css'

interface BadgeProps {
  children: ReactNode
  accent?: boolean
  solid?: boolean
  muted?: boolean
  danger?: boolean
}

export function Badge({ children, accent = false, solid = false, muted = false, danger = false }: BadgeProps) {
  const classes = [styles.badge]
  if (accent) classes.push(styles.accent)
  if (solid) classes.push(styles.solid)
  if (muted) classes.push(styles.muted)
  if (danger) classes.push(styles.danger)
  return <span className={classes.join(' ')}>{children}</span>
}
