import { useCallback, useEffect, useState } from 'react'
import { Badge } from '../../components/Badge'
import { Button } from '../../components/Button'
import { Panel } from '../../components/Panel'
import { TextInput } from '../../components/TextInput'
import { modsBrowse, modsLogin } from '../../daemon/mods'
import { useDaemon } from '../../daemon/store'
import type { Mod, ModsBrowse, ModsGame } from '../../daemon/types'
import { useT } from '../../i18n/t'
import { compact, day } from '../../lib/format'
import styles from './BrowseTab.module.css'
import { errorMessage } from '../../lib/errors'

interface BrowseTabProps {
  game: ModsGame
  onOpenMod: (mod: Mod) => void
  onApiKey: () => void
  onFailed: (message: string) => void
}

const SORTS: { key: string; label: string }[] = [
  { key: 'created', label: 'New' },
  { key: 'updated', label: 'Updated' },
  { key: 'downloads', label: 'Popular' },
  { key: 'endorsements', label: 'Endorsed' },
]

export function BrowseTab({ game, onOpenMod, onApiKey, onFailed }: BrowseTabProps) {
  const t = useT()
  const account = useDaemon((state) => state.mods.account)
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState('downloads')
  const [result, setResult] = useState<ModsBrowse | null>(null)
  const [searching, setSearching] = useState(false)

  // Query and sort travel as arguments; reading them from state made every
  // keystroke a new callback and fired a request per character.
  const search = useCallback(
    async (page: number, replace: boolean, terms: string, order: string) => {
      setSearching(true)
      try {
        const reply = await modsBrowse(game.id, terms, order, page)
        setResult((current) =>
          replace || !current ? reply : { ...reply, results: [...current.results, ...reply.results] },
        )
        if (reply.error) onFailed(reply.error)
      } catch (error) {
        onFailed(errorMessage(error))
      } finally {
        setSearching(false)
      }
    },
    [game.id, onFailed],
  )

  useEffect(() => {
    setQuery('')
    setSort('downloads')
    setResult(null)
  }, [game.id])

  useEffect(() => {
    if (!account?.stored) return
    void search(0, true, '', 'downloads')
  }, [account?.stored, search])

  if (!account) return null

  if (!account.stored) {
    return (
      <Panel title={t('Sign in to Nexus')} note={t('Your browser opens Nexus; approving there is the whole login.')}>
        <div className={styles.signIn}>
          <span className={styles.spacer} />
          <Button size="small" onClick={onApiKey}>
            {t('Use an API key')}
          </Button>
          <Button size="small" variant="primary" onClick={() => void modsLogin()}>
            {t('Sign in')}
          </Button>
        </div>
      </Panel>
    )
  }

  return (
    <div className={styles.pane}>
      <div className={styles.searchRow}>
        <TextInput
          label={t('Search Nexus')}
          value={query}
          onValue={setQuery}
          placeholder={t('Search Nexus, or leave empty for the most downloaded')}
          className={styles.field}
          onKeyDown={(event) => {
            if (event.key === 'Enter') void search(0, true, query, sort)
          }}
        />
        <Button size="small" busy={searching} onClick={() => void search(0, true, query, sort)}>
          {t('Search')}
        </Button>
      </div>

      <div className={styles.sortRow}>
        {SORTS.map((entry) => (
          <button
            key={entry.key}
            type="button"
            className={styles.chip}
            data-on={sort === entry.key}
            onClick={() => {
              setSort(entry.key)
              void search(0, true, query, entry.key)
            }}
          >
            {t(entry.label)}
          </button>
        ))}
        <span className={styles.spacer} />
        {result ? <span className={styles.count}>{`${result.results.length} ${t('mods')}`}</span> : null}
      </div>

      <div className={styles.grid}>
        {(result?.results ?? []).map((mod) => (
          <button type="button" className={styles.card} key={mod.mod_id} onClick={() => onOpenMod(mod)}>
            <span className={styles.cover}>
              <span className={styles.letter}>{mod.name.slice(0, 1)}</span>
              {mod.picture_url ? (
                <img className={styles.picture} src={mod.picture_url} alt="" loading="lazy" />
              ) : null}
              {mod.adult ? (
                <span className={styles.adult}>
                  <Badge danger>{t('ADULT')}</Badge>
                </span>
              ) : null}
            </span>
            <span className={styles.text}>
              <span className={styles.name}>{mod.name}</span>
              <span className={styles.meta}>
                {[mod.author || t('unknown author'), mod.category].filter(Boolean).join('  ·  ')}
              </span>
              <span className={styles.meta}>
                {mod.updated ? `${t('updated')} ${day(mod.updated)}` : mod.created ? `${t('added')} ${day(mod.created)}` : ''}
              </span>
              <span className={styles.summary}>{mod.summary}</span>
              <span className={styles.stats}>
                {[
                  mod.version,
                  `${compact(mod.downloads)} ${t('downloads')}`,
                  `${compact(mod.endorsements)} ${t('endorsements')}`,
                ]
                  .filter(Boolean)
                  .join('  ·  ')}
              </span>
            </span>
          </button>
        ))}
      </div>

      {result?.more && result.results.length > 0 ? (
        <div className={styles.more}>
          <Button
            size="small"
            disabled={searching}
            onClick={() => void search(result.page + 1, false, query, sort)}
          >
            {t('Load more')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
