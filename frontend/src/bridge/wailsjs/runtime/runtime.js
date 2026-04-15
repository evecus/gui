// Web mode: replaces Wails runtime with WebSocket-based event bus

let _ws = null
let _reconnectTimer = null
const _listeners = {}   // name -> [callback, ...]

function getWsUrl() {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${location.host}/ws`
}

function connect() {
  if (_ws && (_ws.readyState === WebSocket.CONNECTING || _ws.readyState === WebSocket.OPEN)) return

  _ws = new WebSocket(getWsUrl())

  _ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      if (msg.type === 'event') {
        const fns = _listeners[msg.name]
        if (fns) fns.forEach(fn => fn(...(msg.args || [])))
      }
    } catch {}
  }

  _ws.onclose = () => {
    if (!_reconnectTimer) {
      _reconnectTimer = setTimeout(() => {
        _reconnectTimer = null
        connect()
      }, 2000)
    }
  }

  _ws.onerror = () => _ws.close()
}

connect()

export function EventsOn(name, callback) {
  if (!_listeners[name]) _listeners[name] = []
  _listeners[name].push(callback)
}

export function EventsOnce(name, callback) {
  const wrapper = (...args) => {
    EventsOff(name, wrapper)
    callback(...args)
  }
  EventsOn(name, wrapper)
}

export function EventsOnMultiple(name, callback, maxCallbacks) {
  let count = 0
  const wrapper = (...args) => {
    count++
    if (maxCallbacks > 0 && count >= maxCallbacks) EventsOff(name, wrapper)
    callback(...args)
  }
  EventsOn(name, wrapper)
}

export function EventsOff(name, ...additionalNames) {
  delete _listeners[name]
  additionalNames.forEach(n => delete _listeners[n])
}

export function EventsOffAll() {
  Object.keys(_listeners).forEach(k => delete _listeners[k])
}

export function EventsEmit(name, ...args) {
  // Send event to server (e.g. to cancel a download)
  if (_ws && _ws.readyState === WebSocket.OPEN) {
    _ws.send(JSON.stringify({ type: 'emit', name, args }))
  }
  // Also fire local listeners
  const fns = _listeners[name]
  if (fns) fns.forEach(fn => fn(...args))
}

// Window operations - no-op in web mode
export function WindowHide() {}
export function WindowShow() {}
export function WindowMinimise() {}
export function WindowMaximise() {}
export function WindowUnmaximise() {}
export function WindowSetTitle() {}
export function Quit() {}

// Notifications - stub
export async function IsNotificationAvailable() { return false }
export async function RequestNotificationAuthorization() { return false }
export async function SendNotification() {}

// Log stubs
export function LogPrint(m) { console.log(m) }
export function LogTrace(m) { console.trace(m) }
export function LogDebug(m) { console.debug(m) }
export function LogError(m) { console.error(m) }
export function LogFatal(m) { console.error(m) }
export function LogInfo(m)  { console.info(m) }
export function LogWarning(m) { console.warn(m) }

// Window stubs
export function WindowReload() { location.reload() }
export function WindowReloadApp() { location.reload() }
export function WindowSetAlwaysOnTop() {}
export function WindowSetSystemDefaultTheme() {}
export function WindowSetLightTheme() {}
export function WindowSetDarkTheme() {}
export function WindowCenter() {}
export function WindowFullscreen() {}
export function WindowUnfullscreen() {}
export async function WindowIsFullscreen() { return false }
export function WindowSetSize() {}
export async function WindowGetSize() { return { w: window.innerWidth, h: window.innerHeight } }
export function WindowSetMaxSize() {}
export function WindowSetMinSize() {}
export function WindowSetPosition() {}
export async function WindowGetPosition() { return { x: 0, y: 0 } }

// Clipboard
export async function ClipboardGetText() {
  return navigator.clipboard?.readText?.() ?? ''
}
export async function ClipboardSetText(text) {
  return navigator.clipboard?.writeText?.(text).then(() => true) ?? false
}

// File drop - no-op
export function OnFileDrop() {}
export function OnFileDropOff() {}
export function CanResolveFilePaths() { return false }
export function ResolveFilePaths() {}

// Notification stubs
export async function InitializeNotifications() {}
export async function CleanupNotifications() {}
export async function CheckNotificationAuthorization() { return false }
export async function SendNotificationWithActions() {}
export async function RegisterNotificationCategory() {}
export async function RemoveNotificationCategory() {}
export async function RemoveAllPendingNotifications() {}
export async function RemovePendingNotification() {}
export async function RemoveAllDeliveredNotifications() {}
export async function RemoveDeliveredNotification() {}
export async function RemoveNotification() {}
