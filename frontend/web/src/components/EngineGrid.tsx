import { Badge } from './Badge'
import type { EngineStatus } from '../daemon/types'
import { useT } from '../i18n/t'
import styles from './EngineGrid.module.css'

const ORDER = ['legendary', 'gogdl', 'nile', 'comet', 'umu', 'wine']

const PURPOSE: Record<string, string> = {
  legendary: 'Epic Games: store, library and installs',
  gogdl: 'GOG: store, library and installs',
  nile: 'Amazon Games: store, library and installs',
  comet: 'GOG achievements and cloud saves in game',
  umu: 'Runs Windows games through Proton',
  wine: 'Runs Windows games through Wine',
}

interface EngineGridProps {
  engines: Record<string, EngineStatus> | undefined
}

function detail(engine: EngineStatus | undefined): { text: string; mono: boolean } {
  if (!engine?.available) return { text: 'not installed', mono: false }
  const version = engine.version?.trim() ?? ''
  if (version && !version.toLowerCase().startsWith('usage:')) return { text: version, mono: true }
  if (engine.path) return { text: engine.path, mono: true }
  return { text: 'installed', mono: true }
}

export function EngineGrid({ engines }: EngineGridProps) {
  const t = useT()
  return (
    <>
      <p className={styles.mark}>{t('ENGINES')}</p>
      <div className={styles.grid}>
        {ORDER.map((name) => {
          const engine = engines?.[name]
          const info = detail(engine)
          return (
            <article className={styles.card} key={name}>
              <header className={styles.head}>
                <span className={styles.name}>{name}</span>
                <Badge accent={engine?.available} danger={!engine?.available}>
                  {engine?.available ? t('READY') : t('MISSING')}
                </Badge>
              </header>
              <p className={styles.purpose}>{t(PURPOSE[name] ?? '')}</p>
              <p
                className={styles.detail}
                data-mono={info.mono}
                data-missing={!engine?.available}
                title={info.text}
              >
                {t(info.text)}
              </p>
            </article>
          )
        })}
      </div>
    </>
  )
}
