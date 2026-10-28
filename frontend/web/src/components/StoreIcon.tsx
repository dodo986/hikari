import { STORE_MARKS } from '../lib/storeMarks'
import type { StoreId } from '../lib/storeMarks'

interface StoreIconProps {
  store: StoreId
  size?: number
}

// Each mark carries a different amount of built-in padding, so they are sized
// to look equal next to each other rather than to share one pixel size.
const OPTICAL: Record<StoreId, number> = { epic: 30, gog: 34, amazon: 30 }

export function StoreIcon({ store, size }: StoreIconProps) {
  const px = size ?? OPTICAL[store]
  return (
    <svg
      viewBox="0 0 24 24"
      width={px}
      height={px}
      fill="currentColor"
      aria-hidden="true"
      focusable="false"
    >
      <path d={STORE_MARKS[store]} />
    </svg>
  )
}
