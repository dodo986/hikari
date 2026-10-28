import { Button } from './Button'
import { useT } from '../i18n/t'

interface RefreshButtonProps {
  onClick: () => void
  busy?: boolean
  disabled?: boolean
}

export function RefreshButton({ onClick, busy = false, disabled = false }: RefreshButtonProps) {
  const t = useT()
  return (
    <Button size="small" busy={busy} disabled={disabled} onClick={onClick}>
      {busy ? t('Refreshing…') : `↻ ${t('Refresh')}`}
    </Button>
  )
}
