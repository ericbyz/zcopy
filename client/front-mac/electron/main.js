import { randomUUID } from 'crypto'
import { app, BrowserWindow, dialog, ipcMain, globalShortcut, shell } from 'electron'
import { spawn, spawnSync } from 'child_process'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const isDev = !app.isPackaged

let backendProcess = null
let fileProviderHostProcess = null
const bridgeState = {
  url: '',
  token: '',
  stateDir: '',
  hostBundlePath: ''
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

function resolveBundledFileProviderHostArchive() {
  if (isDev) {
    return path.resolve(__dirname, '../fileprovider-host/ZCopyFileProviderHost.zip')
  }
  return path.join(process.resourcesPath, 'fileprovider-host', 'ZCopyFileProviderHost.zip')
}

function resolveInstalledFileProviderHost() {
  return path.join(app.getPath('home'), 'Applications', 'ZCopyFileProviderHost.app')
}

function resolveFileProviderHostExecutable(bundlePath) {
  return path.join(bundlePath, 'Contents', 'MacOS', 'ZCopyFileProviderHost')
}

function ensureFileProviderHostInstalled() {
  const sourceArchive = resolveBundledFileProviderHostArchive()
  const installedBundle = resolveInstalledFileProviderHost()
  if (!fs.existsSync(sourceArchive)) {
    throw new Error(`file provider host archive not found: ${sourceArchive}`)
  }
  fs.mkdirSync(path.dirname(installedBundle), { recursive: true })
  fs.rmSync(installedBundle, { recursive: true, force: true })
  const unzip = spawnSync('ditto', ['-x', '-k', sourceArchive, path.dirname(installedBundle)], {
    stdio: 'ignore'
  })
  if (unzip.status !== 0 || !fs.existsSync(installedBundle)) {
    throw new Error(`failed to install file provider host from archive: ${sourceArchive}`)
  }
  bridgeState.hostBundlePath = installedBundle
  return installedBundle
}

function waitForBridgeInfo(bridgeInfoPath, expectedToken, timeoutMs = 15000) {
  const startedAt = Date.now()
  return new Promise((resolve, reject) => {
    const poll = () => {
      if (Date.now() - startedAt > timeoutMs) {
        reject(new Error(`timed out waiting for file provider host bridge: ${bridgeInfoPath}`))
        return
      }
      if (!fs.existsSync(bridgeInfoPath)) {
        setTimeout(poll, 250)
        return
      }
      try {
        const parsed = JSON.parse(fs.readFileSync(bridgeInfoPath, 'utf8'))
        if (parsed.token !== expectedToken || !parsed.url) {
          setTimeout(poll, 250)
          return
        }
        resolve(parsed)
      } catch {
        setTimeout(poll, 250)
      }
    }
    poll()
  })
}

async function startFileProviderBridge() {
  if (process.platform !== 'darwin') {
    return
  }

  const hostBundle = ensureFileProviderHostInstalled()
  const executable = resolveFileProviderHostExecutable(hostBundle)
  const stateDir = path.join(app.getPath('userData'), 'fileprovider-host')
  const bridgeInfoPath = path.join(stateDir, 'bridge.json')
  const token = randomUUID()

  fs.rmSync(stateDir, { recursive: true, force: true })
  fs.mkdirSync(stateDir, { recursive: true })

  if (fileProviderHostProcess && !fileProviderHostProcess.killed) {
    fileProviderHostProcess.kill()
  }

  fileProviderHostProcess = spawn(executable, [], {
    env: {
      ...process.env,
      ZCOPY_FILE_PROVIDER_STATE_DIR: stateDir,
      ZCOPY_FILE_PROVIDER_BRIDGE_TOKEN: token
    },
    stdio: 'ignore'
  })

  bridgeState.stateDir = stateDir
  bridgeState.token = token

  const bridgeInfo = await waitForBridgeInfo(bridgeInfoPath, token)
  bridgeState.url = bridgeInfo.url
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
  const logDir = path.join(app.getPath('userData'), 'logs')
  fs.mkdirSync(logDir, { recursive: true })
  const backendLogPath = path.join(logDir, 'backend.log')
  const backendLogFd = fs.openSync(backendLogPath, 'a')
  backendProcess = spawn(backendExe, [], {
    cwd: path.dirname(backendExe),
    env,
    stdio: ['ignore', backendLogFd, backendLogFd]
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
  try {
    await startFileProviderBridge()
  } catch (error) {
    console.warn('[zcopy] failed to start native file provider host', error)
  }
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
  if (fileProviderHostProcess && !fileProviderHostProcess.killed) {
    fileProviderHostProcess.kill()
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

ipcMain.handle('shell:open-path', async (_event, targetPath) => {
  if (!targetPath || typeof targetPath !== 'string') {
    return '路径无效'
  }
  try {
    if (fs.existsSync(targetPath)) {
      const stat = fs.statSync(targetPath)
      if (stat.isDirectory()) {
        const fallback = spawnSync('open', [targetPath], { stdio: 'ignore' })
        if (fallback.status === 0) {
          return ''
        }
      }
    }
    const failure = await shell.openPath(targetPath)
    if (!failure) {
      return ''
    }
    if (fs.existsSync(targetPath)) {
      shell.showItemInFolder(targetPath)
      return ''
    }
    const fallback = spawnSync('open', [targetPath], { stdio: 'ignore' })
    if (fallback.status === 0) {
      return ''
    }
    return failure
  } catch (error) {
    return error instanceof Error ? error.message : String(error)
  }
})
