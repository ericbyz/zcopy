import { describe, it, expect } from 'vitest'
import { getTaskStatusText, calcSyncProgress } from '../utils/task.js'

describe('getTaskStatusText', () => {
  const statusTests = [
    { name: 'syncing state', input: { syncReport: { state: 'syncing' } }, want: '同步中' },
    { name: 'completed state', input: { syncReport: { state: 'completed' } }, want: '空闲' },
    { name: 'idle syncReport state', input: { syncReport: { state: 'idle' } }, want: '空闲' },
    { name: 'no syncReport but status idle', input: { status: 'idle' }, want: '空闲' },
    { name: 'failed state', input: { syncReport: { state: 'failed' } }, want: '失败' },
    { name: 'error state', input: { syncReport: { state: 'error' } }, want: '失败' },
    { name: 'empty object', input: {}, want: '空闲' },
    { name: 'null value', input: null, want: '空闲' },
    { name: 'unknown state', input: { syncReport: { state: 'paused' } }, want: 'paused' }
  ]

  for (const tt of statusTests) {
    it(tt.name, () => {
      expect(getTaskStatusText(tt.input)).toBe(tt.want)
    })
  }
})

describe('calcSyncProgress', () => {
  const progressTests = [
    { name: 'no files', input: { syncReport: { totalFiles: 0 } }, want: 0 },
    { name: 'half completed', input: { syncReport: { totalFiles: 10, uploadedFiles: 5, failedFiles: 0 } }, want: 50 },
    { name: 'all completed', input: { syncReport: { totalFiles: 10, uploadedFiles: 10, failedFiles: 0 } }, want: 100 },
    { name: 'completed including failed', input: { syncReport: { totalFiles: 10, uploadedFiles: 8, failedFiles: 2 } }, want: 100 },
    { name: 'empty object', input: {}, want: 0 },
    { name: 'null value', input: null, want: 0 }
  ]

  for (const tt of progressTests) {
    it(tt.name, () => {
      expect(calcSyncProgress(tt.input)).toBe(tt.want)
    })
  }
})
