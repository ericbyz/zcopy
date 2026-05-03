const DEFAULT_FILE_SERVER_WEB_URL = 'http://localhost:5176'
const DEFAULT_FILE_SERVER_WEB_PORT = '5176'

function getFileServerWebPort(fallbackURL) {
  try {
    return new URL(fallbackURL).port || DEFAULT_FILE_SERVER_WEB_PORT
  } catch {
    return DEFAULT_FILE_SERVER_WEB_PORT
  }
}

export function findServerForTask(task, servers = []) {
  if (!task?.serverId) return null
  return servers.find((server) => server.id === task.serverId) || null
}

export function buildFileServerWebURL(server, fallbackURL = DEFAULT_FILE_SERVER_WEB_URL) {
  const webUrl = String(server?.webUrl || '').trim()
  if (webUrl) {
    try {
      const parsed = new URL(webUrl.includes('://') ? webUrl : `http://${webUrl}`)
      parsed.pathname = '/'
      parsed.search = ''
      parsed.hash = ''
      return parsed.toString().replace(/\/$/, '')
    } catch {
      return fallbackURL
    }
  }

  const address = String(server?.address || '').trim()
  if (!address) return fallbackURL

  try {
    const parsed = new URL(address.includes('://') ? address : `http://${address}`)
    parsed.port = getFileServerWebPort(fallbackURL)
    parsed.pathname = '/'
    parsed.search = ''
    parsed.hash = ''
    return parsed.toString().replace(/\/$/, '')
  } catch {
    return fallbackURL
  }
}

export function buildFileServerPathURL({ task, servers = [], fallbackURL = DEFAULT_FILE_SERVER_WEB_URL }) {
  const server = findServerForTask(task, servers)
  const url = new URL(buildFileServerWebURL(server, fallbackURL))
  const remotePath = task?.remotePath || ''
  if (remotePath) {
    url.searchParams.set('path', remotePath)
  }
  return url.toString()
}
