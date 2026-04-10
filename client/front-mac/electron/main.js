import { randomUUID } from 'crypto'
import { app, BrowserWindow, dialog, ipcMain, globalShortcut } from 'electron'
import { spawn } from 'child_process'
import { createServer } from 'http'
import fs from 'fs'
import path from 'path'
import { createRequire } from 'module'
import { fileURLToPath } from 'url'

const require = createRequire(import.meta.url)
const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const isDev = !app.isPackaged

let backendProcess = null
let bridgeServer = null
let bridgeState = {
  url: '',
  token: '',
  fileProvider: null
}

function resolveBackendExecutable() {
  if (isDev) {
    return path.resolve(__dirname, '../../backend/zcopy-client-backend')
  }
  return path.join(process.resourcesPath, 'backend', 'zcopy-client-backend')
}

function resolveBackendConfig() {
  if (isDev) {
    return path.resolve(__dirname, '../../backend/config/config.yaml')
  }
  return path.join(process.resourcesPath, 'backend', 'config', 'config.yaml')
}

function resolveRendererEntry() {
  if (isDev) {
    return path.resolve(__dirname, '../../front/dist/index.html')
  }
  return path.join(process.resourcesPath, 'renderer', 'index.html')
}

function loadFileProviderModule() {
  if (process.platform !== 'darwin') {
    return null
  }
  if (bridgeState.fileProvider) {
    return bridgeState.fileProvider
  }
  try {
    bridgeState.fileProvider = require('electron-macos-file-provider')
  } catch (error) {
    console.warn('[zcopy] failed to load electron-macos-file-provider', error)
    bridgeState.fileProvider = null
  }
  return bridgeState.fileProvider
}

function sendJSON(response, statusCode, payload) {
  response.writeHead(statusCode, { 'Content-Type': 'application/json; charset=utf-8' })
  response.end(JSON.stringify(payload))
}

function verifyBridgeRequest(request) {
  const auth = request.headers.authorization || ''
  return auth === `Bearer ${bridgeState.token}`
}

async function readJSONBody(request) {
  const chunks = []
  for await (const chunk of request) {
    chunks.push(chunk)
  }
  if (chunks.length === 0) {
    return {}
  }
  return JSON.parse(Buffer.concat(chunks).toString('utf8'))
}

function registerDomain(fileProvider, payload) {
  return new Promise((resolve, reject) => {
    fileProvider.addDomain(
      payload.id,
      payload.name,
      {
        url: payload.url,
        user: payload.user,
        password: payload.password
      },
      (error) => {
        if (error) {
          reject(new Error(String(error)))
          return
        }
        resolve()
      }
    )
  })
}

async function queryDomainStatus(fileProvider, id, name) {
  try {
    const mountPath = await fileProvider.getUserVisiblePath(id, name)
    return {
      registered: Boolean(mountPath),
      mountPath: mountPath || '',
      logPath: fileProvider.getFileProviderLogPath()
    }
  } catch (error) {
    return {
      registered: false,
      reason: error instanceof Error ? error.message : String(error),
      mountPath: '',
      logPath: typeof fileProvider.getFileProviderLogPath === 'function' ? fileProvider.getFileProviderLogPath() : ''
    }
  }
}

function startFileProviderBridge() {
  if (process.platform !== 'darwin') {
    return Promise.resolve()
  }

  const fileProvider = loadFileProviderModule()
  bridgeState.token = randomUUID()

  return new Promise((resolve) => {
    bridgeServer = createServer(async (request, response) => {
      if (!verifyBridgeRequest(request)) {
        sendJSON(response, 401, { message: 'unauthorized' })
        return
      }
      if (!fileProvider) {
        sendJSON(response, 501, { message: 'file provider module unavailable' })
        return
      }

      try {
        const url = new URL(request.url || '/', 'http://127.0.0.1')
        if (request.method === 'POST' && url.pathname === '/register') {
          const payload = await readJSONBody(request)
          await registerDomain(fileProvider, payload)
          const status = await queryDomainStatus(fileProvider, payload.id, payload.name)
          sendJSON(response, 200, {
            message: 'ok',
            ...status
          })
          return
        }
        if (request.method === 'GET' && url.pathname === '/status') {
          const id = url.searchParams.get('id') || ''
          const name = url.searchParams.get('name') || ''
          const status = await queryDomainStatus(fileProvider, id, name)
          sendJSON(response, 200, status)
          return
        }
        sendJSON(response, 404, { message: 'not found' })
      } catch (error) {
        sendJSON(response, 500, {
          message: error instanceof Error ? error.message : String(error)
        })
      }
    })

    bridgeServer.listen(0, '127.0.0.1', () => {
      const address = bridgeServer.address()
      const port = typeof address === 'object' && address ? address.port : 0
      bridgeState.url = `http://127.0.0.1:${port}`
      resolve()
    })
  })
}

function startBackend() {
  const backendExe = resolveBackendExecutable()
  const backendConfig = resolveBackendConfig()
  if (!fs.existsSync(backendExe)) {
    throw new Error(`backend executable not found: ${backendExe}`)
  }
  if (!fs.existsSync(backendConfig)) {
    throw new Error(`backend config not found: ${backendConfig}`)
  }
  const env = {
    ...process.env,
    ZCOPY_CLIENT_CONFIG: backendConfig,
    ZCOPY_CLIENT_DATA_DIR: path.join(app.getPath('userData'), 'backend-data')
  }
  if (process.platform === 'darwin' && bridgeState.url && bridgeState.token) {
    env.ZCOPY_CLIENT_FP_BRIDGE_URL = bridgeState.url
    env.ZCOPY_CLIENT_FP_BRIDGE_TOKEN = bridgeState.token
  }
  backendProcess = spawn(backendExe, [], {
    cwd: path.dirname(backendExe),
    env,
    stdio: 'ignore'
  })
}

function createWindow() {
  const win = new BrowserWindow({
    width: 1360,
    height: 860,
    minWidth: 1100,
    minHeight: 700,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      devTools: true
    }
  })

  win.loadFile(resolveRendererEntry())

  win.webContents.on('before-input-event', (event, input) => {
    if (input.key === 'F12' && input.type === 'keyDown') {
      if (win.webContents.isDevToolsOpened()) {
        win.webContents.closeDevTools()
      } else {
        win.webContents.openDevTools({ mode: 'detach' })
      }
      event.preventDefault()
    }
  })
}

app.whenReady().then(async () => {
  await startFileProviderBridge()
  startBackend()
  createWindow()
  globalShortcut.register('F12', () => {
    const windows = BrowserWindow.getAllWindows()
    if (windows.length === 0) {
      return
    }
    const win = windows[0]
    if (win.webContents.isDevToolsOpened()) {
      win.webContents.closeDevTools()
    } else {
      win.webContents.openDevTools({ mode: 'detach' })
    }
  })
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    app.quit()
  }
})

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow()
  }
})

app.on('will-quit', () => {
  if (backendProcess && !backendProcess.killed) {
    backendProcess.kill()
  }
  if (bridgeServer) {
    bridgeServer.close()
  }
  globalShortcut.unregisterAll()
})

ipcMain.handle('dialog:pick-folder', async () => {
  const result = await dialog.showOpenDialog({
    properties: ['openDirectory', 'createDirectory']
  })
  if (result.canceled || result.filePaths.length === 0) {
    return ''
  }
  return result.filePaths[0]
})
