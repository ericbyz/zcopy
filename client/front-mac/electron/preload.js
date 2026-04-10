import { contextBridge, ipcRenderer } from 'electron'

contextBridge.exposeInMainWorld('desktopApi', {
  pickFolder: () => ipcRenderer.invoke('dialog:pick-folder')
})
