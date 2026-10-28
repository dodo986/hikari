import { Panel } from './Panel'
import type { Game } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './FactsPanel.module.css'

interface FactsPanelProps {
  game: Game | null
  settings: { target_exe?: string; wine_prefix?: string } | null
  size: string
}

const EMPTY = 'n/a'

export function FactsPanel({ game, size }: FactsPanelProps) {
  const t = useT()
  const row = (label: string, value: string | undefined) => (
    <div className={styles.row} key={label}>
      <span className={styles.label}>{label}</span>
      <span className={styles.value}>{value && value.length > 0 ? value : EMPTY}</span>
    </div>
  )

  return (
    <Panel
      title={t('Game details')}
      subtitle={t('What the daemon will start, and the rest of its record for this title.')}
    >
      <div className={styles.rows}>
        {row(t('Store'), game?.store.toUpperCase())}
        {row(t('Platform'), game?.platform)}
        {row(t('Runner'), game?.runner)}
        {row(t('Version'), game?.version)}
        {row(t('Install size'), size)}
        {row(t('Install path'), game?.install_path)}
        {row(t('Executable'), game?.executable)}
        {row(
          t('Source'),
          game?.source === 'heroic-import' ? t('Heroic import') : t('Hikari'),
        )}
        {row(t('Offline play'), game?.can_run_offline ? t('Available') : t('Not reported'))}
        {row(t('Cloud saves'), game?.cloud_saves ? t('Supported') : t('Not reported'))}
      </div>
    </Panel>
  )
}
