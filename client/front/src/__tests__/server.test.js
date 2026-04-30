import { describe, expect, it } from 'vitest'
import { buildFileServerPathURL, buildFileServerWebURL, findServerForTask } from '../utils/server.js'

describe('findServerForTask', () => {
  it('finds the server that owns a task', () => {
    const servers = [
      { id: 'server-a', name: 'A', address: '10.0.0.1:8890' },
      { id: 'server-b', name: 'B', address: '10.0.0.2:8890' }
    ]

    expect(findServerForTask({ serverId: 'server-b' }, servers)).toEqual(servers[1])
  })

  it('returns null when the task has no matching server', () => {
    expect(findServerForTask({ serverId: 'missing' }, [])).toBeNull()
  })
})

describe('buildFileServerWebURL', () => {
  it('uses the task server host with the file server web port', () => {
    expect(buildFileServerWebURL({ address: '10.0.0.2:8890' })).toBe('http://10.0.0.2:5176')
  })

  it('preserves https when the server address includes a scheme', () => {
    expect(buildFileServerWebURL({ address: 'https://files.example.com:8890' })).toBe('https://files.example.com:5176')
  })

  it('falls back when the server address is absent', () => {
    expect(buildFileServerWebURL(null, 'http://fallback:5176')).toBe('http://fallback:5176')
  })

  it('uses the configured fallback web port for matching servers', () => {
    expect(buildFileServerWebURL({ address: '10.0.0.2:8890' }, 'http://localhost:8080')).toBe('http://10.0.0.2:8080')
  })
})

describe('buildFileServerPathURL', () => {
  it('opens the remote path on the matching file server', () => {
    const url = buildFileServerPathURL({
      task: { serverId: 'server-b', remotePath: 'docs/project a' },
      servers: [
        { id: 'server-a', address: '10.0.0.1:8890' },
        { id: 'server-b', address: '10.0.0.2:8890' }
      ],
      fallbackURL: 'http://localhost:5176'
    })

    expect(url).toBe('http://10.0.0.2:5176/?path=docs%2Fproject+a')
  })
})
