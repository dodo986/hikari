import { useCallback, useEffect, useState } from 'react'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { JobPanel } from '../../components/JobPanel'
import { UpdateBanner } from '../../components/UpdateBanner'
import {
  modsDeploy,
  modsInstallArchive,
  modsInstalled,
  modsLogout,
  modsRestore,
  modsUpdate,
  modsUpdates,
} from '../../daemon/mods'
import { refreshMods } from '../../daemon/modsLink'
import { useDaemon } from '../../daemon/store'
import type { ModsGame, ModsInstalled, ModUpdate, ModsUpdates } from '../../daemon/types'
import { pickFile } from '../../host/bridge'
import { useT } from '../../i18n/t'
import { middle } from '../../lib/text'
import { BrowseTab } from './BrowseTab'
import { InstalledTab } from './InstalledTab'
import styles from './GameView.module.css'
import { errorMessage } from '../../lib/errors'

interface GameViewProps {
  game: ModsGame
  profile: string
  onProfile: (profile: string) => void
  onBack: () => void
  onNote: (message: string) => void
  onFailed: (message: string) => void
  onOpenMod: (modId: number, name: string) => void
  onApiKey: () => void
  onSetFolder: () => void
}

export function GameView({
  game,
  profile,
  onProfile,
  onBack,
  onNote,
  onFailed,
  onOpenMod,
  onApiKey,
  onSetFolder,
}: GameViewProps) {
  const t = useT()
  const job = useDaemon((state) => state.mods.job)
  const error = useDaemon((state) => state.mods.error)
  const account = useDaemon((state) => state.mods.account)
  const setMods = useDaemon((state) => state.setMods)
  const [tab, setTab] = useState<'installed' | 'browse'>('installed')
  const [installed, setInstalled] = useState<ModsInstalled | null>(null)
  const [updates, setUpdates] = useState<ModsUpdates | null>(null)
  const busy = Boolean(job)
  const deployLabel = game.deploy_active ? t('Deploy again') : t('Deploy')

  const loadInstalled = useCallback(() => {
    void modsInstalled({ game: game.id, profile })
      .then(setInstalled)
      .catch((failure: Error) => {
        setInstalled(null)
        onFailed(failure.message)
      })
  }, [game.id, profile, onFailed])

  useEffect(() => {
    loadInstalled()
    if (!game.configured) {
      setUpdates(null)
      return
    }
    void modsUpdates({ game: game.id, profile })
      .then(setUpdates)
      .catch(() => setUpdates(null))
  }, [game.id, profile, game.configured, loadInstalled])

  const deploy = async () => {
    try {
      await modsDeploy({ game: game.id, profile })
      onNote(t('Deploying'))
    } catch (failure) {
      onFailed(errorMessage(failure))
    }
  }

  const archive = async () => {
    const path = await pickFile(t('Choose a mod archive'))
    if (!path) return
    try {
      await modsInstallArchive({ game: game.id, profile }, path)
      onNote(t('Installing'))
    } catch (failure) {
      onFailed(errorMessage(failure))
    }
  }

  const update = async (row: ModUpdate) => {
    try {
      await modsUpdate({ game: game.id, profile }, row.mod)
      onNote(`${t('Updating')} ${row.mod}`)
    } catch (failure) {
      onFailed(errorMessage(failure))
    }
  }

  return (
    <div className={styles.view}>
      <header className={styles.bar}>
        <Button size="small" onClick={onBack}>
          {`← ${t('Mods')}`}
        </Button>
        <span className={styles.crumb}>{middle(`MODS  /  ${game.name.toUpperCase()}`, 44)}</span>
        <span className={styles.spacer} />
        <Badge accent={game.deploy_active} muted={!game.deploy_active}>
          {profile || t('NO PROFILE')}
        </Badge>
        {game.deploy_active ? <Badge accent>{t('DEPLOYED')}</Badge> : null}
        {game.configured ? null : (
          <Button size="small" onClick={onSetFolder}>
            {t('Set folder')}
          </Button>
        )}
        <Button size="small" disabled={busy || !game.configured} onClick={() => void archive()}>
          {t('Install archive')}
        </Button>
        {game.deploy_active ? (
          <Button
            size="small"
            disabled={busy}
            onClick={() =>
              void modsRestore(game.id).catch((failure: Error) => onFailed(failure.message))
            }
          >
            {t('Restore')}
          </Button>
        ) : null}
        <Button
          size="small"
          variant={game.deploy_active ? 'quiet' : 'primary'}
          disabled={busy || !game.configured}
          onClick={() => void deploy()}
        >
          {deployLabel}
        </Button>
      </header>

      <div className={styles.body}>
        {updates?.updates.length ? (
          <UpdateBanner updates={updates.updates} disabled={busy} onUpdate={(row) => void update(row)} />
        ) : null}

        <div className={styles.tabs}>
          <Button
            size="small"
            variant={tab === 'installed' ? 'primary' : 'quiet'}
            onClick={() => setTab('installed')}
          >
            {`${t('Installed')}  ${installed?.mods.length ?? 0}`}
          </Button>
          <Button
            size="small"
            variant={tab === 'browse' ? 'primary' : 'quiet'}
            onClick={() => setTab('browse')}
          >
            {t('Browse Nexus')}
          </Button>
          <span className={styles.spacer} />
          {account?.stored ? (
            <>
              <span className={styles.account}>
                {account.user ? `${t('signed in as')} ${account.user}` : t('signed in')}
                {account.premium ? `  ·  ${t('premium')}` : ''}
              </span>
              <Button
                size="small"
                onClick={() => {
                  void modsLogout().then(() => {
                    setMods({ account: { stored: false, premium: false } })
                    refreshMods()
                  })
                }}
              >
                {t('Sign out')}
              </Button>
            </>
          ) : null}
          <span className={styles.account}>
            {game.nexus_domain ? `nexusmods.com/${game.nexus_domain}` : t('no Nexus domain for this game')}
          </span>
        </div>

        {job ? <JobPanel /> : null}
        {error ? (
          <div className={styles.error} role="alert">
            <span className={styles.errorText}>{error}</span>
            <Button size="small" onClick={() => setMods({ error: '' })}>
              {t('Dismiss')}
            </Button>
          </div>
        ) : null}

        {tab === 'installed' ? (
          <InstalledTab
            game={game}
            target={{ game: game.id, profile }}
            installed={installed}
            onInstalled={setInstalled}
            onUpdates={setUpdates}
            onFailed={onFailed}
            onNote={onNote}
            busy={busy}
            onChangeProfile={() => {
              const index = game.profiles.indexOf(profile)
              const next = game.profiles[(index + 1) % game.profiles.length] ?? ''
              onProfile(next)
            }}
          />
        ) : (
          <BrowseTab
            game={game}
            onOpenMod={(mod) => onOpenMod(mod.mod_id, mod.name)}
            onApiKey={onApiKey}
            onFailed={onFailed}
          />
        )}
      </div>
    </div>
  )
}
