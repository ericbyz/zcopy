import { app, BrowserWindow, dialog, ipcMain, globalShortcut, shell } from 'electron'
import { spawn } from 'child_process'
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
  backendProcess = spawn(backendExe, [], {
    cwd: path.dirname(backendExe),
    env,
    windowsHide: true,
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

app.whenReady().then(() => {
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
