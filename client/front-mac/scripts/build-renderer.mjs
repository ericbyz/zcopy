import { existsSync } from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const frontPath = path.resolve(__dirname, '../../front')
const srcEntry = path.join(frontPath, 'src', 'main.js')
const distEntry = path.join(frontPath, 'dist', 'index.html')

if (existsSync(srcEntry)) {
  const result = spawnSync('npm', ['run', 'build'], {
    cwd: frontPath,
    stdio: 'inherit'
  })
  process.exit(result.status ?? 1)
}

if (existsSync(distEntry)) {
  console.log(`Renderer source missing, reusing existing dist at ${distEntry}`)
  process.exit(0)
}

console.error('Renderer source and dist are both missing; cannot package mac client')
process.exit(1)
