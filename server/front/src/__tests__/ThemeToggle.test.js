import { describe, it, expect } from 'vitest'
import { cycleThemeMode } from '../utils/theme.js'

describe('cycleThemeMode', () => {
  it('cycles from light to dark', () => {
    expect(cycleThemeMode('light')).toBe('dark')
  })

  it('cycles from dark to auto', () => {
    expect(cycleThemeMode('dark')).toBe('auto')
  })

  it('cycles from auto to light', () => {
    expect(cycleThemeMode('auto')).toBe('light')
  })

  it('handles unknown values by returning light', () => {
    expect(cycleThemeMode('unknown')).toBe('light')
  })
})
