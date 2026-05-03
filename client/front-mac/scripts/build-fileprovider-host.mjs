import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import path from 'node:path'
import { execFileSync, spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const projectRoot = path.resolve(__dirname, '..')
const hostRoot = path.join(projectRoot, 'fileprovider-host')
const buildRoot = path.join(hostRoot, '.build')
const appPath = path.join(buildRoot, 'zcopy.app')
const executablePath = path.join(appPath, 'Contents', 'MacOS', 'ZCopyFileProviderHost')
const bundledOutputPath = path.join(projectRoot, 'electron', 'fileprovider-host', 'zcopy.app')
const bundledZipPath = path.join(projectRoot, 'electron', 'fileprovider-host', 'ZCopyFileProviderHost.zip')
const appexPath = path.join(projectRoot, 'electron', 'PlugIns', 'EleFileProvider.appex')

if (!existsSync(appexPath)) {
  console.error(`Missing File Provider appex: ${appexPath}`)
  process.exit(1)
}

rmSync(buildRoot, { recursive: true, force: true })
rmSync(bundledOutputPath, { recursive: true, force: true })
rmSync(bundledZipPath, { force: true })
mkdirSync(path.join(appPath, 'Contents', 'MacOS'), { recursive: true })
mkdirSync(path.join(appPath, 'Contents', 'PlugIns'), { recursive: true })
mkdirSync(path.dirname(bundledOutputPath), { recursive: true })

cpSync(path.join(hostRoot, 'Info.plist'), path.join(appPath, 'Contents', 'Info.plist'))
cpSync(appexPath, path.join(appPath, 'Contents', 'PlugIns', 'EleFileProvider.appex'), { recursive: true })

const compile = spawnSync('xcrun', [
  'swiftc',
  '-target', 'arm64-apple-macos13.0',
  '-framework', 'AppKit',
  '-framework', 'FileProvider',
  '-framework', 'Network',
  path.join(hostRoot, 'Sources', 'main.swift'),
  '-o', executablePath
], {
  cwd: projectRoot,
  stdio: 'inherit'
})

if (compile.status !== 0) {
  process.exit(compile.status ?? 1)
}

const resolveCodesignIdentity = () => {
  if (process.env.ZCOPY_HOST_CODESIGN_IDENTITY) {
    return process.env.ZCOPY_HOST_CODESIGN_IDENTITY
  }
  const output = execFileSync('security', ['find-identity', '-v', '-p', 'codesigning'], { encoding: 'utf8' })
  const lines = output.split('\n')
  const appleDevelopment = lines.find((line) => line.includes('Apple Development'))
  if (!appleDevelopment) {
    throw new Error('No Apple Development codesigning identity found')
  }
  const match = appleDevelopment.match(/"(.+?)"/)
  if (!match) {
    throw new Error('Unable to parse Apple Development codesigning identity')
  }
  return match[1]
}

const identity = resolveCodesignIdentity()

const sign = spawnSync('codesign', [
  '--force',
  '--sign', identity,
  '--entitlements', path.join(hostRoot, 'Host.entitlements'),
  appPath
], {
  cwd: projectRoot,
  stdio: 'inherit'
})

if (sign.status !== 0) {
  process.exit(sign.status ?? 1)
}

cpSync(appPath, bundledOutputPath, { recursive: true })
const archive = spawnSync('ditto', [
  '-c',
  '-k',
  '--sequesterRsrc',
  '--keepParent',
  appPath,
  bundledZipPath
], {
  cwd: buildRoot,
  stdio: 'inherit'
})

if (archive.status !== 0) {
  process.exit(archive.status ?? 1)
}

console.log(`Built File Provider host app at ${bundledOutputPath}`)
console.log(`Archived File Provider host app at ${bundledZipPath}`)
