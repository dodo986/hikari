import { useCallback, useEffect, useState } from 'react'
import { navigate, parseRoute } from '../router'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { JobPanel } from '../components/JobPanel'
import { ModFilesSheet } from '../components/ModFilesSheet'
import { NexusKeySheet } from '../components/NexusKeySheet'
import { PageHeader } from '../components/PageHeader'
import { Panel } from '../components/Panel'
import { RefreshButton } from '../components/RefreshButton'
import { modsConfigure, modsDownloads, modsInstallArchive, modsInstallFile, modsSetup, nexusFilePage } from '../daemon/mods'
import { refreshMods } from '../daemon/modsLink'
import { useDaemon } from '../daemon/store'
import type { ModFile, ModsGame } from '../daemon/types'
import { openUrl, pickFolder } from '../host/bridge'
import { useT } from '../i18n/t'
import { GameList, profileOf } from './mods/GameList'
import { GameView } from './mods/GameView'
import styles from './Mods.module.css'
import { errorMessage } from '../lib/errors'

export function Mods() {
  const t = useT()
  const status = useDaemon((state) => state.mods.status)
  const games = useDaemon((state) => state.mods.games)
  const account = useDaemon((state) => state.mods.account)
  const job = useDaemon((state) => state.mods.job)
  const error = useDaemon((state) => state.mods.error)
  const setMods = useDaemon((state) => state.setMods)
  const connection = useDaemon((state) => state.connection)
  const [openId, setOpenId] = useState(() => parseRoute(window.location.hash).game ?? '')

  // The route owns the open game; the back button and a pasted link both change it.
  useEffect(() => {
    const sync = () => {
      const wanted = parseRoute(window.location.hash).game ?? ''
      setOpenId((current) => (current === wanted ? current : wanted))
    }
    window.addEventListener('hashchange', sync)
    return () => window.removeEventListener('hashchange', sync)
  }, [])
  const [profiles, setProfiles] = useState<Record<string, string>>({})
  const [note, setNote] = useState('')
  const [files, setFiles] = useState<{ open: boolean; modId: number; name: string }>({
    open: false,
    modId: 0,
    name: '',
  })
  const [keyOpen, setKeyOpen] = useState(false)
  const [waiting, setWaiting] = useState<{ modId: number; fileId: number; label: string } | null>(null)

  const game = games.find((entry) => entry.id === openId) ?? null
  const busy = Boolean(job)
  const ready = games.filter((entry) => entry.configured).length

  useEffect(() => {
    if (connection !== 'live') return
    void refreshMods()
  }, [connection])

  const say = useCallback((text: string) => setNote(text), [])
  const fail = useCallback((message: string) => setMods({ error: message }), [])

  useEffect(() => {
    if (!note) return
    const timer = window.setTimeout(() => setNote(''), 4000)
    return () => window.clearTimeout(timer)
  }, [note])

  useEffect(() => {
    if (!waiting || !game) return
    let ticks = 0
    const timer = window.setInterval(() => {
      ticks += 1
      void modsDownloads({ game: game.id, profile: '' }, waiting.modId, waiting.fileId)
        .then(async (result) => {
          if (!result.found) {
            if (ticks >= 300) {
              window.clearInterval(timer)
              setWaiting(null)
              fail(t('Nothing arrived from the browser in 15 minutes; the Nexus page is still open if you want to try again.'))
            }
            return
          }
          window.clearInterval(timer)
          setWaiting(null)
          await modsInstallArchive({ game: game.id, profile: profileOf(game, profiles) }, result.found)
        })
        .catch((error: unknown) => {
          window.clearInterval(timer)
          setWaiting(null)
          fail(errorMessage(error))
        })
    }, 3000)
    return () => window.clearInterval(timer)
  }, [waiting, game, profiles, fail, t])

  const installFile = async (file: ModFile) => {
    if (!game) return
    const target = { game: game.id, profile: profileOf(game, profiles) }
    setFiles({ open: false, modId: 0, name: '' })
    const domain = game.nexus_domain ?? ''
    if (account && !account.premium) {
      if (!domain) {
        fail(t('This game has no Nexus domain, so the file page cannot be opened.'))
        return
      }
      void openUrl(nexusFilePage(domain, files.modId, file.file_id))
      setWaiting({ modId: files.modId, fileId: file.file_id, label: file.name })
      return
    }
    try {
      await modsInstallFile(target, files.modId, file.file_id)
      say(`${t('Downloading')} ${file.name}`)
    } catch (failure) {
      fail(errorMessage(failure))
    }
  }

  const setFolder = async (target: ModsGame) => {
    const path = await pickFolder(t('Choose the game folder'))
    if (!path) return
    try {
      const result = await modsConfigure(target.id, path)
      if (result.problems?.length) fail(result.problems.join('; '))
      else say(`${target.name}: ${t('folder saved')}`)
      await refreshMods()
    } catch (failure) {
      fail(errorMessage(failure))
    }
  }

  const counts = `${ready} ${t('ready to mod')}  ·  ${games.length - ready} ${t('without a game folder')}`

  return (
    <div className={styles.page}>
      {game ? (
        <GameView
          game={game}
          profile={profileOf(game, profiles)}
          onProfile={(profile) => setProfiles((current) => ({ ...current, [game.id]: profile }))}
          onBack={() => {
            setOpenId('')
            navigate({ page: 'mods' })
          }}
          onNote={say}
          onFailed={fail}
          onOpenMod={(modId, name) => setFiles({ open: true, modId, name })}
          onApiKey={() => setKeyOpen(true)}
          onSetFolder={() => void setFolder(game)}
        />
      ) : (
        <>
          <div className={styles.head}>
            <PageHeader title={t('Mods')} subtitle={counts} />
            <div className={styles.headActions}>
              {status ? (
                <Badge accent={status.available} danger={!status.available}>
                  {status.available ? t('MODS READY') : t('BACKEND OFFLINE')}
                </Badge>
              ) : null}
              <RefreshButton busy={busy} onClick={() => void refreshMods()} />
            </div>
          </div>

          <div className={styles.body}>
            {status && !status.available ? (
              <Panel title={t('Modding backend unavailable')} subtitle={status.reason}>
                <div className={styles.setup}>
                  <p className={styles.setupText}>
                    {t(
                      'The modding backend ships inside Hikari and needs an interpreter with its Python packages. Set it up here, once.',
                    )}
                  </p>
                  <Button variant="primary" size="small" disabled={busy} onClick={() => void modsSetup()}>
                    {t('Set up')}
                  </Button>
                </div>
              </Panel>
            ) : null}

            {job ? <JobPanel /> : null}

            {waiting ? (
              <div className={styles.wait}>
                <span className={styles.waitText}>
                  {`${t(
                    'Nexus needs Premium for a direct download, so the file page is open in your browser. Press the Mod manager download button and Hikari installs it as soon as it lands.',
                  )}  ${waiting.label}`}
                </span>
                <Button size="small" onClick={() => setWaiting(null)}>
                  {t('Cancel')}
                </Button>
              </div>
            ) : null}

            {error ? (
              <Panel title={t('That did not work')} subtitle={error}>
                <div className={styles.errorRow}>
                  <Button size="small" onClick={() => setMods({ error: '' })}>
                    {t('Dismiss')}
                  </Button>
                </div>
              </Panel>
            ) : null}

            {note ? <p className={styles.note}>{note}</p> : null}

            <GameList
              onOpen={(entry) => {
                setOpenId(entry.id)
                navigate({ page: 'mods', game: entry.id })
              }}
              onNote={say}
              onFailed={fail}
            />
          </div>
        </>
      )}

      <ModFilesSheet
        open={files.open}
        game={game?.id ?? ''}
        modId={files.modId}
        modName={files.name}
        busy={busy}
        onClose={() => setFiles({ open: false, modId: 0, name: '' })}
        onInstall={(file) => void installFile(file)}
      />
      <NexusKeySheet
        open={keyOpen}
        onClose={() => setKeyOpen(false)}
        onStored={() => void refreshMods()}
        onFailed={fail}
      />
    </div>
  )
}
