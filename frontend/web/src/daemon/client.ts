import type { ConnectionState, EndpointConfig } from './types'

type Pending = {
  resolve: (value: unknown) => void
  reject: (error: Error) => void
  timer: number
}

export interface RpcError extends Error {
  code: number
}

type EventHandler = (params: unknown) => void
type StateHandler = (state: ConnectionState) => void

const REQUEST_TIMEOUT_MS = 30_000
const BACKOFF_START_MS = 250
const BACKOFF_MAX_MS = 5_000

// One request per id, events on the same connection, reconnect with backoff.
export class RpcClient {
  private socket: WebSocket | null = null
  private nextId = 1
  private pending = new Map<number, Pending>()
  private handlers = new Map<string, Set<EventHandler>>()
  private topics = new Set<string>()
  private state: ConnectionState = 'connecting'
  private backoff = BACKOFF_START_MS
  private reconnectTimer = 0
  private started = false
  private attempts = 0
  private refused = 0

  constructor(
    private readonly open: () => Promise<EndpointConfig>,
    private readonly onState: StateHandler = () => {},
  ) {}

  get connection(): ConnectionState {
    return this.state
  }

  start(): void {
    if (this.started) return
    this.started = true
    void this.connect()
  }

  on(event: string, handler: EventHandler): () => void {
    const set = this.handlers.get(event) ?? new Set<EventHandler>()
    set.add(handler)
    this.handlers.set(event, set)
    return () => set.delete(handler)
  }

  call<T>(method: string, params: unknown = {}): Promise<T> {
    const socket = this.socket
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error('hikarid is not connected'))
    }
    const id = this.nextId++
    const frame = JSON.stringify({ jsonrpc: '2.0', id, method, params })
    return new Promise<T>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        this.pending.delete(id)
        reject(new Error(`${method} timed out`))
      }, REQUEST_TIMEOUT_MS)
      this.pending.set(id, {
        resolve: resolve as (value: unknown) => void,
        reject,
        timer,
      })
      socket.send(frame)
    })
  }

  private async connect(): Promise<void> {
    this.started = true
    let endpoint: EndpointConfig
    try {
      endpoint = await this.open()
    } catch {
      this.attempts++
      this.setState('offline')
      this.scheduleReconnect()
      return
    }
    let socket: WebSocket
    try {
      socket = new WebSocket(`${endpoint.url}?token=${encodeURIComponent(endpoint.token)}`)
    } catch {
      // A malformed endpoint must land in the same retry path as a missing one.
      this.attempts++
      this.setState('offline')
      this.scheduleReconnect()
      return
    }
    this.socket = socket
    let opened = false
    // A superseded socket must not report state or serve calls.
    socket.onmessage = (message: MessageEvent<string>) => {
      if (this.socket === socket) this.receive(message.data)
    }
    // A refused handshake reports here first; onclose decides what happens next.
    socket.onerror = () => {}
    socket.onclose = () => {
      if (this.socket !== socket) return
      this.failAll(new Error('hikarid closed the connection'))
      this.attempts++
      // A socket that never opened means the upgrade was refused, which is a
      // stale token rather than a stopped daemon.
      if (!opened) this.refused++
      if (this.refused >= 2) this.setState('unauthorized')
      else this.setState(this.attempts >= 2 ? 'offline' : 'connecting')
      this.scheduleReconnect()
    }
    socket.onopen = () => {
      if (this.socket !== socket) {
        socket.close()
        return
      }
      opened = true
      this.attempts = 0
      this.refused = 0
      this.backoff = BACKOFF_START_MS
      this.setState('live')
      if (this.topics.size > 0) {
        void this.call('events.subscribe', { topics: [...this.topics] }).catch(() => {})
      }
    }
  }

  private scheduleReconnect(): void {
    window.clearTimeout(this.reconnectTimer)
    const delay = this.backoff
    this.backoff = Math.min(BACKOFF_MAX_MS, Math.round(this.backoff * 1.7))
    this.reconnectTimer = window.setTimeout(() => {
      this.socket = null
      void this.connect()
    }, delay)
  }

  private setState(state: ConnectionState): void {
    if (this.state === state) return
    this.state = state
    this.onState(state)
  }

  private receive(raw: string): void {
    let frame: {
      id?: number
      method?: string
      result?: unknown
      error?: { code: number; message: string }
      params?: unknown
    }
    try {
      frame = JSON.parse(raw) as typeof frame
    } catch {
      return
    }
    if (frame.method) {
      for (const handler of this.handlers.get(frame.method) ?? []) handler(frame.params)
      return
    }
    if (frame.id === undefined) return
    const pending = this.pending.get(frame.id)
    if (!pending) return
    this.pending.delete(frame.id)
    window.clearTimeout(pending.timer)
    if (frame.error) {
      const error = new Error(frame.error.message) as RpcError
      error.code = frame.error.code
      pending.reject(error)
      return
    }
    pending.resolve(frame.result)
  }

  private failAll(error: Error): void {
    for (const [, pending] of this.pending) {
      window.clearTimeout(pending.timer)
      pending.reject(error)
    }
    this.pending.clear()
  }

  subscribe(topics: string[]): void {
    let added = false
    for (const topic of topics) {
      if (!this.topics.has(topic)) {
        this.topics.add(topic)
        added = true
      }
    }
    if (!added) return
    if (this.state === 'live') {
      void this.call('events.subscribe', { topics: [...this.topics] }).catch(() => {})
    }
  }
}
