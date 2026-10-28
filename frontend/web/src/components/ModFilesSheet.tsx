import { useEffect, useState } from 'react'
import { Button } from './Button'
import { Sheet } from './Sheet'
import { modsFiles } from '../daemon/mods'
import type { ModFile } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './ModFilesSheet.module.css'

interface ModFilesSheetProps {
  open: boolean
  game: string
  modId: number
  modName: string
  busy: boolean
  onClose: () => void
  onInstall: (file: ModFile) => void
}

export function ModFilesSheet({ open, game, modId, modName, busy, onClose, onInstall }: ModFilesSheetProps) {
  const t = useT()
  const [files, setFiles] = useState<ModFile[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [archived, setArchived] = useState(false)

  useEffect(() => {
    if (!open || !modId) return
    setArchived(false)
    setFiles([])
    setError('')
    setLoading(true)
    void modsFiles(game, modId)
      .then((result) => {
        setFiles(result.files ?? [])
        if (result.error) setError(result.error)
      })
      .catch((failure: Error) => setError(failure.message))
      .finally(() => setLoading(false))
  }, [open, game, modId])

  const hidden = files.filter((file) => file.category === 'ARCHIVED' || file.category === 'REMOVED')
  const shown = archived ? files : files.filter((file) => !hidden.includes(file))

  return (
    <Sheet
      open={open}
      title={modName || t('Files')}
      closeLabel={t('Close')}
      onClose={onClose}
      footer={
        <>
          {hidden.length > 0 ? (
            <Button size="small" onClick={() => setArchived((value) => !value)}>
              {archived ? t('Hide archived') : t('Show archived')}
            </Button>
          ) : null}
          <Button size="small" onClick={onClose}>
            {t('Close')}
          </Button>
        </>
      }
    >
      <p className={styles.subtitle}>{t('Pick the file to download and install; it unpacks into this profile.')}</p>
      {loading ? <p className={styles.faint}>{t('Asking Nexus for the file list…')}</p> : null}
      {error ? <p className={styles.error}>{error}</p> : null}
      {!loading && !error && files.length === 0 ? (
        <p className={styles.faint}>{t('Nexus lists no files for this mod.')}</p>
      ) : null}
      <div className={styles.list}>
        {shown.map((file) => (
          <div className={styles.row} key={file.file_id}>
            <div className={styles.text}>
              <span className={styles.name}>{file.name}</span>
              <span className={styles.meta}>
                {[file.category, file.version, file.size, file.uploaded].filter(Boolean).join('  ·  ')}
              </span>
            </div>
            <Button
              size="small"
              variant={file.primary ? 'primary' : 'quiet'}
              disabled={busy}
              onClick={() => onInstall(file)}
            >
              {t('Install')}
            </Button>
          </div>
        ))}
      </div>
      {hidden.length > 0 ? (
        <p className={styles.faint}>{`${hidden.length} ${t('archived builds hidden')}`}</p>
      ) : null}
    </Sheet>
  )
}
