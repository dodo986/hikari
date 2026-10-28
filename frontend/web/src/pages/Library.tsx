import { useMemo, useState } from 'react'
import { gameKey, useDaemon } from '../daemon/store'
import { refreshAll } from '../daemon/api'
import type { Game } from '../daemon/types'
import { useT } from '../i18n/t'
import { Button } from '../components/Button'
import { Tile } from '../components/Tile'
import { navigate } from '../router'
import { AddGameSheet } from './AddGameSheet'
import styles from './Library.module.css'

const CHIPS: { key: string; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'epic', label: 'Epic' },
  { key: 'gog', label: 'GOG' },
  { key: 'amazon', label: 'Amazon' },
  { key: 'steam', label: 'Steam' },
  { key: 'heroic', label: 'From Heroic' },
  { key: 'local', label: 'Local' },
]

const SOURCES = new Set(CHIPS.map((chip) => chip.key))

function matches(game: Game, query: string, store: string, storeMode: boolean): boolean {
  if (storeMode ? game.installed === true : game.installed !== true) return false
  if (store !== 'all') {
    const source = game.source && SOURCES.has(game.source) ? game.source : game.store
    if (source !== store) return false
  }
  if (query.length > 0 && !game.title.toLowerCase().includes(query.toLowerCase())) return false
  return true
}

export function Library() {
  const t = useT()
  const games = useDaemon((state) => state.games)
  const running = useDaemon((state) => state.running)
  const installs = useDaemon((state) => state.installs)
  const [query, setQuery] = useState('')
  const [store, setStore] = useState('all')
  const [storeMode, setStoreMode] = useState(false)
  const [sortBy, setSortBy] = useState<'title' | 'recent'>('title')
  const [refreshing, setRefreshing] = useState(false)
  const [note, setNote] = useState('')
  const [failed, setFailed] = useState(false)
  const [adding, setAdding] = useState(false)

  const installed = games.filter((game) => game.installed).length
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

  const refresh = async () => {
    setRefreshing(true)
    setNote('')
    try {
      await refreshAll()
      setNote(t('Library refreshed'))
      setFailed(false)
    } catch (error) {
      setNote(error instanceof Error ? error.message : String(error))
      setFailed(true)
    } finally {
      setRefreshing(false)
    }
  }

  return (
    <div className={styles.page}>
      <header className={styles.head}>
        <div>
          <h1 className={styles.title}>{t('Library')}</h1>
          <p className={styles.sub}>
            {installed} {t('installed')} · {games.length - installed} {t('available to install')}
            {note ? (
              <span className={failed ? styles.error : styles.note}> · {note}</span>
            ) : null}
          </p>
        </div>
        <div className={styles.controls}>
          <input
            className={styles.search}
            type="search"
            placeholder={t('Search')}
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            aria-label={t('Search')}
          />
          <Button onClick={() => setAdding(true)}>{t('Add game')}</Button>
          <Button onClick={() => void refresh()} busy={refreshing}>
            {refreshing ? t('Checking') : t('Refresh')}
          </Button>
        </div>
      </header>

      <div className={styles.chips}>
        {CHIPS.map((chip) => (
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
        ))}
        <span className={styles.divider} />
        <button
          type="button"
          className={[styles.chip, storeMode ? '' : styles.chipActive].join(' ')}
          onClick={() => setStoreMode(false)}
          aria-pressed={!storeMode}
        >
          {t('Installed')}
        </button>
        <button
          type="button"
          className={[styles.chip, storeMode ? styles.chipActive : ''].join(' ')}
          onClick={() => setStoreMode(true)}
          aria-pressed={storeMode}
        >
          {t('Store')}
        </button>
        <span className={styles.divider} />
        <button
          type="button"
          className={styles.sort}
          onClick={() => setSortBy(sortBy === 'title' ? 'recent' : 'title')}
        >
          {sortBy === 'title' ? t('A–Z') : t('Playing first')}
        </button>
      </div>

      <div className={styles.gridWrap}>
        {shown.length === 0 ? (
          <div className={styles.empty}>
            <span>{storeMode ? t('Nothing available to install here.') : t('Nothing installed yet.')}</span>
            <Button size="small" variant="quiet" onClick={() => setStoreMode(!storeMode)}>
              {storeMode ? t('Show installed') : t('Browse the store')}
            </Button>
          </div>
        ) : (
          <div className={styles.grid}>
            {shown.map((game, index) => {
              const key = gameKey(game.store, game.id)
              return (
                <Tile
                  key={key}
                  index={index}
                  game={game}
                  running={Boolean(running[key])}
                  progress={installs[key]?.percent ?? null}
                  onOpen={() => navigate({ page: 'game', store: game.store, id: game.id })}
                />
              )
            })}
          </div>
        )}
      </div>

      <AddGameSheet open={adding} onClose={() => setAdding(false)} />
    </div>
  )
}
