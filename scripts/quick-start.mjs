#!/usr/bin/env node
import { cpSync, existsSync, mkdirSync, readdirSync, statSync } from 'node:fs'
import path from 'node:path'
import { spawn, spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..')
const isWindows = process.platform === 'win32'
const isMac = process.platform === 'darwin'
const npmCommand = isWindows ? 'npm.cmd' : 'npm'

if (!isWindows && !isMac) {
  console.error(`当前系统 ${process.platform} 暂不支持该快捷脚本，仅支持 macOS 和 Windows。`)
  process.exit(1)
}

const serverBackendDir = path.join(repoRoot, 'server', 'backend')
const serverFrontDir = path.join(repoRoot, 'server', 'front')
const windowsClientDir = path.join(repoRoot, 'client', 'front')
const macClientDir = path.join(repoRoot, 'client', 'front-mac')
const timeLabel = createTimeLabel()
const outputDir = path.join(
  repoRoot,
  'release',
  'quick-start',
  isMac ? 'mac' : 'windows',
  timeLabel
)
const childProcesses = []

function createTimeLabel() {
  const now = new Date()
  const pad = (value) => String(value).padStart(2, '0')
  return [
    now.getFullYear(),
    pad(now.getMonth() + 1),
    pad(now.getDate())
  ].join('') + '-' + [
    pad(now.getHours()),
    pad(now.getMinutes()),
    pad(now.getSeconds())
  ].join('')
}

function startLongRunningProcess(label, command, args, cwd) {
  console.log(`\n[${label}] 启动中...`)
  const child = spawn(command, args, {
    cwd,
    env: process.env,
    stdio: 'inherit'
  })

  child.on('exit', (code, signal) => {
    if (signal) {
      console.log(`[${label}] 已退出，signal=${signal}`)
      return
    }
    console.log(`[${label}] 已退出，code=${code ?? 0}`)
  })

  childProcesses.push(child)
  return child
}

function runBlockingProcess(label, command, args, cwd) {
  console.log(`\n[${label}] 执行：${command} ${args.join(' ')}`)
  const result = spawnSync(command, args, {
    cwd,
    env: process.env,
    stdio: 'inherit'
  })

  if (result.status !== 0) {
    throw new Error(`[${label}] 执行失败，退出码：${result.status ?? 1}`)
  }
}

function collectFiles(dir, matcher) {
  if (!existsSync(dir)) {
    return []
  }

  const entries = readdirSync(dir, { withFileTypes: true })
  const results = []

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      results.push(...collectFiles(fullPath, matcher))
      continue
    }
    if (matcher(fullPath)) {
      results.push(fullPath)
    }
  }

  return results
}

function sortByMtimeDesc(filePaths) {
  return [...filePaths].sort((left, right) => {
    return statSync(right).mtimeMs - statSync(left).mtimeMs
  })
}

function copyArtifacts(artifactPaths) {
  if (artifactPaths.length === 0) {
    throw new Error('没有找到可复制的打包产物。')
  }

  mkdirSync(outputDir, { recursive: true })
  for (const artifactPath of artifactPaths) {
    const targetPath = path.join(outputDir, path.basename(artifactPath))
    cpSync(artifactPath, targetPath, { recursive: true })
  }
}

function packageWindowsClient() {
  runBlockingProcess('Windows 客户端打包', npmCommand, ['run', 'dist:portable'], windowsClientDir)
  const releaseDir = path.join(windowsClientDir, 'release')
  const portableExes = sortByMtimeDesc(
    collectFiles(releaseDir, (filePath) => filePath.toLowerCase().endsWith('.exe'))
  )

  if (portableExes.length === 0) {
    throw new Error(`未在 ${releaseDir} 中找到 exe 产物。`)
  }

  copyArtifacts([portableExes[0]])
}

function packageMacClient() {
  runBlockingProcess('macOS 客户端打包', npmCommand, ['run', 'dist'], macClientDir)
  const releaseDir = path.join(macClientDir, 'release')
  const dmgFiles = sortByMtimeDesc(
    collectFiles(releaseDir, (filePath) => filePath.toLowerCase().endsWith('.dmg'))
  )
  const appPath = path.join(releaseDir, 'mac-arm64', 'ZCopyClient.app')
  const artifacts = []

  if (dmgFiles.length > 0) {
    artifacts.push(dmgFiles[0])
  }
  if (existsSync(appPath)) {
    artifacts.push(appPath)
  }

  if (artifacts.length === 0) {
    throw new Error(`未在 ${releaseDir} 中找到 dmg 或 app 产物。`)
  }

  copyArtifacts(artifacts)
}

function stopAllChildren() {
  for (const child of childProcesses) {
    if (child.killed) {
      continue
    }
    try {
      child.kill('SIGTERM')
    } catch {
      // 子进程可能已退出，这里忽略即可。
    }
  }
}

function keepAlive() {
  console.log('\n服务端仍在运行，按 Ctrl+C 可一并停止。')
  setInterval(() => {}, 1 << 30)
}

process.on('SIGINT', () => {
  console.log('\n收到中断信号，正在停止服务...')
  stopAllChildren()
  process.exit(0)
})

process.on('SIGTERM', () => {
  stopAllChildren()
  process.exit(0)
})

try {
  startLongRunningProcess('服务端后端', 'go', ['run', 'main.go'], serverBackendDir)
  startLongRunningProcess('服务端前端', npmCommand, ['run', 'dev'], serverFrontDir)

  if (isMac) {
    packageMacClient()
  } else {
    packageWindowsClient()
  }

  console.log(`\n打包完成，产物已复制到：${outputDir}`)
  keepAlive()
} catch (error) {
  stopAllChildren()
  console.error(error instanceof Error ? error.message : String(error))
  process.exit(1)
}
