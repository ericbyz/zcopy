export function getTaskStatusText(task) {
  const state = task?.syncReport?.state || task?.status || 'idle'
  if (state === 'syncing') return '同步中'
  if (state === 'completed' || state === 'idle') return '空闲'
  if (state === 'failed' || state === 'error') return '失败'
  return state
}

export function calcSyncProgress(task) {
  const total = Number(task?.syncReport?.totalFiles || 0)
  if (total <= 0) return 0
  const finished =
    Number(task?.syncReport?.uploadedFiles || 0) +
    Number(task?.syncReport?.failedFiles || 0)
  return Math.min(100, Math.round((finished / total) * 100))
}
