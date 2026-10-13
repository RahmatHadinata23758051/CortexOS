import { useCallback, useEffect, useRef, useState } from 'react'
import {
  dispatchBrainTaskStream,
  isBrainTerminalState,
  onBrainStateForTask,
  onBrainStreamForTask,
  type BrainDispatchInput,
} from '../bridge/brain'
import {
  type BrainBridge,
  type BrainDispatchResult,
  type BrainState,
  type BrainStreamChunk,
} from '../types'

export type UseBrainDispatchState = {
  taskId: string | null
  state: BrainState
  output: string
  tokens: number
  steps: number
  durationMs: number
  error: string | null
  chunks: BrainStreamChunk[]
  isRunning: boolean
}

export type UseBrainDispatchOptions = {
  bridge: BrainBridge
  onComplete?: (result: BrainDispatchResult) => void
  onError?: (err: Error) => void
}

const initialState: UseBrainDispatchState = {
  taskId: null,
  state: 'idle',
  output: '',
  tokens: 0,
  steps: 0,
  durationMs: 0,
  error: null,
  chunks: [],
  isRunning: false,
}

export function useBrainDispatch({ bridge, onComplete, onError }: UseBrainDispatchOptions) {
  const [session, setSession] = useState<UseBrainDispatchState>(initialState)
  const activeTaskRef = useRef<string | null>(null)
  const outputAccumulatorRef = useRef<string>('')

  const reset = useCallback(() => {
    activeTaskRef.current = null
    outputAccumulatorRef.current = ''
    setSession(initialState)
  }, [])

  const run = useCallback(
    async (input: BrainDispatchInput) => {
      reset()

      const generatedTaskId = input.taskId ?? `task-${Date.now()}`
      activeTaskRef.current = generatedTaskId

      setSession((prev) => ({
        ...prev,
        taskId: generatedTaskId,
        state: 'planning',
        isRunning: true,
        error: null,
      }))

      // Register event listeners before dispatching
      const unsubChunk = onBrainStreamForTask(generatedTaskId, (chunk) => {
        if (activeTaskRef.current !== generatedTaskId) return

        if (chunk.type === 'token') {
          outputAccumulatorRef.current += chunk.data
        }

        setSession((prev) => {
          let nextState: BrainState = prev.state
          if (chunk.type === 'token' && prev.state !== 'streaming') {
            nextState = 'streaming'
          }

          return {
            ...prev,
            state: nextState,
            output: outputAccumulatorRef.current,
            chunks: [...prev.chunks, chunk],
          }
        })
      })

      const unsubState = onBrainStateForTask(generatedTaskId, (result) => {
        if (activeTaskRef.current !== generatedTaskId) return

        setSession((prev) => ({
          ...prev,
          state: result.state,
          steps: result.steps,
          tokens: result.tokens,
          durationMs: result.durationMs,
          output: result.output || outputAccumulatorRef.current,
          error: result.error || null,
          isRunning: !isBrainTerminalState(result.state),
        }))
      })

      try {
        const result = await dispatchBrainTaskStream(bridge, {
          ...input,
          taskId: generatedTaskId,
        })

        if (activeTaskRef.current === generatedTaskId) {
          setSession((prev) => ({
            ...prev,
            taskId: result.taskId,
            state: result.state,
            output: result.output || outputAccumulatorRef.current,
            steps: result.steps,
            tokens: result.tokens,
            durationMs: result.durationMs,
            error: result.error || null,
            isRunning: false,
          }))

          if (result.error) {
            onError?.(new Error(result.error))
          } else {
            onComplete?.(result)
          }
        }
        return result
      } catch (err) {
        const error = err instanceof Error ? err : new Error(String(err))
        if (activeTaskRef.current === generatedTaskId) {
          setSession((prev) => ({
            ...prev,
            state: 'failed',
            error: error.message,
            isRunning: false,
          }))
          onError?.(error)
        }
        throw error
      } finally {
        unsubChunk()
        unsubState()
      }
    },
    [bridge, onComplete, onError, reset],
  )

  useEffect(() => {
    return () => {
      activeTaskRef.current = null
    }
  }, [])

  return {
    ...session,
    run,
    reset,
  }
}
