import { cpSync, existsSync, mkdirSync, rmSync } from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const projectRoot = path.resolve(__dirname, '..')
const buildOutputPath = path.join(projectRoot, 'fileprovider', '.build', 'Release')
const productPath = path.join(buildOutputPath, 'EleFileProvider.appex')
const outputPath = path.join(projectRoot, 'electron', 'PlugIns', 'EleFileProvider.appex')
const allowUnsigned = process.env.ZCOPY_ALLOW_UNSIGNED_FILEPROVIDER === '1'

mkdirSync(path.dirname(outputPath), { recursive: true })
mkdirSync(path.dirname(productPath), { recursive: true })
rmSync(outputPath, { recursive: true, force: true })
rmSync(buildOutputPath, { recursive: true, force: true })

const args = [
  '-project', path.join(projectRoot, 'fileprovider', 'EleFileProvider.xcodeproj'),
  '-target', 'EleFileProvider',
  '-configuration', 'Release',
  `CONFIGURATION_BUILD_DIR=${buildOutputPath}`,
  'build'
]

if (allowUnsigned) {
  args.splice(args.length - 1, 0, 'CODE_SIGNING_ALLOWED=NO')
}

const build = spawnSync('xcodebuild', args, {
  cwd: projectRoot,
  stdio: 'inherit'
})

if (build.status !== 0) {
  process.exit(build.status ?? 1)
}

if (!existsSync(productPath)) {
  console.error(`Built appex not found: ${productPath}`)
  process.exit(1)
}

cpSync(productPath, outputPath, { recursive: true })
console.log(`Copied File Provider extension to ${outputPath}`)
