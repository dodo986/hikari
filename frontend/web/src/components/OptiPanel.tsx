import { useEffect, useState } from 'react'
import { Badge } from './Badge'
import { Button } from './Button'
import { Panel } from './Panel'
import { TextInput } from './TextInput'
import { optiDetect, optiInstall, optiRemove, optiReleases } from '../daemon/api'
import type { Key } from '../daemon/api'
import type { OptiStatus } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './OptiPanel.module.css'

interface OptiPanelProps {
  gameKey_: Key
  status: OptiStatus | null
  installed: boolean
  onNote: (note: string) => void
  onFailed: (message: string) => void
  onChanged: () => void
}

const HOOKS = ['dxgi.dll', 'winmm.dll', 'version.dll', 'd3d12.dll']
const UPSCALERS = ['auto', 'fsr', 'xess', 'dlss']

export function OptiPanel({ gameKey_: key, status, installed, onNote, onFailed, onChanged }: OptiPanelProps) {
  const t = useT()
  const [gpu, setGpu] = useState('')
  const [latest, setLatest] = useState('')
  const [open, setOpen] = useState(false)
  const [phase, setPhase] = useState('')
  const [tag, setTag] = useState('')
  const [hook, setHook] = useState(HOOKS[0]!)
  const [upscaler, setUpscaler] = useState('auto')
  const [framegen, setFramegen] = useState(false)

  useEffect(() => {
    void optiDetect()
      .then((result) => setGpu(result.gpus?.[0]?.name ?? ''))
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!open || latest) return
    void optiReleases()
      .then((result) => setLatest(result.releases?.[0]?.tag ?? ''))
      .catch(() => {})
  }, [open, latest])

  const install = async (advanced: boolean) => {
    try {
      await optiInstall(
        key,
        advanced
          ? {
              tag: tag.trim() || latest,
              injection: hook,
              upscaler: upscaler === 'auto' ? '' : upscaler,
              framegen,
              fg_input: framegen ? 'fsrfg' : '',
            }
          : {},
      )
      onNote(t('OptiScaler installing'))
      setPhase('starting')
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    }
  }

  const remove = async () => {
    try {
      await optiRemove(key)
      onNote('')
      onChanged()
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    }
  }

  const on = status?.installed ?? false

  return (
    <Panel
      title={t('OptiScaler')}
      subtitle={t('Clean-room upscaling and frame generation, installed beside the game executable.')}
    >
      <div className={styles.stack}>
        <div className={styles.status}>
          <Badge accent={on}>{on ? t('INSTALLED') : t('NOT INSTALLED')}</Badge>
          {status?.version ? <Badge>{status.version}</Badge> : null}
          {status?.injection_dll ? <Badge>{status.injection_dll}</Badge> : null}
          <span className={styles.spacer} />
          {gpu ? <span className={styles.gpu}>{`${t('Detected GPU')}: ${gpu}`}</span> : null}
        </div>

        {status?.launch_options ? (
          <p className={styles.launch}>
            {`${t("Set this in Steam's launch options for this game")}:`}
            <br />
            {status.launch_options}
          </p>
        ) : null}

        <div className={styles.actions}>
          {on ? (
            <Button size="small" onClick={() => void remove()}>
              {t('Remove')}
            </Button>
          ) : (
            <Button
              size="small"
              variant="primary"
              disabled={!installed}
              onClick={() => void install(false)}
            >
              {t('Enable OptiScaler')}
            </Button>
          )}
          <Button size="small" onClick={() => setOpen((value) => !value)}>
            {open ? t('Hide options') : t('Options')}
          </Button>
          {phase ? <span className={styles.phase}>{phase}</span> : null}
        </div>

        {open ? (
          <div className={styles.advanced}>
            <div className={styles.row}>
              <span className={styles.label}>{t('Version')}</span>
              <TextInput
                label={t('Version')}
                value={tag}
                onValue={setTag}
                placeholder={latest || 'latest'}
                className={styles.version}
              />
            </div>
            <div className={styles.pills}>
              <span className={styles.label}>{t('Hook')}</span>
              {HOOKS.map((name) => (
                <button
                  key={name}
                  type="button"
                  className={styles.pill}
                  data-on={hook === name}
                  onClick={() => setHook(name)}
                >
                  {name}
                </button>
              ))}
            </div>
            <div className={styles.pills}>
              <span className={styles.label}>{t('Upscaler')}</span>
              {UPSCALERS.map((name) => (
                <button
                  key={name}
                  type="button"
                  className={styles.pill}
                  data-on={upscaler === name}
                  onClick={() => setUpscaler(name)}
                >
                  {name}
                </button>
              ))}
            </div>
            <div className={styles.pills}>
              <button
                type="button"
                className={styles.pill}
                data-on={framegen}
                onClick={() => setFramegen((value) => !value)}
              >
                {t('Frame generation')}
              </button>
              {framegen ? (
                <span className={styles.hint}>{t('input fsrfg (works on every GPU)')}</span>
              ) : null}
            </div>
            <Button
              size="small"
              variant="primary"
              disabled={!installed}
              onClick={() => void install(true)}
            >
              {t('Install with these options')}
            </Button>
          </div>
        ) : null}
      </div>
    </Panel>
  )
}
