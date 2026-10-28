import { useMemo, useState } from 'react'
import { gameKey, useDaemon } from '../daemon/store'
import { refreshAll } from '../daemon/api'
import type { Game } from '../daemon/types'
import { useT } from '../i18n/t'
import { Button } from '../components/Button'
import { EmptyState } from '../components/EmptyState'
import { GameGrid } from '../components/GameGrid'
import { PageHeader } from '../components/PageHeader'
import { TextInput } from '../components/TextInput'
import { Tile } from '../components/Tile'
import { navigate } from '../router'
import { AddGameSheet } from './AddGameSheet'
import styles from './Library.module.css'
import { errorMessage } from '../lib/errors'

const CHIPS: { key: string; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'epic', label: 'Epic' },
  { key: 'gog', label: 'GOG' },
  { key: 'amazon', label: 'Amazon' },
  { key: 'steam', label: 'Steam' },
  { key: 'heroic-import', label: 'From Heroic' },
  { key: 'local', label: 'Local' },
]

const SOURCES = new Set(CHIPS.map((chip) => chip.key))

function matches(game: Game, query: string, store: string, storeMode: boolean): boolean {
  if (storeMode ? game.installed === true : game.installed !== true) return false
  if (store !== 'all') {
    const source = game.source && SOURCES.has(game.source) ? game.source : game.store
    // A heroic import keeps its store, so the chip is an or on both fields.
    const hit = store === 'heroic-import' ? game.source === store : source === store
    if (!hit) return false
  }
  if (query.length > 0 && !game.title.toLowerCase().includes(query.toLowerCase())) return false
  return true
}

export function Library({ storeMode = false }: { storeMode?: boolean }) {
  const t = useT()
  const games = useDaemon((state) => state.games)
  const running = useDaemon((state) => state.running)
  const installs = useDaemon((state) => state.installs)
  const [query, setQuery] = useState('')
  const [store, setStore] = useState('all')
  const [sortBy, setSortBy] = useState<'title' | 'recent'>('title')
  const [refreshing, setRefreshing] = useState(false)
  const [note, setNote] = useState('')
  const [failed, setFailed] = useState(false)
  const [adding, setAdding] = useState(false)

  const installed = games.filter((game) => game.installed).length
  const storeCount = games.length - installed
  const counts = useMemo(() => {
    const out: Record<string, number> = {}
    for (const chip of CHIPS) {
      out[chip.key] = games.filter((game) => matches(game, '', chip.key, storeMode)).length
    }
    return out
  }, [games, storeMode])

  const shown = useMemo(() => {
    const list = games.filter((game) => matches(game, query, store, storeMode))
    return list.sort((a, b) => {
      if (sortBy === 'recent') {
        const playingA = running[gameKey(a.store, a.id)] ? 1 : 0
        const playingB = running[gameKey(b.store, b.id)] ? 1 : 0
        if (playingA !== playingB) return playingB - playingA
        const timeA = a.last_played ? Date.parse(a.last_played) : 0
        const timeB = b.last_played ? Date.parse(b.last_played) : 0
        if (timeA !== timeB) return timeB - timeA
      }
      return a.title.localeCompare(b.title)
    })
  }, [games, query, store, storeMode, sortBy, running])

  const order = useMemo(
    () => new Map(shown.map((game, index) => [gameKey(game.store, game.id), index])),
    [shown],
  )

  const openers = useMemo(() => {
    const map = new Map<string, () => void>()
    for (const game of games) {
      const key = gameKey(game.store, game.id)
      map.set(key, () => navigate({ page: 'game', store: game.store, id: game.id }))
    }
    return map
  }, [games])

  const byKey = useMemo(() => {
    const map = new Map<string, Game>()
    for (const game of games) map.set(gameKey(game.store, game.id), game)
    return map
  }, [games])

  const refresh = async () => {
    setRefreshing(true)
    setNote('')
    try {
      await refreshAll()
      setNote(t('Library refreshed'))
      setFailed(false)
    } catch (error) {
      setNote(errorMessage(error))
      setFailed(true)
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div className={styles.page}>
      <PageHeader
        title={storeMode ? t('Store') : t('Library')}
        subtitle={
          note ? (
            <span className={failed ? styles.error : styles.note}>{note}</span>
          ) : storeMode ? (
            `${storeCount} ${t('available to install')}  ·  ${installed} ${t('installed')}`
          ) : (
            `${installed} ${t('installed')}  ·  ${storeCount} ${t('in the store')}`
          )
        }
        actions={
          <>
            <TextInput
              label={t('Search games')}
              type="search"
              value={query}
              onValue={setQuery}
              placeholder={t('Search games')}
              className={styles.search}
            />
            {storeMode ? null : (
              <Button size="small" onClick={() => setAdding(true)}>
                {`+ ${t('Add game')}`}
              </Button>
            )}
            <Button size="small" onClick={() => void refresh()} busy={refreshing}>
              {refreshing ? t('Checking') : t('Refresh')}
            </Button>
          </>
        }
      />

      <div className={styles.chips}>
        {CHIPS.map((chip) =>
          chip.key === 'all' || (counts[chip.key] ?? 0) > 0 ? (
            <button
              key={chip.key}
              type="button"
              className={[styles.chip, store === chip.key ? styles.chipActive : ''].join(' ')}
              onClick={() => setStore(chip.key)}
              aria-pressed={store === chip.key}
              data-testid={`chip-${chip.key}`}
            >
              {t(chip.label)}
              <span className={styles.chipCount}>{counts[chip.key] ?? 0}</span>
            </button>
          ) : null,
        )}
        <span className={styles.spacer} />
        <span className={styles.sortLabel}>{t('Sort')}</span>
        <button
          type="button"
          className={[styles.chip, sortBy === 'title' ? styles.chipActive : ''].join(' ')}
          onClick={() => setSortBy('title')}
          aria-pressed={sortBy === 'title'}
        >
          {t('A-Z')}
        </button>
        <button
          type="button"
          className={[styles.chip, sortBy === 'recent' ? styles.chipActive : ''].join(' ')}
          onClick={() => setSortBy('recent')}
          aria-pressed={sortBy === 'recent'}
        >
          {t('Playing first')}
        </button>
      </div>

      <hr className={styles.rule} />

      <div className={styles.gridWrap}>
        {shown.length === 0 ? (
          <EmptyState
            title={storeMode ? t('Nothing available to install here.') : t('Nothing installed yet.')}
            action={
              <Button
                size="small"
                variant="quiet"
                onClick={() => navigate({ page: storeMode ? 'library' : 'store' })}
              >
                {storeMode ? t('Show installed') : t('Browse the store')}
              </Button>
            }
          />
        ) : (
          <GameGrid
            keys={shown.map((game) => gameKey(game.store, game.id))}
            render={(key, leaving) => {
              const game = byKey.get(key)
              if (!game) return null
              const open = openers.get(key)
              if (!open) return null
              return (
                <Tile
                  index={order.get(key) ?? 0}
                  game={game}
                  running={Boolean(running[key])}
                  progress={installs[key]?.percent ?? null}
                  leaving={leaving}
                  onOpen={open}
                />
              )
            }}
          />
        )}
      </div>

      <AddGameSheet open={adding} onClose={() => setAdding(false)} />
    </div>
  )
}
