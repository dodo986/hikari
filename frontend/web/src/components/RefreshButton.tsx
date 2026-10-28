import { Button } from './Button'
import { useT } from '../i18n/t'

interface RefreshButtonProps {
  onClick: () => void
  busy?: boolean
}

export function RefreshButton({ onClick, busy = false }: RefreshButtonProps) {
  const t = useT()
  return (
    <Button size="small" busy={busy} onClick={onClick}>
      {busy ? t('Refreshing…') : `↻ ${t('Refresh')}`}
    </Button>
  )
}
