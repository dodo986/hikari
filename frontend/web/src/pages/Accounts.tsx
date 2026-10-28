import { useEffect, useState } from 'react'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { PageHeader } from '../components/PageHeader'
import { Panel } from '../components/Panel'
import { TextInput } from '../components/TextInput'
import { autoLogin, authStatus, beginLogin, completeLogin, logout } from '../daemon/api'
import { refreshAuth } from '../daemon/link'
import { useDaemon } from '../daemon/store'
import { openUrl } from '../host/bridge'
import { useT } from '../i18n/t'
import styles from './Accounts.module.css'

const STORES = ['epic', 'gog', 'amazon'] as const
type Store = (typeof STORES)[number]

const STORE_NAMES: Record<Store, string> = {
  epic: 'Epic Games',
  gog: 'GOG',
  amazon: 'Amazon Games',
}

const STORE_LETTERS: Record<Store, string> = { epic: 'E', gog: 'G', amazon: 'A' }

interface Flow {
  url?: string
  hint?: string
}

export function Accounts() {
  const t = useT()
  const auth = useDaemon((state) => state.auth)
  const [flows, setFlows] = useState<Partial<Record<Store, Flow>>>({})
  const [busy, setBusy] = useState<Partial<Record<Store, boolean>>>({})
  const [codes, setCodes] = useState<Partial<Record<Store, string>>>({})
  const [note, setNote] = useState<{ text: string; error: boolean } | null>(null)

  useEffect(() => {
    void authStatus().catch(() => {})
  }, [])

  const say = (text: string, error = false) => setNote({ text, error })

  const login = async (store: Store) => {
    setNote(null)
    setBusy((current) => ({ ...current, [store]: true }))
    try {
      await autoLogin(store)
      await refreshAuth()
      say(`${t('Signed in to ')}${STORE_NAMES[store]}`)
      return
    } catch {
      // No private browser, or the automatic flow is unsupported: fall back to
      // the code flow, which always exists.
    } finally {
      setBusy((current) => ({ ...current, [store]: false }))
    }
    try {
      const started = await beginLogin(store)
      if (started.state !== 'pending') return
      setFlows((current) => ({ ...current, [store]: { url: started.url, hint: started.hint } }))
      say(t('Finish in your browser, then paste the code'))
      if (started.url) void openUrl(started.url)
    } catch (error) {
      say(error instanceof Error ? error.message : String(error), true)
    }
  }

  const finish = async (store: Store) => {
    const code = codes[store] ?? ''
    if (code.length === 0) {
      say(t('Paste the code from the browser to finish.'), true)
      return
    }
    try {
      await completeLogin(store, code)
      setFlows((current) => ({ ...current, [store]: undefined }))
      await refreshAuth()
      say(`${t('Signed in to ')}${STORE_NAMES[store]}`)
    } catch (error) {
      say(error instanceof Error ? error.message : String(error), true)
    }
  }

  const signOut = async (store: Store) => {
    try {
      await logout(store)
      await refreshAuth()
      say(`${t('Signed out of ')}${STORE_NAMES[store]}`)
    } catch (error) {
      say(error instanceof Error ? error.message : String(error), true)
    }
  }

  return (
    <div className={styles.page}>
      <PageHeader
        title={t('Accounts')}
        subtitle={t('Sign in once per store. Hikari keeps its own engine sessions, separate from Heroic.')}
      />

      {note ? (
        <p className={note.error ? styles.error : styles.note} role="status">
          {note.text}
        </p>
      ) : null}

      {STORES.map((store) => {
        const status = auth[store] ?? { logged_in: false }
        const flow = flows[store]
        return (
          <Panel
            key={store}
            title={STORE_NAMES[store]}
            action={
              status.logged_in ? (
                <Button size="small" onClick={() => void signOut(store)}>
                  {t('Log out')}
                </Button>
              ) : busy[store] ? null : flow ? null : (
                <Button size="small" variant="primary" onClick={() => void login(store)}>
                  {t('Log in')}
                </Button>
              )
            }
          >
            <div className={styles.row}>
              <span className={styles.letter} data-on={status.logged_in}>
                {STORE_LETTERS[store]}
              </span>
              <div className={styles.who}>
                <span className={styles.badges}>
                  <Badge accent={status.logged_in}>{status.logged_in ? t('CONNECTED') : t('SIGNED OUT')}</Badge>
                  {status.logged_in ? (
                    <span className={styles.user}>{status.user || status.user_id || ''}</span>
                  ) : null}
                </span>
                {busy[store] ? (
                  <span className={styles.waiting}>{t('Waiting for the login window…')}</span>
                ) : null}
              </div>
            </div>

            {flow ? (
              <div className={styles.flow}>
                <p className={styles.hint}>
                  {`${t('Finish in your browser, then paste the code')} · ${flow.hint ?? ''}`}
                </p>
                <div className={styles.flowRow}>
                  <TextInput
                    label={t('Paste the authorization code')}
                    placeholder={t('Paste the authorization code')}
                    value={codes[store] ?? ''}
                    onValue={(value) => setCodes((current) => ({ ...current, [store]: value }))}
                  />
                  <Button size="small" variant="primary" onClick={() => void finish(store)}>
                    {t('Finish')}
                  </Button>
                  <Button size="small" onClick={() => flow.url && void openUrl(flow.url)}>
                    {t('Open browser')}
                  </Button>
                  <Button
                    size="small"
                    onClick={() => {
                      setFlows((current) => ({ ...current, [store]: undefined }))
                      setNote(null)
                    }}
                  >
                    {t('Cancel')}
                  </Button>
                </div>
              </div>
            ) : null}
          </Panel>
        )
      })}

      <p className={styles.footnote}>
        {t(
          'Automatic login uses a private Chromium window; it reads only the authorization code in the final URL, never your password or cookies. Without Chromium, Hikari opens your default browser and asks for the code.',
        )}
      </p>
    </div>
  )
}
