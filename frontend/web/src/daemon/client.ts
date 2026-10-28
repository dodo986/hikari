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

/**
 * JSON-RPC 2.0 over a websocket, one request per id, events pushed by the
 * daemon on the same connection. Reconnects on its own and re-subscribes.
 */
export class RpcClient {
  private socket: WebSocket | null = null
  private nextId = 1
  private pending = new Map<number, Pending>()
  private handlers = new Map<string, Set<EventHandler>>()
  private topics = new Set<string>()
  private state: ConnectionState = 'connecting'
  private backoff = BACKOFF_START_MS
  private reconnectTimer = 0
  private closed = false
  private started = false
  private attempts = 0

  constructor(
    private readonly open: () => Promise<EndpointConfig>,
    private readonly onState: StateHandler = () => {},
  ) {}

  get connection(): ConnectionState {
    return this.state
  }

  start(): void {
    if (this.started || this.closed) return
    this.started = true
    void this.connect()
  }

  stop(): void {
    this.closed = true
    window.clearTimeout(this.reconnectTimer)
    this.socket?.close()
    this.socket = null
    this.failAll(new Error('the connection was closed'))
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
    if (this.closed) return
    this.started = true
    this.setState(this.attempts === 0 ? 'connecting' : this.state)
    let endpoint: EndpointConfig
    try {
      endpoint = await this.open()
    } catch {
      this.attempts++
      this.setState('offline')
      this.scheduleReconnect()
      return
    }
    const socket = new WebSocket(`${endpoint.url}?token=${encodeURIComponent(endpoint.token)}`)
    this.socket = socket
    // A socket replaced by a newer attempt must not report state or serve calls.
    socket.onmessage = (message: MessageEvent<string>) => {
      if (this.socket === socket) this.receive(message.data)
    }
    socket.onerror = () => {
      // A handshake refusal surfaces as an error before close; the close
      // handler decides what to do next.
    }
    socket.onclose = () => {
      if (this.socket !== socket) return
      this.failAll(new Error('hikarid closed the connection'))
      if (this.closed) return
      this.attempts++
      this.setState(this.attempts >= 2 ? 'offline' : 'connecting')
      this.scheduleReconnect()
    }
    socket.onopen = () => {
      if (this.socket !== socket) {
        socket.close()
        return
      }
      this.attempts = 0
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
      if (this.closed) return
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

  /** Subscribe once; the topics survive reconnects. */
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
