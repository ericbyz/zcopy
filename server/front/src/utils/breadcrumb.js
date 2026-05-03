export function buildBreadcrumbList(currentPath) {
  if (!currentPath) {
    return [{ label: '根目录', path: '' }]
  }

  const parts = currentPath.split('/').filter(Boolean)
  return [{ label: '根目录', path: '' }].concat(
    parts.map((part, index) => ({
      label: part,
      path: parts.slice(0, index + 1).join('/')
    }))
  )
}
