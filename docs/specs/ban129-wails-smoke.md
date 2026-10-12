# BAN-129 Wails desktop bridge smoke

This manual smoke check crosses the actual desktop boundary: Wails generates
`window.go.platform.Bridge` from the Go binding, invokes the production Go
bridge and application service, and delivers the resulting event through the
Wails runtime. The existing Vitest integration fixture remains a frontend
contract test; it does not establish this desktop integration.

## Run

From the repository root, start the Wails development application with browser
developer tools enabled:

```powershell
wails dev -devtools
```

Wait for the CortexOS window to finish loading. Open its developer console and
run the following snippet. It subscribes first, then calls the generated
`GetCockpitRuntime` binding, which makes the real Go bridge call the application
service and emit `cortexos:cockpit:runtime:changed` through Wails.

```javascript
(async () => {
  const eventName = 'cortexos:cockpit:runtime:changed'
  const bridge = window.go?.platform?.Bridge
  const runtime = window.runtime
  if (!bridge?.GetCockpitRuntime || !runtime?.EventsOn) {
    throw new Error('Wails generated binding or runtime event API is unavailable')
  }

  let unsubscribe
  let timer
  try {
    const event = new Promise((resolve, reject) => {
      unsubscribe = runtime.EventsOn(eventName, resolve)
      timer = setTimeout(() => reject(new Error(`Timed out waiting for ${eventName}`)), 5000)
    })
    const snapshot = await bridge.GetCockpitRuntime()
    const payload = await event
    if (snapshot?.schemaVersion !== 'cortexos.cockpit.bridge.v1' || snapshot.status !== 'ready') {
      throw new Error(`Unexpected Go bridge response: ${JSON.stringify(snapshot)}`)
    }
    if (JSON.stringify(payload) !== JSON.stringify(snapshot)) {
      throw new Error('Wails event payload did not match the Go bridge response')
    }
    console.info('BAN-129 Wails bridge smoke passed', { snapshot, payload })
    return { snapshot, payload }
  } finally {
    clearTimeout(timer)
    if (typeof unsubscribe === 'function') unsubscribe()
    else runtime.EventsOff?.(eventName)
  }
})()
```

## Pass criteria

- The window loads and the generated `window.go.platform.Bridge` binding exists.
- `GetCockpitRuntime()` resolves with the current Cockpit schema and ready
  status from the real Go service.
- The Wails runtime delivers the matching event within five seconds.
- The console prints `BAN-129 Wails bridge smoke passed` without an error.
- Closing the window exits the development application cleanly.

The bridge remains a transport adapter: business logic and event-triggering
service calls stay in Go, while React and Phaser remain presentation-only.
