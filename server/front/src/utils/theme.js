export function cycleThemeMode(current) {
  const modes = ['light', 'dark', 'auto']
  const idx = modes.indexOf(current)
  if (idx === -1) {
    return 'light'
  }
  return modes[(idx + 1) % modes.length]
}
