import { Toggle } from './Toggle'
import styles from './SettingsToggle.module.css'

interface SettingsToggleProps {
  name: string
  hint: string
  checked: boolean
  available?: boolean
  onChange: (value: boolean) => void
}

export function SettingsToggle({ name, hint, checked, available = true, onChange }: SettingsToggleProps) {
  return (
    <div className={styles.row} data-unavailable={!available}>
      <div className={styles.text}>
        <span className={styles.name}>{name}</span>
        <span className={styles.hint} data-warn={!available}>
          {hint}
        </span>
      </div>
      <Toggle checked={checked} onChange={onChange} label={name} disabled={!available} />
    </div>
  )
}
