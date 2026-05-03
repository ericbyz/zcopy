import { describe, it, expect } from 'vitest'
import { parseDownloadFilename } from '../utils/format.js'

describe('parseDownloadFilename', () => {
  const tests = [
    {
      name: 'null contentDisposition returns fallback',
      input: [null, 'fallback.txt'],
      want: 'fallback.txt'
    },
    {
      name: 'empty string returns fallback',
      input: ['', 'fallback.txt'],
      want: 'fallback.txt'
    },
    {
      name: 'UTF-8 encoded filename',
      input: ['filename*=UTF-8\'\'test%20file.txt', 'fallback.txt'],
      want: 'test file.txt'
    },
    {
      name: 'basic quoted filename',
      input: ['filename="report.pdf"', 'fallback.txt'],
      want: 'report.pdf'
    },
    {
      name: 'basic unquoted filename',
      input: ['filename=doc.docx', 'fallback.txt'],
      want: 'doc.docx'
    },
    {
      name: 'Chinese UTF-8 filename',
      input: ['filename*=UTF-8\'\'%E4%B8%AD%E6%96%87.txt', 'fallback.txt'],
      want: '中文.txt'
    },
    {
      name: 'complex disposition with multiple params',
      input: ['attachment; filename*=UTF-8\'\'image.png; size=12345', 'fallback.txt'],
      want: 'image.png'
    }
  ]

  for (const tt of tests) {
    it(tt.name, () => {
      expect(parseDownloadFilename(...tt.input)).toBe(tt.want)
    })
  }
})
