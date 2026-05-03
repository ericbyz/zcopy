import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

describe('TaskList', () => {
  it('passes server list to task cards so task status can show its file server', () => {
    const source = readFileSync(
      join(process.cwd(), 'src/views/components/TaskList.vue'),
      'utf8'
    )

    expect(source).toContain('serverList')
    expect(source).toContain(':server-list="serverList"')
  })
})
