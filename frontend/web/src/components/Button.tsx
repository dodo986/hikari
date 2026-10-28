import type { ButtonHTMLAttributes, ReactNode } from 'react'
import styles from './Button.module.css'

type Variant = 'default' | 'primary' | 'quiet' | 'danger'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: 'default' | 'small'
  wide?: boolean
  busy?: boolean
  children: ReactNode
}

export function Button({
  variant = 'default',
  size = 'default',
  wide = false,
  busy = false,
  disabled,
  children,
  className,
  ...rest
}: ButtonProps) {
  const classes = [styles.button]
  if (variant !== 'default') classes.push(styles[variant])
  if (size === 'small') classes.push(styles.small)
  if (wide) classes.push(styles.wide)
  if (className) classes.push(className)
  return (
    <button {...rest} className={classes.join(' ')} disabled={disabled || busy} aria-busy={busy}>
      {children}
    </button>
  )
}
