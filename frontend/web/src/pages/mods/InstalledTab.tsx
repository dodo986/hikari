import { useState } from 'react'
import { Button } from '../../components/Button'
import { EmptyState } from '../../components/EmptyState'
import { RefreshButton } from '../../components/RefreshButton'
import {
  modsHealth,
  modsInstalled,
  modsMissing,
  modsRemove,
  modsRepair,
  modsToggle,
  modsUpdates,
} from '../../daemon/mods'
import type { ModsTarget } from '../../daemon/mods'
import type { ModFinding, ModsGame, ModsInstalled, ModMissing, ModsUpdates } from '../../daemon/types'
import { useT } from '../../i18n/t'
import styles from './InstalledTab.module.css'
import { errorMessage } from '../../lib/errors'

interface InstalledTabProps {
  game: ModsGame
  target: ModsTarget
  installed: ModsInstalled | null
  onInstalled: (list: ModsInstalled | null) => void
  onUpdates: (updates: ModsUpdates | null) => void
  onFailed: (message: string) => void
  onNote: (message: string) => void
  busy: boolean
  onChangeProfile: () => void
}

export function InstalledTab({
  game,
  target,
  installed,
  onInstalled,
  onUpdates,
  onFailed,
  onNote,
  busy,
  onChangeProfile,
}: InstalledTabProps) {
  const t = useT()
  const [health, setHealth] = useState<ModFinding[] | null>(null)
  const [missing, setMissing] = useState<ModMissing[] | null>(null)
  const [checking, setChecking] = useState('')
  const [pending, setPending] = useState('')
  const [loadingUpdates, setLoadingUpdates] = useState(false)

  const load = () => {
    void modsInstalled(target)
      .then(onInstalled)
      .catch((error: Error) => {
        onInstalled(null)
        onFailed(error.message)
      })
  }

  const checkHealth = async () => {
    setChecking('health')
    try {
      const result = await modsHealth(target)
      setHealth(result.findings ?? [])
    } catch (error) {
      onFailed(errorMessage(error))
    } finally {
      setChecking('')
    }
  }

  const repair = async () => {
    if (checking) return
    setChecking('repair')
    try {
      const result = await modsRepair(target)
      if (!result.job) onNote(result.detail || t('Nothing to repair'))
      else onNote(t('Repairing'))
      if (result.job) setHealth(null)
    } catch (error) {
      onFailed(errorMessage(error))
    } finally {
      setChecking('')
    }
  }

  const checkMissing = async () => {
    setChecking('missing')
    try {
      const result = await modsMissing(target)
      setMissing(result.missing ?? [])
    } catch (error) {
      onFailed(errorMessage(error))
    } finally {
      setChecking('')
    }
  }

  const checkUpdates = async () => {
    setLoadingUpdates(true)
    try {
      onUpdates(await modsUpdates(target))
    } catch (error) {
      onFailed(errorMessage(error))
    } finally {
      setLoadingUpdates(false)
    }
  }

  const toggle = async (name: string) => {
    if (busy) return
    try {
      await modsToggle(target, name)
      load()
    } catch (error) {
      onFailed(errorMessage(error))
    }
  }

  const problems = (health ?? []).filter((finding) => finding.severity !== 'info')

  return (
    <div className={styles.pane}>
      <div className={styles.bar}>
        <span className={styles.profile}>{installed?.profile || target.profile}</span>
        {game.profiles.length > 1 ? (
          <Button size="small" onClick={onChangeProfile}>
            {t('Change profile')}
          </Button>
        ) : null}
        <span className={styles.spacer} />
        <span className={styles.hint}>{t('Click a row to toggle it; deploy applies the list.')}</span>
        <Button size="small" busy={loadingUpdates} onClick={() => void checkUpdates()}>
          {t('Check updates')}
        </Button>
        <Button size="small" onClick={() => void checkHealth()} busy={checking === 'health'}>
          {t('Check health')}
        </Button>
        <Button size="small" busy={checking === 'repair'} onClick={() => void repair()}>
          {t('Repair mods')}
        </Button>
        <Button size="small" busy={checking === 'missing'} onClick={() => void checkMissing()}>
          {t('Check requirements')}
        </Button>
        <RefreshButton onClick={load} />
      </div>

      {checking === 'health' ? (
        <p className={styles.faint}>{t('Checking that every mod loader is really in the game folder…')}</p>
      ) : null}
      {health && problems.length === 0 ? (
        <p className={styles.faint}>
          {t('Nothing missing: every loader file is in the game folder and each loader has a log.')}
        </p>
      ) : null}
      {problems.map((finding) => (
        <p className={styles.finding} data-error={finding.severity === 'error'} key={finding.id}>
          {finding.mod ? `${finding.detail}  ·  ${finding.mod}` : finding.detail}
        </p>
      ))}

      {checking === 'missing' ? (
        <p className={styles.faint}>{t('Asking Nexus what these mods need…')}</p>
      ) : null}
      {missing && missing.length === 0 ? (
        <p className={styles.faint}>{t('Every listed requirement is installed.')}</p>
      ) : null}
      {missing?.map((entry) => (
        <p className={styles.finding} data-error key={entry.mod}>
          {`${entry.mod} ${t('needs')} ${entry.needs
            .map((need) => (need.external ? `${need.name} (${t('outside Nexus')})` : need.name))
            .join(', ')}`}
        </p>
      ))}

      <div className={styles.rows}>
        {(installed?.mods ?? []).map((mod, index) => (
          <div
            className={styles.row}
            key={mod.name}
            onMouseLeave={() => setPending('')}
            onBlur={() => setPending('')}
            onKeyDown={(event) => {
              if (event.key === 'Escape') setPending('')
            }}
          >
            <button
              type="button"
              className={styles.toggle}
              onClick={() => void toggle(mod.name)}
              aria-pressed={mod.enabled}
            >
              <span className={styles.index}>{index + 1}</span>
              <span className={styles.box} data-on={mod.enabled} />
              <span className={styles.name} data-off={!mod.enabled}>
                {mod.name}
              </span>
              {mod.enabled ? null : <span className={styles.off}>{t('disabled')}</span>}
            </button>
            <Button
              size="small"
              disabled={busy}
              onClick={() => {
                if (pending !== mod.name) {
                  setPending(mod.name)
                  return
                }
                setPending('')
                void modsRemove(target, mod.name)
                  .then(load)
                  .catch((error: unknown) => onFailed(errorMessage(error)))
              }}
            >
              {pending === mod.name ? t('Remove?') : t('Uninstall')}
            </Button>
          </div>
        ))}
      </div>

      {installed && installed.mods.length === 0 ? (
        <div className={styles.plate}>
          <EmptyState title={t('Nothing installed in this profile yet, browse Nexus or install an archive.')} />
        </div>
      ) : null}
    </div>
  )
}
