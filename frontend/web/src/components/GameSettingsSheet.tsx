import { useEffect, useState } from 'react'
import { Button } from './Button'
import { Sheet } from './Sheet'
import { SettingsToggle } from './SettingsToggle'
import { TextInput } from './TextInput'
import {
  installWine,
  launchOptions,
  removeLocal,
  repairGame,
  saveGameSettings,
  syncSaves,
  wineReleases,
  wineVersions,
} from '../daemon/detail'
import type { Game, GameSettings, LaunchOption, WineVersion } from '../daemon/types'
import { pickFile } from '../host/bridge'
import { useT } from '../i18n/t'
import styles from './GameSettingsSheet.module.css'

interface GameSettingsSheetProps {
  open: boolean
  game: Game
  settings: GameSettings | null
  options: LaunchOption[]
  onClose: () => void
  onSaved: (settings: GameSettings, note: string) => void
  onNote: (note: string) => void
  onFailed: (message: string) => void
  onRemoved: () => void
  onLaunchOption: (option: LaunchOption | null) => void
}

const isProton = (version: WineVersion | undefined) => (version?.type ?? '') === 'proton'

export function GameSettingsSheet({
  open,
  game,
  settings,
  options,
  onClose,
  onSaved,
  onNote,
  onFailed,
  onRemoved,
  onLaunchOption,
}: GameSettingsSheetProps) {
  const t = useT()
  const [draft, setDraft] = useState<GameSettings | null>(settings)
  const [exe, setExe] = useState(settings?.target_exe ?? '')
  const [prefix, setPrefix] = useState(settings?.wine_prefix ?? '')
  const [saves, setSaves] = useState(settings?.saves_path ?? '')
  const [versions, setVersions] = useState<WineVersion[]>([])
  const [picking, setPicking] = useState(false)
  const [repairing, setRepairing] = useState(false)
  const [index, setIndex] = useState(-1)

  useEffect(() => {
    if (!open) return
    setDraft(settings)
    setExe(settings?.target_exe ?? '')
    setPrefix(settings?.wine_prefix ?? '')
    setSaves(settings?.saves_path ?? '')
    void wineVersions()
      .then((result) => setVersions(result.versions ?? []))
      .catch(() => setVersions([]))
    void launchOptions({ store: game.store, id: game.id })
      .then((result) => setIndex((result.options ?? []).findIndex((item) => item.name === options[0]?.name)))
      .catch(() => {})
  }, [open, settings, game.store, game.id, options])

  const save = async (patch: Partial<GameSettings>, note: string) => {
    if (!draft) {
      onNote(t('Settings are still loading'))
      return
    }
    const next: GameSettings = {
      ...draft,
      ...patch,
      target_exe: exe.trim(),
      wine_prefix: prefix.trim(),
      saves_path: saves.trim(),
    }
    try {
      await saveGameSettings({ store: game.store, id: game.id }, next)
      setDraft(next)
      onSaved(next, note)
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    }
  }

  const proton = isProton(draft?.wine_version)
  const plainWine = !proton

  const chooseExe = async () => {
    const path = await pickFile(t('Choose the game executable'))
    if (!path) return
    setExe(path)
    void save({ target_exe: path }, t('Game executable saved'))
  }

  const latestProton = async () => {
    try {
      const releases = await wineReleases()
      const tag = releases.releases?.[0]?.tag
      if (!tag) {
        onNote(t('no releases available'))
        return
      }
      await installWine(tag)
      onNote(`${t('Downloading')} ${tag}`)
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    }
  }

  const layer = draft?.wine_version?.name
    ? `${draft.wine_version.name}  ·  ${draft.wine_version.type || 'wine'}`
    : t('system wine')

  return (
    <Sheet
      open={open}
      title={t('Game settings')}
      closeLabel={t('Close')}
      onClose={onClose}
      footer={
        <>
          <Button size="small" onClick={() => void save({}, t('Settings saved'))}>
            {t('Save paths and executable')}
          </Button>
          {game.installed && game.store !== 'local' ? (
            <Button
              size="small"
              busy={repairing}
              onClick={async () => {
                setRepairing(true)
                try {
                  await repairGame({ store: game.store, id: game.id })
                  onNote(t('Verifying the install'))
                } catch (error) {
                  setRepairing(false)
                  onFailed(error instanceof Error ? error.message : String(error))
                }
              }}
            >
              {repairing ? t('Repairing…') : t('Verify & repair')}
            </Button>
          ) : null}
          {game.store === 'local' ? (
            <Button
              size="small"
              variant="danger"
              onClick={async () => {
                await removeLocal(game.id)
                onRemoved()
              }}
            >
              {t('Remove from library')}
            </Button>
          ) : null}
        </>
      }
    >
      <div className={styles.stack}>
        <p className={styles.subtitle}>
          {t('Changes apply immediately. The prefix and save path apply when you save.')}
        </p>

        <p className={styles.mark}>{t('LAUNCH OPTIONS')}</p>
        {options.length === 0 ? (
          <p className={styles.note}>{t('This game reports no extra launch options.')}</p>
        ) : (
          <div className={styles.pills}>
            {options.map((option, position) => (
              <button
                key={option.name}
                type="button"
                className={styles.pill}
                data-on={position === index}
                onClick={() => {
                  setIndex(position)
                  onLaunchOption(option)
                }}
              >
                {option.name}
              </button>
            ))}
          </div>
        )}

        <hr className={styles.rule} />
        <p className={styles.mark}>{t('GAME')}</p>
        <div className={styles.row}>
          <span className={styles.label}>{t('Executable')}</span>
          <TextInput
            label={t('Executable')}
            value={exe}
            onValue={setExe}
            placeholder={t("the game's own executable")}
            className={styles.field}
          />
          <Button size="small" onClick={() => void chooseExe()}>
            {t('Choose…')}
          </Button>
          {exe ? (
            <Button
              size="small"
              onClick={() => {
                setExe('')
                void save({ target_exe: '' }, t("Back to the game's own executable"))
              }}
            >
              {t("Use game's")}
            </Button>
          ) : null}
        </div>
        {game.executable ? (
          <p className={styles.helper}>{`${t("The game's own:")} ${game.executable}`}</p>
        ) : null}
        <p className={styles.helper}>
          {t(
            'Change this when the launcher starts the wrong file: a pre-launcher that closes on its own, or the executable a mod loader needs.',
          )}
        </p>

        <hr className={styles.rule} />
        <p className={styles.mark}>{t('COMPATIBILITY')}</p>
        <p className={styles.layer}>{layer}</p>
        <div className={styles.row}>
          <Button size="small" onClick={() => setPicking((value) => !value)}>
            {picking ? t('Hide versions') : t('Change version')}
          </Button>
          <Button size="small" onClick={() => void latestProton()}>
            {t('Get latest Proton')}
          </Button>
        </div>
        {picking ? (
          <div className={styles.pills}>
            {versions.map((version) => (
              <button
                key={version.bin}
                type="button"
                className={styles.pill}
                data-on={version.bin === draft?.wine_version?.bin}
                onClick={() => {
                  void save(
                    {
                      wine_version: {
                        bin: version.bin,
                        name: version.name,
                        type: version.type,
                        source: version.source,
                      },
                    },
                    t('Compatibility layer saved'),
                  )
                  setPicking(false)
                }}
              >
                {`${version.name} (${version.type})`}
              </button>
            ))}
          </div>
        ) : null}
        <div className={styles.row}>
          <span className={styles.label}>{t('Wine prefix')}</span>
          <TextInput
            label={t('Wine prefix')}
            value={prefix}
            onValue={setPrefix}
            placeholder={t('one prefix per game')}
            className={styles.field}
          />
        </div>

        <hr className={styles.rule} />
        <p className={styles.mark}>{t('SAVES')}</p>
        <div className={styles.row}>
          <span className={styles.label}>{t('Save path')}</span>
          <TextInput
            label={t('Save path')}
            value={saves}
            onValue={setSaves}
            placeholder={t("the game's save directory")}
            className={styles.field}
          />
          {game.store === 'epic' ? (
            <>
              <Button
                size="small"
                onClick={() =>
                  void syncSaves({ store: game.store, id: game.id }, 'download').then(
                    () => onNote(t('Saves downloaded')),
                    (error: Error) => onFailed(error.message),
                  )
                }
              >
                {t('Download')}
              </Button>
              <Button
                size="small"
                onClick={() =>
                  void syncSaves({ store: game.store, id: game.id }, 'upload').then(
                    () => onNote(t('Saves uploaded')),
                    (error: Error) => onFailed(error.message),
                  )
                }
              >
                {t('Upload')}
              </Button>
            </>
          ) : null}
        </div>

        <hr className={styles.rule} />
        <p className={styles.mark}>{t('LAUNCH EXTRAS')}</p>
        <SettingsToggle
          name={t('MangoHud')}
          hint={t('FPS and frame-time overlay')}
          checked={draft?.show_mangohud ?? false}
          onChange={(value) => void save({ show_mangohud: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('GameMode')}
          hint={t('CPU governor and scheduling for the game process')}
          checked={draft?.use_gamemode ?? false}
          onChange={(value) => void save({ use_gamemode: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('FSR (Proton)')}
          hint={t('Full-screen FSR inside Proton')}
          checked={draft?.enable_fsr ?? false}
          available={proton}
          onChange={(value) => void save({ enable_fsr: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('Offline mode')}
          hint={t('Launch without store checks when the game allows it')}
          checked={draft?.offline_mode ?? false}
          onChange={(value) => void save({ offline_mode: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('Disable umu')}
          hint={t('Use the Proton build directly instead of umu-run')}
          checked={draft?.disable_umu ?? false}
          onChange={(value) => void save({ disable_umu: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('DXVK')}
          hint={plainWine ? t('Newest DXVK in the game prefix') : t('Plain Wine only')}
          checked={draft?.auto_install_dxvk ?? false}
          available={plainWine}
          onChange={(value) => void save({ auto_install_dxvk: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('VKD3D-Proton')}
          hint={plainWine ? t('Direct3D 12 translation for plain Wine') : t('Plain Wine only')}
          checked={draft?.auto_install_vkd3d ?? false}
          available={plainWine}
          onChange={(value) => void save({ auto_install_vkd3d: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('DXVK-NVAPI')}
          hint={plainWine ? t('DLSS hooks for plain Wine') : t('Plain Wine only')}
          checked={draft?.auto_install_dxvk_nvapi ?? false}
          available={plainWine}
          onChange={(value) => void save({ auto_install_dxvk_nvapi: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('EAC runtime')}
          hint={proton ? t('Easy Anti-Cheat runtime for Proton') : t('Proton only')}
          checked={draft?.eac_runtime ?? false}
          available={proton}
          onChange={(value) => void save({ eac_runtime: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('BattlEye runtime')}
          hint={proton ? t('BattlEye runtime for Proton') : t('Proton only')}
          checked={draft?.battleye_runtime ?? false}
          available={proton}
          onChange={(value) => void save({ battleye_runtime: value }, t('Saved'))}
        />
        <SettingsToggle
          name={t('Epic EOS overlay')}
          hint={game.store === 'epic' ? t('Hooks the game; off by default') : t('Epic games only')}
          checked={draft?.enable_eos_overlay ?? false}
          available={game.store === 'epic'}
          onChange={(value) => void save({ enable_eos_overlay: value }, t('Saved'))}
        />
      </div>
    </Sheet>
  )
}
