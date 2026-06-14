import { useSyncExternalStore } from 'react'
import { QueryRunner, RunnerSnapshot } from './runner'

// Thin React adapter over the framework-free QueryRunner.
export function useQueryRunner(runner: QueryRunner): RunnerSnapshot {
  return useSyncExternalStore(
    (onStoreChange) => runner.subscribe(onStoreChange),
    () => runner.getSnapshot()
  )
}
