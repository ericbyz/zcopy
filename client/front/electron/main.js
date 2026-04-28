import { app, BrowserWindow, dialog, ipcMain, globalShortcut, shell } from 'electron'
import { spawn, spawnSync } from 'child_process'
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const isDev = !app.isPackaged
let backendProcess = null

function resolveBackendExecutable() {
  if (isDev) {
    return path.resolve(__dirname, '../../backend/zcopy-client-backend.exe')
  }
  return path.join(process.resourcesPath, 'backend', 'zcopy-client-backend.exe')
}

function resolveBackendConfig() {
  if (isDev) {
    return path.resolve(__dirname, '../../backend/config/config.yaml')
  }
  return path.join(process.resourcesPath, 'backend', 'config', 'config.yaml')
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
  const logDir = path.join(app.getPath('userData'), 'logs')
  fs.mkdirSync(logDir, { recursive: true })
  const backendLogPath = path.join(logDir, 'backend.log')
  const backendLogFd = fs.openSync(backendLogPath, 'a')
  backendProcess = spawn(backendExe, [], {
    cwd: path.dirname(backendExe),
    env,
    windowsHide: true,
    stdio: ['ignore', backendLogFd, backendLogFd]
  })
}

async function waitForBackendHealth() {
  const startedAt = Date.now()
  const timeoutMs = 10000
  const pollIntervalMs = 100

  while (Date.now() - startedAt < timeoutMs) {
    try {
      const response = await fetch('http://localhost:8090/health')
      if (response.ok) {
        return
      }
    } catch {
    }
    await new Promise(resolve => setTimeout(resolve, pollIntervalMs))
  }

  throw new Error('backend health check timed out after 10 seconds')
}

function createWindow() {
  const iconPath = isDev
    ? path.resolve(__dirname, '../resource/icon.png')
    : path.join(process.resourcesPath, 'app', 'build', 'icon.png')
  const win = new BrowserWindow({
    width: 900,
    height: 640,
    minWidth: 760,
    minHeight: 540,
    title: '',
    titleBarStyle: 'hidden',
    backgroundColor: '#f5f5f7',
    icon: iconPath,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      devTools: true
    }
  })

  if (isDev) {
    win.loadURL('http://localhost:5173')
    win.webContents.openDevTools({ mode: 'detach' })
  } else {
    win.loadFile(path.join(__dirname, '../dist/index.html'))
  }

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
  startBackend()
  try {
    await waitForBackendHealth()
  } catch (error) {
    console.error('[zcopy] backend health check failed:', error)
    app.quit()
    return
  }
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
      win.webContents.openDevTools({ mode: 'detached' })
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
    const tryOpen = async () => {
      const failure = await shell.openPath(targetPath)
      if (!failure) {
        return ''
      }
      const fallback = spawnSync('open', [targetPath], { stdio: 'ignore' })
      return fallback.status === 0 ? '' : failure
    }

    const tryReveal = () => {
      if (fs.existsSync(targetPath)) {
        shell.showItemInFolder(targetPath)
        return ''
      }
      const reveal = spawnSync('open', ['-R', targetPath], { stdio: 'ignore' })
      return reveal.status === 0 ? '' : `路径不存在：${targetPath}`
    }

    if (fs.existsSync(targetPath)) {
      const stat = fs.statSync(targetPath)
      if (stat.isDirectory()) {
        const failure = await tryOpen()
        if (!failure) {
          return ''
        }
        const revealed = tryReveal()
        return revealed || failure
      }
    }

    const failure = await tryOpen()
    if (!failure) {
      return ''
    }

    const revealed = tryReveal()
    if (!revealed) {
      return ''
    }
    return failure || revealed
  } catch (error) {
    return error instanceof Error ? error.message : String(error)
  }
})

ipcMain.handle('shell:open-external', async (_event, targetUrl) => {
  if (!targetUrl || typeof targetUrl !== 'string') {
    return '链接无效'
  }
  try {
    const parsed = new URL(targetUrl)
    if (!['http:', 'https:'].includes(parsed.protocol)) {
      return '仅支持打开 http/https 链接'
    }
    await shell.openExternal(parsed.toString())
    return ''
  } catch (error) {
    return error instanceof Error ? error.message : String(error)
  }
})
