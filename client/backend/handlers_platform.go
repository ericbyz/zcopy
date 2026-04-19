package main

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/platform"
)

func (a *AppState) initTaskCFAPI(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	if runtime.GOOS == "darwin" {
		if !a.fileProviderAvailable() {
			c.JSON(http.StatusBadRequest, gin.H{"message": "当前 mac 客户端未启用 File Provider 支持"})
			return
		}
		status, err := a.initTaskFileProvider(task)
		if err != nil {
			a.pushLog("error", task, "", "注册 File Provider 失败: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "注册 File Provider 失败: " + err.Error()})
			return
		}
		a.pushLog("info", task, "", "File Provider 域注册成功")
		c.JSON(http.StatusOK, gin.H{
			"message":    "File Provider 域注册成功",
			"mountPath":  status.MountPath,
			"logPath":    status.LogPath,
			"registered": status.Registered,
		})
		return
	}
	if runtime.GOOS != "windows" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "仅 Windows 支持 Cloud Files API"})
		return
	}
	localPath := task.LocalPath
	if task.CloudOnly && localPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "获取用户目录失败: " + err.Error()})
			return
		}
		localPath = filepath.Join(homeDir, "ZCopy", platform.SafeName(task.Name))
		if err := os.MkdirAll(localPath, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "创建本地目录失败: " + err.Error()})
			return
		}
		task.LocalPath = localPath
		if err := a.store.Upsert(task); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "保存任务失败"})
			return
		}
	}
	if err := registerWindowsSyncRoot(task.ID, task.Name, localPath); err != nil {
		a.pushLog("error", task, "", "注册 Cloud Files 同步根失败: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "注册 Cloud Files 同步根失败: " + err.Error()})
		return
	}
	a.pushLog("info", task, "", "Cloud Files 同步根注册成功")
	c.JSON(http.StatusOK, gin.H{"message": "Cloud Files 同步根注册成功"})
}

func (a *AppState) taskCFAPIStatus(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	if runtime.GOOS == "darwin" {
		if !a.fileProviderAvailable() {
			c.JSON(http.StatusOK, gin.H{
				"supported":  false,
				"registered": false,
				"reason":     "mac File Provider bridge 未就绪",
			})
			return
		}
		status, err := a.getTaskFileProviderStatus(task)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"supported":  true,
				"registered": false,
				"reason":     err.Error(),
				"rootId":     platform.SyncRootID(task.ID),
				"mode":       "file_provider",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"supported":  true,
			"registered": status.Registered,
			"reason":     status.Reason,
			"rootId":     platform.SyncRootID(task.ID),
			"mountPath":  status.MountPath,
			"logPath":    status.LogPath,
			"mode":       "file_provider",
		})
		return
	}
	if runtime.GOOS != "windows" {
		c.JSON(http.StatusOK, gin.H{"supported": false, "registered": false})
		return
	}
	registered, reason := isSyncRootRegistered(task.ID)
	c.JSON(http.StatusOK, gin.H{
		"supported":  true,
		"registered": registered,
		"reason":     reason,
		"rootId":     platform.SyncRootID(task.ID),
	})
}

func (a *AppState) systemCapabilities(c *gin.Context) {
	onDemandSupport := runtime.GOOS == "windows" || a.fileProviderAvailable()
	onDemandMode := "incremental-upload"
	if runtime.GOOS == "darwin" && onDemandSupport {
		onDemandMode = "file_provider"
	}
	c.JSON(http.StatusOK, gin.H{
		"os":              runtime.GOOS,
		"onDemandSupport": onDemandSupport,
		"onDemandMode":    onDemandMode,
	})
}

func registerWindowsSyncRoot(taskID string, taskName string, localPath string) error {
	script := `
param(
  [Parameter(Mandatory = $true)][string]$RootPath,
  [Parameter(Mandatory = $true)][string]$RootId,
  [Parameter(Mandatory = $true)][string]$DisplayName
)
Set-StrictMode -Version Latest
if (-not (Test-Path -LiteralPath $RootPath)) {
  throw "同步根路径不存在: $RootPath"
}
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Storage.Provider.StorageProviderSyncRootManager, Windows.Storage, ContentType=WindowsRuntime]
if (-not [Windows.Storage.Provider.StorageProviderSyncRootManager]::IsSupported()) {
  throw "当前系统不支持 StorageProviderSyncRootManager"
}
$folderOp = [Windows.Storage.StorageFolder]::GetFolderFromPathAsync($RootPath)
$folder = [System.Runtime.InteropServices.WindowsRuntime.WindowsRuntimeMarshal]::AsTask($folderOp).GetAwaiter().GetResult()
$info = [Windows.Storage.Provider.StorageProviderSyncRootInfo]::new()
$info.Id = $RootId
$info.Path = $folder
$info.DisplayNameResource = $DisplayName
$info.IconResource = "$env:SystemRoot\System32\shell32.dll,3"
$info.HydrationPolicy = [Windows.Storage.Provider.StorageProviderHydrationPolicy]::Progressive
$info.HydrationPolicyModifier = [Windows.Storage.Provider.StorageProviderHydrationPolicyModifier]::AutoDehydrationAllowed
$info.PopulationPolicy = [Windows.Storage.Provider.StorageProviderPopulationPolicy]::Full
$info.InSyncPolicy = [Windows.Storage.Provider.StorageProviderInSyncPolicy]::FileCreationTime -bor [Windows.Storage.Provider.StorageProviderInSyncPolicy]::DirectoryCreationTime -bor [Windows.Storage.Provider.StorageProviderInSyncPolicy]::FileLastWriteTime -bor [Windows.Storage.Provider.StorageProviderInSyncPolicy]::DirectoryLastWriteTime
$info.Version = "1.0.0"
$info.ShowSiblingsAsGroup = $false
try {
  $existing = [Windows.Storage.Provider.StorageProviderSyncRootManager]::GetSyncRootInformationForId($RootId)
  if ($existing) {
    [Windows.Storage.Provider.StorageProviderSyncRootManager]::Unregister($RootId)
  }
} catch {
}
[Windows.Storage.Provider.StorageProviderSyncRootManager]::Register($info)
`
	tempFile, err := os.CreateTemp("", "zcopy-cfapi-*.ps1")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempPath)
	if err := os.WriteFile(tempPath, []byte(script), 0644); err != nil {
		return err
	}
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Sta",
		"-File", tempPath,
		"-RootPath", localPath,
		"-RootId", platform.SyncRootID(taskID),
		"-DisplayName", "ZCopy "+taskName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

func isSyncRootRegistered(taskID string) (bool, string) {
	keyPath := `HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\SyncRootManager\` + platform.SyncRootID(taskID)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "if (Test-Path '"+keyPath+"') { Write-Output 'yes' } else { Write-Output 'no' }")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	return strings.TrimSpace(string(out)) == "yes", strings.TrimSpace(string(out))
}
