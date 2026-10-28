import type { ReactNode } from 'react'
import styles from './Badge.module.css'

interface BadgeProps {
  children: ReactNode
  accent?: boolean
  solid?: boolean
}

export function Badge({ children, accent = false, solid = false }: BadgeProps) {
  const classes = [styles.badge]
  if (accent) classes.push(styles.accent)
  if (solid) classes.push(styles.solid)
  return <span className={classes.join(' ')}>{children}</span>
}
