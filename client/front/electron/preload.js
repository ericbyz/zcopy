import { contextBridge, ipcRenderer } from 'electron'

contextBridge.exposeInMainWorld('desktopApi', {
  pickFolder: () => ipcRenderer.invoke('dialog:pick-folder'),
  openPath: (targetPath) => ipcRenderer.invoke('shell:open-path', targetPath)
})
