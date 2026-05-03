import { describe, it, expect } from 'vitest'
import { buildBreadcrumbList } from '../utils/breadcrumb.js'

describe('buildBreadcrumbList', () => {
  const tests = [
    {
      name: 'empty string returns root only',
      input: '',
      want: [{ label: '根目录', path: '' }]
    },
    {
      name: 'single directory',
      input: 'docs',
      want: [
        { label: '根目录', path: '' },
        { label: 'docs', path: 'docs' }
      ]
    },
    {
      name: 'nested directories',
      input: 'docs/projects/2024',
      want: [
        { label: '根目录', path: '' },
        { label: 'docs', path: 'docs' },
        { label: 'projects', path: 'docs/projects' },
        { label: '2024', path: 'docs/projects/2024' }
      ]
    },
    {
      name: 'leading slash',
      input: '/leading/slash',
      want: [
        { label: '根目录', path: '' },
        { label: 'leading', path: 'leading' },
        { label: 'slash', path: 'leading/slash' }
      ]
    },
    {
      name: 'trailing slash',
      input: 'trailing/',
      want: [
        { label: '根目录', path: '' },
        { label: 'trailing', path: 'trailing' }
      ]
    }
  ]

  for (const tt of tests) {
    it(tt.name, () => {
      expect(buildBreadcrumbList(tt.input)).toEqual(tt.want)
    })
  }
})
