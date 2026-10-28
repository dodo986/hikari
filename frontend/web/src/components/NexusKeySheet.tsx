import { useState } from 'react'
import { Button } from './Button'
import { Sheet } from './Sheet'
import { TextInput } from './TextInput'
import { modsNexus } from '../daemon/mods'
import { refreshModsAccount } from '../daemon/modsLink'
import { useT } from '../i18n/t'
import styles from './ModFilesSheet.module.css'

interface NexusKeySheetProps {
  open: boolean
  onClose: () => void
  onStored: () => void
  onFailed: (message: string) => void
}

export function NexusKeySheet({ open, onClose, onStored, onFailed }: NexusKeySheetProps) {
  const t = useT()
  const [key, setKey] = useState('')

  const save = async () => {
    try {
      await modsNexus(key.trim())
      setKey('')
      onClose()
      await refreshModsAccount()
      onStored()
    } catch (error) {
      onFailed(error instanceof Error ? error.message : String(error))
    }
  }

  return (
    <Sheet
      open={open}
      title={t('Nexus API key')}
      closeLabel={t('Close')}
      onClose={onClose}
      footer={
        <>
          <Button size="small" onClick={onClose}>
            {t('Cancel')}
          </Button>
          <Button size="small" variant="primary" disabled={key.trim().length < 9} onClick={() => void save()}>
            {t('Save')}
          </Button>
        </>
      }
    >
      <p className={styles.subtitle}>
        {t('nexusmods.com, Account, API. Hikari keeps the key in its own data directory.')}
      </p>
      <TextInput
        label={t('Nexus API key')}
        type="password"
        value={key}
        onValue={setKey}
        placeholder={t('paste the key')}
      />
    </Sheet>
  )
}
