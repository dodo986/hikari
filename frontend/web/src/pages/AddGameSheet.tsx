import { useState } from 'react'
import { rpc } from '../daemon/link'
import { searchArtwork } from '../daemon/api'
import { refreshLibrary } from '../daemon/link'
import { useT } from '../i18n/t'
import { Button } from '../components/Button'
import { Sheet } from '../components/Sheet'
import styles from './AddGameSheet.module.css'
import { errorMessage } from '../lib/errors'

interface AddGameSheetProps {
  open: boolean
  onClose: () => void
}

interface Candidate {
  url: string
  thumb?: string
}

export function AddGameSheet({ open, onClose }: AddGameSheetProps) {
  const t = useT()
  const [title, setTitle] = useState('')
  const [executable, setExecutable] = useState('')
  const [platform, setPlatform] = useState<'linux' | 'windows'>('linux')
  const [covers, setCovers] = useState<Candidate[]>([])
  const [cover, setCover] = useState('')
  const [note, setNote] = useState('')
  const [failed, setFailed] = useState(false)
  const [searching, setSearching] = useState(false)

  const reset = () => {
    setTitle('')
    setExecutable('')
    setPlatform('linux')
    setCovers([])
    setCover('')
    setNote('')
    setFailed(false)
  }

  const close = () => {
    reset()
    onClose()
  }

  const findCovers = async () => {
    if (title.trim().length < 2) {
      setNote(t('type a title to fetch covers'))
      setFailed(true)
      return
    }
    setSearching(true)
    try {
      const result = await searchArtwork(title.trim())
      setCovers(result.candidates ?? [])
      setNote(result.candidates?.length ? '' : t('none found'))
      setFailed(false)
    } catch (error) {
      setNote(errorMessage(error))
      setFailed(true)
    } finally {
      setSearching(false)
    }
  }

  const browse = async () => {
    try {
      const picked = await window.hikari?.pickFile(t('All files (*)'))
      if (picked) setExecutable(picked)
    } catch (error) {
      setNote(errorMessage(error))
      setFailed(true)
    }
  }

  const submit = async () => {
    if (title.trim() === '' || executable.trim() === '') {
      setNote(t('A title and an absolute executable path are required'))
      setFailed(true)
      return
    }
    try {
      await rpc.call('game.add_local', {
        title: title.trim(),
        executable: executable.trim(),
        platform,
        artwork: cover,
      })
      await refreshLibrary()
      close()
    } catch (error) {
      setNote(errorMessage(error))
      setFailed(true)
    }
  }

  return (
    <Sheet
      open={open}
      title={t('Add a local game')}
      closeLabel={t('Close')}
      onClose={close}
      footer={
        <div className={styles.foot}>
          <span className={failed ? `${styles.note} ${styles.error}` : styles.note}>{note}</span>
          <span className={styles.choice}>
            <Button variant="quiet" onClick={close}>
              {t('Cancel')}
            </Button>
            <Button variant="primary" onClick={() => void submit()}>
              {t('Add to library')}
            </Button>
          </span>
        </div>
      }
    >
      <div className={styles.form}>
        <div className={styles.field}>
          <label className={styles.label} htmlFor="add-title">
            {t('Title')}
          </label>
          <input
            id="add-title"
            className={styles.input}
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            autoFocus
          />
        </div>

        <div className={styles.field}>
          <span className={styles.label}>{t('Executable')}</span>
          <div className={styles.rowFields}>
            <input
              className={styles.input}
              value={executable}
              onChange={(event) => setExecutable(event.target.value)}
              placeholder="/absolute/path/to/game.exe"
            />
            <Button onClick={() => void browse()}>{t('Browse…')}</Button>
          </div>
        </div>

        <div className={styles.field}>
          <span className={styles.label}>{t('Runs with')}</span>
          <div className={styles.choice}>
            <Button
              variant={platform === 'linux' ? 'primary' : 'default'}
              onClick={() => setPlatform('linux')}
            >
              {t('Linux (native)')}
            </Button>
            <Button
              variant={platform === 'windows' ? 'primary' : 'default'}
              onClick={() => setPlatform('windows')}
            >
              {t('Windows (Wine/Proton)')}
            </Button>
          </div>
        </div>

        <div className={styles.field}>
          <span className={styles.label}>{t('Cover')}</span>
          <div className={styles.rowFields}>
            <input
              className={styles.input}
              value={cover}
              onChange={(event) => setCover(event.target.value)}
              placeholder={t('type a title to fetch covers')}
            />
            <Button onClick={() => void findCovers()} busy={searching}>
              {t('Search covers')}
            </Button>
          </div>
          {covers.length > 0 ? (
            <div className={styles.covers}>
              {covers.slice(0, 12).map((candidate) => (
                <button
                  key={candidate.url}
                  type="button"
                  className={[styles.cover, cover === candidate.url ? styles.coverActive : ''].join(' ')}
                  onClick={() => setCover(candidate.url)}
                  aria-label={t('Cover')}
                >
                  <img src={candidate.thumb || candidate.url} alt="" loading="lazy" />
                </button>
              ))}
            </div>
          ) : null}
        </div>
      </div>
    </Sheet>
  )
}
