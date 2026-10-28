import type { ButtonHTMLAttributes, ReactNode } from 'react'
import styles from './Button.module.css'

type Variant = 'default' | 'primary' | 'quiet' | 'danger'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: 'default' | 'small'
  busy?: boolean
  children: ReactNode
}

export function Button({
  variant = 'default',
  size = 'default',
  busy = false,
  disabled,
  children,
  className,
  ...rest
}: ButtonProps) {
  const classes = [styles.button]
  if (variant !== 'default') classes.push(styles[variant])
  if (size === 'small') classes.push(styles.small)
  if (className) classes.push(className)
  return (
    <button {...rest} className={classes.join(' ')} disabled={disabled || busy} aria-busy={busy}>
      {children}
    </button>
  )
}
