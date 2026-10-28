import { useEffect, useRef } from 'react'
import type { ReactNode } from 'react'
import { Button } from './Button'
import styles from './Sheet.module.css'

interface SheetProps {
  open: boolean
  title: string
  closeLabel: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}

// The native dialog gives focus trapping and escape for free.
export function Sheet({ open, title, closeLabel, onClose, children, footer }: SheetProps) {
  const ref = useRef<HTMLDialogElement>(null)
  const pressed = useRef<Element | null>(null)

  useEffect(() => {
    const dialog = ref.current
    if (!dialog) return
    if (open && !dialog.open) dialog.showModal()
    if (!open && dialog.open) dialog.close()
  }, [open])

  return (
    <dialog
      ref={ref}
      className={styles.dialog}
      onClose={onClose}
      onClick={(event) => {
        // A drag that ends on the backdrop keeps its selection instead of closing.
        if (event.target === ref.current && pressed.current === ref.current) onClose()
      }}
      onMouseDown={(event) => {
        pressed.current = event.target as Element
      }}
      aria-label={title}
    >
      <div className={styles.head}>
        <h2 className={styles.title}>{title}</h2>
        <Button variant="quiet" size="small" onClick={onClose}>
          {closeLabel}
        </Button>
      </div>
      <div className={styles.body}>{children}</div>
      {footer ? <div className={styles.footer}>{footer}</div> : null}
    </dialog>
  )
}
