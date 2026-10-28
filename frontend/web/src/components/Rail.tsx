import type { Route } from '../router'
import { href } from '../router'
import { useT } from '../i18n/t'
import type { ConnectionState } from '../daemon/types'
import styles from './Rail.module.css'

interface RailProps {
  route: Route
  games: number
  installed: number
  connection: ConnectionState
  selection: string
}

const NAV: { page: Route['page']; label: string; glyph: string }[] = [
  { page: 'library', label: 'Library', glyph: '▤' },
  { page: 'store', label: 'Store', glyph: '▦' },
  { page: 'mods', label: 'Mods', glyph: '◈' },
  { page: 'downloads', label: 'Downloads', glyph: '↓' },
  { page: 'accounts', label: 'Accounts', glyph: '◉' },
  { page: 'settings', label: 'Settings', glyph: '⚙' },
  { page: 'about', label: 'About', glyph: 'ⓘ' },
]

export function Rail({ route, games, installed, connection, selection }: RailProps) {
  const t = useT()
  const browse = NAV.slice(0, 3)
  const system = NAV.slice(3)
  const activePage = route.page === 'game' ? 'library' : route.page

  const item = (entry: (typeof NAV)[number]) => (
    <a
      key={entry.page}
      className={[styles.item, activePage === entry.page ? styles.active : '']
        .filter(Boolean)
        .join(' ')}
      href={href({ page: entry.page })}
      aria-current={activePage === entry.page ? 'page' : undefined}
    >
      <span className={styles.glyph} aria-hidden="true">
        {entry.glyph}
      </span>
      {t(entry.label)}
      {entry.page === 'library' && games > 0 ? <span className={styles.count}>{games}</span> : null}
    </a>
  )

  return (
    <nav className={styles.rail} aria-label={t('Main')}>
      <div className={styles.brand}>
        <span className={styles.brandName}>Hikari</span>
        <span className={styles.brandMark}>{t('GAME LAUNCHER')}</span>
      </div>

      <div className={styles.section}>
        <div className={styles.sectionTitle}>{t('Browse')}</div>
        {browse.map(item)}
      </div>

      <div className={styles.section}>
        <div className={styles.sectionTitle}>{t('System')}</div>
        {system.map(item)}
      </div>

      <div className={styles.spacer} />

      <div className={styles.status}>
        {connection === 'live' ? (
          <span>
            {t('YOUR LIBRARY')}: {games} {t('games')} · {installed} {t('installed')}
            {selection ? ` · ${selection}` : ''}
          </span>
        ) : (
          <span className={styles.offline}>
            {connection === 'offline' ? t('The Hikari daemon is not running') : t('Connecting to hikarid')}
          </span>
        )}
      </div>
    </nav>
  )
}
