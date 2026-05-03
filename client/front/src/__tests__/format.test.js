import { describe, it, expect } from 'vitest'
import { formatBytes, formatSpeed } from '../utils/format.js'

describe('formatBytes', () => {
  const byteTests = [
    { name: '0 bytes', input: 0, want: '0 B' },
    { name: 'null value', input: null, want: '0 B' },
    { name: 'undefined value', input: undefined, want: '0 B' },
    { name: '500 bytes', input: 500, want: '500 B' },
    { name: '1 KB', input: 1024, want: '1.00 KB' },
    { name: '1.5 KB', input: 1536, want: '1.50 KB' },
    { name: '1 MB', input: 1048576, want: '1.00 MB' },
    { name: '1 GB', input: 1073741824, want: '1.00 GB' },
    { name: '2.5 GB', input: 2684354560, want: '2.50 GB' }
  ]

  for (const tt of byteTests) {
    it(tt.name, () => {
      expect(formatBytes(tt.input)).toBe(tt.want)
    })
  }
})

describe('formatSpeed', () => {
  const speedTests = [
    { name: '0 speed', input: 0, want: '0 B/s' },
    { name: 'null value', input: null, want: '0 B/s' },
    { name: 'negative speed', input: -100, want: '0 B/s' },
    { name: '512 B/s', input: 512, want: '512 B/s' },
    { name: '1 KB/s', input: 1024, want: '1.00 KB/s' },
    { name: '1 MB/s', input: 1048576, want: '1.00 MB/s' }
  ]

  for (const tt of speedTests) {
    it(tt.name, () => {
      expect(formatSpeed(tt.input)).toBe(tt.want)
    })
  }
})
