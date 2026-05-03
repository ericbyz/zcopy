export function formatBytes(value) {
  const size = Number(value || 0)
  if (size >= 1024 * 1024 * 1024) return `${(size / 1024 / 1024 / 1024).toFixed(2)} GB`
  if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(2)} MB`
  if (size >= 1024) return `${(size / 1024).toFixed(2)} KB`
  return `${size.toFixed(0)} B`
}

export function formatSpeed(value) {
  const speed = Number(value || 0)
  if (speed <= 0) return '0 B/s'
  if (speed >= 1024 * 1024) return `${(speed / 1024 / 1024).toFixed(2)} MB/s`
  if (speed >= 1024) return `${(speed / 1024).toFixed(2)} KB/s`
  return `${speed.toFixed(0)} B/s`
}
