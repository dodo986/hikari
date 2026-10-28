import { useEffect, useState } from 'react'
import { Badge } from './Badge'
import { Button } from './Button'
import { Panel } from './Panel'
import { TextInput } from './TextInput'
import { deleteSecret, secretStatus, setSecret } from '../daemon/api'
import { useT } from '../i18n/t'
import styles from './SecretPanel.module.css'

const NAME = 'steamgriddb'

/** The key is write-only: the daemon reports whether one is stored, never its value. */
export function SecretPanel() {
  const t = useT()
  const [set, setSet] = useState(false)
  const [backend, setBackend] = useState('')
  const [key, setKey] = useState('')
  const [note, setNote] = useState('')

  const load = () => {
    void secretStatus()
      .then((result) => {
        const entry = result.secrets?.find((item) => item.name === NAME)
        setSet(Boolean(entry?.set))
        setBackend(entry?.backend ?? '')
      })
      .catch(() => setSet(false))
  }

  useEffect(load, [])

  const save = async () => {
    if (key.trim() === '') return
    try {
      await setSecret(NAME, key.trim())
      setKey('')
      setNote(t('Key stored in the system keyring'))
      load()
    } catch (error) {
      setNote(error instanceof Error ? error.message : String(error))
    }
  }

  const forget = async () => {
    try {
      await deleteSecret(NAME)
      setNote(t('Key removed'))
      load()
    } catch (error) {
      setNote(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <Panel
      title={t('Artwork')}
      subtitle={t('A SteamGridDB key lets Hikari find covers for games the stores do not ship artwork for. It is stored in the system keyring and never sent back to this window.')}
      action={<Badge accent={set}>{set ? t('KEY SET') : t('NO KEY')}</Badge>}
    >
      <div className={styles.row}>
        <TextInput
          label={t('SteamGridDB key')}
          value={key}
          onValue={setKey}
          type="password"
          placeholder={set ? t('replace the stored key') : t('paste your key')}
          className={styles.field}
        />
        <Button size="small" onClick={() => void save()} disabled={key.trim() === ''}>
          {t('Save key')}
        </Button>
        {set ? (
          <Button size="small" onClick={() => void forget()}>
            {t('Forget')}
          </Button>
        ) : null}
      </div>
      {note ? <p className={styles.note}>{note}</p> : null}
      {set && backend ? <p className={styles.note}>{`${t('stored in')} ${backend}`}</p> : null}
    </Panel>
  )
}
