import { useSyncExternalStore } from 'react'
import { DetailRunner, DetailSnapshot } from './detailRunner'

// Thin React adapter over the framework-free DetailRunner.
export function useDetailRunner(runner: DetailRunner): DetailSnapshot {
  return useSyncExternalStore(
    (onStoreChange) => runner.subscribe(onStoreChange),
    () => runner.getSnapshot()
  )
}
