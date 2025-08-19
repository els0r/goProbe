import { useEffect, useRef } from 'react'

// simplified hook to run an async function with cancellation when deps change
export function useCancelableFetch(fn: (signal: AbortSignal) => void | Promise<void>, deps: unknown[]) {
  const abortRef = useRef<AbortController | null>(null)
  useEffect(() => {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller
    void fn(controller.signal)
    return () => controller.abort()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)
}
