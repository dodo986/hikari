import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import styles from './GameGrid.module.css'

interface GameGridProps {
  keys: string[]
  render: (key: string, leaving: boolean) => ReactNode
  tileWidth?: number
  gap?: number
  fallbackHeight?: number
}

interface Leaving {
  key: string
  slot: number
  node: ReactNode
}

const LEAVE_MS = 200

// A CSS grid snaps on reflow; the QML grid animates each delegate's x and y, so
// tiles live on absolute slots here and a filtered out tile holds its cell while
// it fades.
export function GameGrid({
  keys,
  render,
  tileWidth = 220,
  gap = 16,
  fallbackHeight = 361,
}: GameGridProps) {
  const host = useRef<HTMLDivElement | null>(null)
  const [width, setWidth] = useState(0)
  const [cellHeight, setCellHeight] = useState(fallbackHeight)
  const [leaving, setLeaving] = useState<Leaving[]>([])
  // The first placement is not a move: no slide from the pre-measure position.
  const [settled, setSettled] = useState(false)
  const slots = useRef(new Map<string, number>())
  const timers = useRef<number[]>([])

  useLayoutEffect(() => {
    const element = host.current
    if (!element) return
    const measure = () => setWidth(element.clientWidth)
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  useLayoutEffect(() => {
    const first = host.current?.firstElementChild
    if (first instanceof HTMLElement && first.offsetHeight > 0 && first.offsetHeight !== cellHeight) {
      setCellHeight(first.offsetHeight)
    }
  })

  useEffect(() => {
    const next = new Map<string, number>()
    keys.forEach((key, index) => next.set(key, index))
    const gone: Leaving[] = []
    for (const [key, slot] of slots.current) {
      if (!next.has(key)) gone.push({ key, slot, node: render(key, true) })
    }
    slots.current = next
    if (gone.length === 0) {
      setLeaving((current) => (current.length === 0 ? current : []))
      return
    }
    setLeaving((current) => [...current.filter((item) => !gone.some((next) => next.key === item.key)), ...gone])
    const timer = window.setTimeout(() => {
      setLeaving((current) => current.filter((item) => !gone.some((next) => next.key === item.key)))
    }, LEAVE_MS)
    timers.current.push(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [keys.join('|')])

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => window.requestAnimationFrame(() => setSettled(true)))
    return () => window.cancelAnimationFrame(frame)
  }, [])

  useEffect(() => () => timers.current.forEach((timer) => window.clearTimeout(timer)), [])

  const columns = Math.max(1, Math.floor((width + gap) / (tileWidth + gap)))
  const place = (slot: number) => ({
    transform: `translate3d(${(slot % columns) * (tileWidth + gap)}px, ${
      Math.floor(slot / columns) * (cellHeight + gap)
    }px, 0)`,
  })
  const rows = Math.ceil(keys.length / columns)
  const height = Math.max(0, rows * (cellHeight + gap) - gap)

  return (
    <div
      className={styles.grid}
      ref={host}
      style={{ height }}
      data-columns={columns}
      data-settled={settled}
    >
      {leaving.map((item) => (
        <div
          key={`leaving-${item.key}`}
          className={styles.slot}
          style={{ ...place(item.slot), width: tileWidth, opacity: 0 }}
          aria-hidden="true"
          data-leaving="true"
        >
          {item.node}
        </div>
      ))}
      {keys.map((key, index) => (
        <div key={key} className={styles.slot} style={{ ...place(index), width: tileWidth }}>
          {render(key, false)}
        </div>
      ))}
    </div>
  )
}
