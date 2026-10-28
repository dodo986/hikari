import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { gameSettings, installInfo, launchOptions, loadGame, optiStatus, runLog } from '../../daemon/api'
import type { Key } from '../../daemon/api'
import { rpc } from '../../daemon/link'
import { useDaemon } from '../../daemon/store'
import type { Game, GameSettings, InstallInfo, LaunchOption, OptiStatus, RunLog } from '../../daemon/types'

export interface GameDetailData {
  game: Game | null
  settings: GameSettings | null
  options: LaunchOption[]
  install: InstallInfo | null
  opti: OptiStatus | null
  log: RunLog | null
  crashed: boolean
  reload: () => void
  reloadLog: () => Promise<RunLog | null>
  reloadOpti: () => void
  setGame: (game: Game) => void
  setSettings: (settings: GameSettings) => void
}

export function useGameDetail(key: Key): GameDetailData {
  const [game, setGame] = useState<Game | null>(null)
  const [settings, setSettings] = useState<GameSettings | null>(null)
  const [options, setOptions] = useState<LaunchOption[]>([])
  const [install, setInstall] = useState<InstallInfo | null>(null)
  const [opti, setOpti] = useState<OptiStatus | null>(null)
  const [log, setLog] = useState<RunLog | null>(null)
  const [crashed, setCrashed] = useState(false)
  const [tick, setTick] = useState(0)
  const askedInfo = useRef(false)
  const cached = useDaemon((state) => state.games.find((g) => g.store === key.store && g.id === key.id))

  const reload = useCallback(() => setTick((value) => value + 1), [])

  useEffect(() => {
    let live = true
    setCrashed(false)
    askedInfo.current = false
    void loadGame(key)
      .then((loaded) => {
        if (live) setGame(loaded)
      })
      .catch(() => {})
    void gameSettings(key)
      .then((loaded) => {
        if (live) setSettings(loaded)
      })
      .catch(() => {})
    void launchOptions(key)
      .then((result) => {
        if (live) setOptions(result.options ?? [])
      })
      .catch(() => setOptions([]))
    void optiStatus(key)
      .then((status) => {
        if (live) setOpti(status)
      })
      .catch(() => setOpti(null))
    return () => {
      live = false
    }
  }, [key, tick])

  useEffect(() => {
    if (!cached) return
    setGame((current) => ({ ...current, ...cached }))
  }, [cached])

  const resolved = game ?? cached ?? null
  const installed = resolved?.installed ?? false

  useEffect(() => {
    if (!installed || install || askedInfo.current) return
    askedInfo.current = true
    void installInfo(key)
      .then(setInstall)
      .catch(() => setInstall(null))
  }, [installed, install, key])

  useEffect(() => {
    const off = rpc.on('game.state', (params) => {
      const event = params as { store?: string; id?: string; state?: string }
      if (event.store !== key.store || event.id !== key.id) return
      if (event.state === 'crashed') {
        setCrashed(true)
        void runLog(key).then(setLog).catch(() => {})
      }
    })
    return off
  }, [key])

  const reloadLog = useCallback(async () => {
    try {
      const record = await runLog(key)
      setLog(record)
      return record
    } catch {
      return null
    }
  }, [key])

  const reloadOpti = useCallback(() => {
    void optiStatus(key)
      .then(setOpti)
      .catch(() => {})
  }, [key])

  return useMemo(
    () => ({
      game: resolved,
      settings,
      options,
      install,
      opti,
      log,
      crashed,
      reload,
      reloadLog,
      reloadOpti,
      setGame,
      setSettings,
    }),
    [resolved, settings, options, install, opti, log, crashed, reload, reloadLog, reloadOpti],
  )
}
