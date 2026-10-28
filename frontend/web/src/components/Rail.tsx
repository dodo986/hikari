import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { Route } from '../router'
import { href } from '../router'
import { useT } from '../i18n/t'
import type { ConnectionState, GameStateEvent, ModsGame } from '../daemon/types'
import { UpdateNote } from './UpdateNote'
import styles from './Rail.module.css'

interface RailProps {
  route: Route
  games: number
  titles: Record<string, string>
  installed: number
  playing: GameStateEvent[]
  mods: ModsGame[]
  installs: number
  connection: ConnectionState
}

interface Entry {
  page: Route['page']
  label: string
  glyph: string
  badge: number
}

const GLYPHS: Record<string, string> = {
  library: 'video_library',
  store: 'storefront',
  mods: 'extension',
  downloads: 'download',
  accounts: 'account_circle',
  settings: 'settings',
  about: 'info',
}

function Row({
  entry,
  active,
  onMount,
}: {
  entry: Entry
  active: boolean
  onMount: (node: HTMLAnchorElement | null) => void
}) {
  return (
    <a
      ref={onMount}
      className={styles.item}
      data-active={active}
      href={href({ page: entry.page })}
      aria-current={active ? 'page' : undefined}
    >
      <span className={styles.glyph} aria-hidden="true">
        {GLYPHS[entry.page] ?? 'circle'}
      </span>
      <span className={styles.label}>{entry.label}</span>
      {entry.badge > 0 ? <span className={styles.badge}>{entry.badge}</span> : null}
    </a>
  )
}

function Fold({ label }: { label: string }) {
  return (
    <div className={styles.fold}>
      <span className={styles.foldRule} />
      <span className={styles.foldLabel}>{label.toUpperCase()}</span>
      <span className={styles.foldRule} />
    </div>
  )
}

export function Rail({
  route,
  games,
  titles,
  installed,
  playing,
  mods,
  installs,
  connection,
}: RailProps) {
  const t = useT()
  const activePage = route.page === 'game' ? '' : route.page
  const nodes = useRef(new Map<string, HTMLAnchorElement>())
  const [marker, setMarker] = useState<{ top: number; visible: boolean }>({ top: 0, visible: false })

  const browse: Entry[] = [
    { page: 'library', label: t('Library'), glyph: '', badge: installed },
    { page: 'store', label: t('Store'), glyph: '', badge: Math.max(0, games - installed) },
    { page: 'mods', label: t('Mods'), glyph: '', badge: mods.filter((game) => game.deploy_active).length },
    { page: 'downloads', label: t('Downloads'), glyph: '', badge: installs },
  ]
  const system: Entry[] = [
    { page: 'accounts', label: t('Accounts'), glyph: '', badge: 0 },
    { page: 'settings', label: t('Settings'), glyph: '', badge: 0 },
    { page: 'about', label: t('About'), glyph: '', badge: 0 },
  ]

  const measure = () => {
    const node = activePage ? nodes.current.get(activePage) : null
    const next = node
      ? { top: node.offsetTop + node.offsetHeight / 2 - 2.5, visible: true }
      : { top: marker.top, visible: false }
    setMarker((current) =>
      current.top === next.top && current.visible === next.visible ? current : next,
    )
  }

  useLayoutEffect(measure, [activePage, marker.top])
  useEffect(() => {
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  })

  const row = (entry: Entry) => (
    <Row
      key={entry.page}
      entry={entry}
      active={activePage === entry.page}
      onMount={(node) => {
        if (node) nodes.current.set(entry.page, node)
        else nodes.current.delete(entry.page)
      }}
    />
  )

  return (
    <nav className={styles.rail} aria-label={t('Main')}>
      <div className={styles.brand}>
        <span className={styles.brandName}>光 HIKARI</span>
        <span className={styles.brandMark}>{t('GAME LAUNCHER')}</span>
      </div>

      <div className={styles.nav}>
        <Fold label={t('Browse')} />
        {browse.map(row)}
        <Fold label={t('System')} />
        {system.map(row)}
        {playing.length > 0 ? <Fold label={t('Playing')} /> : null}
        {playing.map((event) => (
          <a
            key={`${event.store}/${event.id}`}
            className={styles.item}
            href={href({ page: 'game', store: event.store, id: event.id })}
          >
            <span className={styles.glyph} aria-hidden="true">
              play_arrow
            </span>
            <span className={styles.label}>{titles[`${event.store}/${event.id}`] ?? event.id}</span>
          </a>
        ))}
        <span
          className={styles.marker}
          data-visible={marker.visible}
          style={{ top: `${marker.top}px` }}
          aria-hidden="true"
        />
      </div>

      <div className={styles.spacer} />
      <UpdateNote />

      {connection === 'live' ? (
        <a className={styles.card} href={href({ page: 'library' })}>
          <span className={styles.cardMark}>{t('YOUR LIBRARY')}</span>
          <span className={styles.cardValue}>{`${games} ${t('games')}`}</span>
          <span className={styles.cardNote}>
            {`${installed} ${t('installed')}${playing.length > 0 ? `  ·  ${playing.length} ${t('playing')}` : ''}`}
          </span>
        </a>
      ) : null}
    </nav>
  )
}
