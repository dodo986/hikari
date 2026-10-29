import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RpcClient } from '../src/daemon/client'

class FakeSocket {
  static OPEN = 1
  static instances: FakeSocket[] = []
  readyState = 0
  sent: string[] = []
  onopen: (() => void) | null = null
  onclose: ((event: { code: number }) => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((event: { data: string }) => void) | null = null

  constructor(readonly url: string) {
    FakeSocket.instances.push(this)
  }

  send(frame: string): void {
    this.sent.push(frame)
  }

  close(code = 1000): void {
    this.readyState = 3
    this.onclose?.({ code })
  }

  accept(): void {
    this.readyState = FakeSocket.OPEN
    this.onopen?.()
  }

  deliver(frame: unknown): void {
    this.onmessage?.({ data: JSON.stringify(frame) })
  }
}

const endpoint = { url: 'ws://127.0.0.1:1/rpc', token: 'abc' }
const latest = (): FakeSocket => FakeSocket.instances[FakeSocket.instances.length - 1]!

beforeEach(() => {
  FakeSocket.instances = []
  vi.stubGlobal('WebSocket', FakeSocket)
  vi.useRealTimers()
})

const settle = () => new Promise((resolve) => setTimeout(resolve, 0))

describe('rpc client', () => {
  it('authenticates the url and reports the connection state', async () => {
    const states: string[] = []
    const client = new RpcClient(async () => endpoint, (state) => states.push(state))
    client.start()
    await settle()
    expect(latest().url).toBe('ws://127.0.0.1:1/rpc?token=abc')
    latest().accept()
    expect(states).toEqual(['live'])
    expect(client.connection).toBe('live')
  })

  it('resolves a call by id and rejects an rpc error', async () => {
    const client = new RpcClient(async () => endpoint)
    client.start()
    await settle()
    latest().accept()

    const ok = client.call<{ games: string[] }>('library.list')
    await settle()
    expect(JSON.parse(latest().sent[0]!)).toMatchObject({ jsonrpc: '2.0', id: 1, method: 'library.list' })
    latest().deliver({ jsonrpc: '2.0', id: 1, result: { games: [] } })
    await expect(ok).resolves.toEqual({ games: [] })

    const failing = client.call('game.launch')
    await settle()
    latest().deliver({ jsonrpc: '2.0', id: 2, error: { code: -32000, message: 'not installed' } })
    await expect(failing).rejects.toThrow('not installed')
  })

  it('routes events to their handlers and keeps subscriptions across reconnects', async () => {
    const client = new RpcClient(async () => endpoint)
    const seen: unknown[] = []
    client.on('library.changed', (params) => seen.push(params))
    client.subscribe(['library.changed'])
    client.start()
    await settle()
    latest().accept()
    await settle()

    latest().deliver({ jsonrpc: '2.0', method: 'library.changed', params: { store: 'gog' } })
    expect(seen).toEqual([{ store: 'gog' }])
    expect(latest().sent.some((frame) => frame.includes('events.subscribe'))).toBe(true)
  })

  it('refuses calls while the socket is closed', async () => {
    const client = new RpcClient(async () => endpoint)
    client.start()
    await settle()
    await expect(client.call('library.list')).rejects.toThrow('not connected')
  })

  it('tells a refused token apart from a stopped daemon', async () => {
    const states: string[] = []
    const client = new RpcClient(async () => endpoint, (state) => states.push(state))
    client.start()
    await settle()
    latest().close(4401)
    expect(client.connection).toBe('unauthorized')
    expect(states).toContain('unauthorized')
  })

  it('picks up the token a restarted daemon wrote', async () => {
    vi.useFakeTimers()
    const tokens = ['old', 'new']
    let opened = 0
    const client = new RpcClient(async () => ({
      url: endpoint.url,
      token: tokens[Math.min(opened++, 1)]!,
    }))
    client.start()
    await vi.advanceTimersByTimeAsync(0)
    expect(latest().url).toContain('token=old')

    latest().close(4401)
    await vi.advanceTimersByTimeAsync(1000)
    expect(latest().url).toContain('token=new')
    vi.useRealTimers()
  })

  it('reports offline when the endpoint cannot be resolved twice', async () => {
    const states: string[] = []
    const client = new RpcClient(
      async () => {
        throw new Error('no daemon')
      },
      (state) => states.push(state),
    )
    client.start()
    await settle()
    expect(states).toContain('offline')
  })
})
