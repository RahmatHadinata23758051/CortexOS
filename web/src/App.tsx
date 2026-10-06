import { useEffect, useState } from 'react'

import { loadRuntimeSnapshot, type RuntimeState } from './bridge/runtime'
import { wailsRuntimeBridge } from './types'

const initialState: RuntimeState = { status: 'loading' }

export function App() {
  const [state, setState] = useState<RuntimeState>(initialState)

  useEffect(() => {
    let active = true
    void loadRuntimeSnapshot(wailsRuntimeBridge).then((nextState) => {
      if (active) {
        setState(nextState)
      }
    })
    return () => {
      active = false
    }
  }, [])

  return (
    <main>
      <section aria-live="polite">
        <p>CortexOS</p>
        <h1>Runtime foundation</h1>
        {state.status === 'loading' && <p>Connecting to the local runtime...</p>}
        {state.status === 'success' && (
          <dl>
            <div>
              <dt>Status</dt>
              <dd>{state.snapshot.status}</dd>
            </div>
            <div>
              <dt>Environment</dt>
              <dd>{state.snapshot.environment}</dd>
            </div>
            <div>
              <dt>Provider</dt>
              <dd>{state.snapshot.provider}</dd>
            </div>
          </dl>
        )}
        {state.status === 'error' && (
          <p role="alert">Unable to connect: {state.message}</p>
        )}
      </section>
    </main>
  )
}
