import type { InputHTMLAttributes } from 'react'
import styles from './TextInput.module.css'

interface TextInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'onChange'> {
  label: string
  onValue: (value: string) => void
}

export function TextInput({ label, onValue, className, ...rest }: TextInputProps) {
  return (
    <input
      {...rest}
      aria-label={label}
      className={[styles.input, className].filter(Boolean).join(' ')}
      onChange={(event) => onValue(event.currentTarget.value)}
    />
  )
}
