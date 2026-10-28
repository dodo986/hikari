import { useState } from 'react'
import { Badge } from './Badge'
import { Button } from './Button'
import { Panel } from './Panel'
import { TextInput } from './TextInput'
import { installTo } from '../daemon/detail'
import type { Game, InstallInfo, InstallProgress } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './InstallPanel.module.css'

interface InstallPanelProps {
  game: Game
  info: InstallInfo | null
  progress: InstallProgress | undefined
  onStarted: (note: string) => void
  onFailed: (message: string) => void
}

export function size(bytes: number | undefined): string {
  if (!bytes || bytes <= 0) return ''
  const gb = bytes / 1024 ** 3
  if (gb >= 1) return `${gb.toFixed(1)} GiB`
  return `${Math.round(bytes / 1024 ** 2)} MiB`
}

export function InstallPanel({ game, info, progress, onStarted, onFailed }: InstallPanelProps) {
  const t = useT()
  const [path, setPath] = useState(`${game.install_path ?? '$HOME/Games/Hikari'}`.replace('$HOME', '~'))
  const [picked, setPicked] = useState<string[]>([])
  const [busy, setBusy] = useState(false)

  const toggleDlc = (id: string) =>
    setPicked((current) => (current.includes(id) ? current.filter((item) => item !== id) : [...current, id]))

  const start = async () => {
    setBusy(true)
    try {
      await installTo({ store: game.store, id: game.id }, path, game.platform ?? 'Windows', picked)
      onStarted(t('Installing'))
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    } finally {
      setBusy(false)
    }
  }

  const download = size(info?.download_size)
  const disk = size(info?.disk_size)

  return (
    <Panel
      title={t('Install')}
      subtitle={t('The engine reports the real download and disk requirements before anything is written.')}
    >
      <div className={styles.stack}>
        <div className={styles.row}>
          <span className={styles.label}>{t('Location')}</span>
          <TextInput
            label={t('Location')}
            value={path}
            onValue={setPath}
            className={styles.field}
            placeholder="~/Games/Hikari"
          />
        </div>

        {download || disk ? (
          <div className={styles.badges}>
            {download ? <Badge>{`${t('DOWNLOAD')}  ${download}`}</Badge> : null}
            {disk ? <Badge>{`${t('ON DISK')}  ${disk}`}</Badge> : null}
          </div>
        ) : null}

        {info?.dlcs?.length ? (
          <div className={styles.badges}>
            {info.dlcs.map((dlc) => (
              <button
                key={dlc.id}
                type="button"
                className={styles.pill}
                data-on={picked.includes(dlc.id)}
                onClick={() => toggleDlc(dlc.id)}
                aria-pressed={picked.includes(dlc.id)}
              >
                {dlc.title}
              </button>
            ))}
          </div>
        ) : null}

        <div className={styles.row}>
          <Button variant="primary" onClick={() => void start()} disabled={path.trim() === '' || busy}>
            {t('Start install')}
          </Button>
          {progress ? (
            <span className={styles.progress}>
              {progress.state}
              {typeof progress.percent === 'number' ? `  ${progress.percent.toFixed(1)}%` : ''}
            </span>
          ) : null}
        </div>
      </div>
    </Panel>
  )
}
