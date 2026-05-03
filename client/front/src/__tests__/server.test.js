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
  it('uses explicit webUrl when available, ignoring address and fallback', () => {
    expect(buildFileServerWebURL({ address: '10.0.0.2:8890', webUrl: 'http://files.example.com:8080' }))
      .toBe('http://files.example.com:8080')
  })

  it('uses explicit webUrl even when fallback differs', () => {
    expect(buildFileServerWebURL({ webUrl: 'https://my-server.com:443' }, 'http://localhost:5176'))
      .toBe('https://my-server.com')
  })

  it('adds http:// to webUrl when no scheme provided', () => {
    expect(buildFileServerWebURL({ webUrl: '10.0.0.5:3000' })).toBe('http://10.0.0.5:3000')
  })

  it('derives web URL from API address by using fallback web port (no webUrl)', () => {
    expect(buildFileServerWebURL({ address: '10.0.0.2:8890' })).toBe('http://10.0.0.2:5176')
  })

  it('preserves https when server address includes a scheme, overrides port to web port (no webUrl)', () => {
    expect(buildFileServerWebURL({ address: 'https://files.example.com:8890' })).toBe('https://files.example.com:5176')
  })

  it('uses configured fallback port for matching servers (no webUrl)', () => {
    expect(buildFileServerWebURL({ address: '10.0.0.2:8890' }, 'http://localhost:8080')).toBe('http://10.0.0.2:8080')
  })

  it('falls back when neither webUrl nor address is present', () => {
    expect(buildFileServerWebURL(null, 'http://fallback:5176')).toBe('http://fallback:5176')
  })
})

describe('buildFileServerPathURL', () => {
  it('opens the remote path on the matching file server (no webUrl)', () => {
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

  it('uses server webUrl when available, with remote path appended', () => {
    const url = buildFileServerPathURL({
      task: { serverId: 'server-b', remotePath: 'docs/project a' },
      servers: [
        { id: 'server-b', address: '10.0.0.2:8890', webUrl: 'http://files.example.com:8080' }
      ],
      fallbackURL: 'http://localhost:5176'
    })

    expect(url).toBe('http://files.example.com:8080/?path=docs%2Fproject+a')
  })
})
