import { PageHeader } from '../components/PageHeader'
import { Panel } from '../components/Panel'
import { useDaemon } from '../daemon/store'
import { openUrl } from '../host/bridge'
import { useT } from '../i18n/t'
import styles from './About.module.css'

const GITHUB = 'https://github.com/dodo986'

export function About() {
  const t = useT()
  const status = useDaemon((state) => state.status)
  const version = status?.version ?? '0.1.0'

  const references = [
    t('Heroic Games Launcher: store behaviour, launch pipeline and settings semantics.'),
    t('Ryoku.Ui: design tokens and components.'),
    t('OptiScaler: upscaler integration, implemented clean-room.'),
  ]

  return (
    <div className={styles.page}>
      <PageHeader title={t('About')} subtitle={t('A game launcher for Ryoku.')} />

      <Panel
        title={`Hikari ${version}  ·  GPL-3.0-or-later`}
        note={t('Epic, GOG, Amazon, Steam and your own games, run through Wine or Proton.')}
      >
        <div className={styles.maker}>
          <div className={styles.makerText}>
            <span className={styles.makerName}>{t('Made by dodo986')}</span>
            <span className={styles.makerNote}>{t('Source, issues and releases live on GitHub.')}</span>
          </div>
          <button type="button" className={styles.link} onClick={() => void openUrl(GITHUB)}>
            <span className={styles.linkLabel}>GitHub</span>
            <span className={styles.linkHandle}>@dodo986 ↗</span>
          </button>
        </div>
      </Panel>

      <Panel title={t('References')} note={t('What Hikari is built on and what it learned from.')}>
        <ul className={styles.list}>
          {references.map((line) => (
            <li key={line} className={styles.item}>
              {line}
            </li>
          ))}
        </ul>
      </Panel>
    </div>
  )
}
