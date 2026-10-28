import { useEffect, useState } from 'react'
import { Badge } from './Badge'
import { Button } from './Button'
import { Panel } from './Panel'
import { installWine, removeWine, wineReleases, wineVersions } from '../daemon/api'
import { rpc } from '../daemon/link'
import type { WineRelease, WineVersion } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './WinePanel.module.css'
import { errorMessage } from '../lib/errors'
import { formatBytes } from '../lib/format'

interface WinePanelProps {
  onEnginesChanged: () => void
}

export function WinePanel({ onEnginesChanged }: WinePanelProps) {
  const t = useT()
  const [versions, setVersions] = useState<WineVersion[]>([])
  const [releases, setReleases] = useState<WineRelease[]>([])
  const [open, setOpen] = useState(false)
  const [status, setStatus] = useState('')
  const [bad, setBad] = useState(false)
  const [busy, setBusy] = useState(false)

  const load = () => {
    void wineVersions()
      .then((result) => setVersions(result.versions ?? []))
      .catch(() => setVersions([]))
  }

  useEffect(load, [])

  useEffect(() => {
    const off = rpc.on('wine.progress', (params) => {
      const event = params as { tag?: string; phase?: string; percent?: number; done?: boolean; error?: string; name?: string }
      setBad(Boolean(event.error))
      if (event.done) {
        setBusy(false)
        setStatus(event.error ? event.error : `${t('Installed')} ${event.name ?? event.tag ?? ''}`.trim())
        load()
        onEnginesChanged()
        return
      }
      if (event.error) {
        setStatus(event.error)
        return
      }
      const percent = typeof event.percent === 'number' ? `${Math.round(event.percent)}%` : ''
      setStatus([event.phase, percent, event.tag ? `· ${event.tag}` : ''].filter(Boolean).join(' '))
    })
    return off
  }, [onEnginesChanged, t])

  const toggleReleases = async () => {
    if (open) {
      setOpen(false)
      return
    }
    setOpen(true)
    setBad(false)
    setStatus(t('loading releases…'))
    try {
      const result = await wineReleases()
      setReleases((result.releases ?? []).slice(0, 8))
      setStatus('')
    } catch (error) {
      setBad(true)
      setStatus(errorMessage(error))
    }
  }

  const download = async (release: WineRelease) => {
    if (busy) return
    setBusy(true)
    setBad(false)
    setStatus(t('starting'))
    try {
      await installWine(release.tag)
    } catch (error) {
      setBusy(false)
      setBad(true)
      setStatus(errorMessage(error))
    }
  }

  const remove = async (version: WineVersion) => {
    setBad(false)
    try {
      await removeWine(version.name)
      setStatus(`${t('Removed')} ${version.name}`)
      load()
      onEnginesChanged()
    } catch (error) {
      setBad(true)
      setStatus(errorMessage(error))
    }
  }

  return (
    <Panel
      title={t('Wine & Proton')}
      note={t('Builds Hikari can run games with, and one-click Proton downloads from the same releases Heroic uses.')}
    >
      <p className={styles.mark}>{t('INSTALLED')}</p>
      {versions.length === 0 ? <p className={styles.empty}>{t('none found')}</p> : null}
      <div className={styles.list}>
        {versions.map((version) => (
          <div className={styles.row} key={version.bin}>
            <span className={styles.name}>{version.name}</span>
            <Badge>{version.type === 'proton' ? t('PROTON') : t('WINE')}</Badge>
            <Badge accent={version.source === 'hikari'}>{(version.source || 'system').toUpperCase()}</Badge>
            {version.source === 'hikari' ? (
              <Button size="small" onClick={() => void remove(version)}>
                {t('Remove')}
              </Button>
            ) : null}
          </div>
        ))}
      </div>

      <div className={styles.actions}>
        <Button size="small" onClick={() => void toggleReleases()}>
          {open ? t('Hide downloads') : t('Download Proton')}
        </Button>
        <span className={bad ? styles.error : styles.status}>{status}</span>
      </div>

      {open ? (
        <div className={styles.list}>
          {releases.map((release) => (
            <div className={styles.row} key={release.tag}>
              <span className={styles.name}>{`${release.tag}  ·  ${release.variant}`}</span>
              <span className={styles.size}>{formatBytes(release.size)}</span>
              <Button size="small" onClick={() => void download(release)}>
                {t('Install')}
              </Button>
            </div>
          ))}
        </div>
      ) : null}
    </Panel>
  )
}
