// Web mode: replaces Wails IPC with fetch calls to /api/<method>

const BASE = '/api'

async function call(method, ...args) {
  const res = await fetch(`${BASE}/${method}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ args }),
  })
  if (!res.ok) throw new Error(`API ${method} failed: ${res.status}`)
  return res.json()
}

export const AbsolutePath       = (a1)                   => call('AbsolutePath', a1)
export const CloseMMDB          = (a1, a2)               => call('CloseMMDB', a1, a2)
export const CopyFile           = (a1, a2)               => call('CopyFile', a1, a2)
export const Download           = (a1, a2, a3, a4, a5, a6) => call('Download', a1, a2, a3, a4, a5, a6)
export const Exec               = (a1, a2, a3)           => call('Exec', a1, a2, a3)
export const ExecBackground     = (a1, a2, a3, a4, a5)   => call('ExecBackground', a1, a2, a3, a4, a5)
export const ExitApp            = ()                     => call('ExitApp')
export const FileExists         = (a1)                   => call('FileExists', a1)
export const GetEnv             = (a1)                   => call('GetEnv', a1)
export const GetInterfaces      = ()                     => call('GetInterfaces')
export const IsStartup          = ()                     => call('IsStartup')
export const KillProcess        = (a1, a2)               => call('KillProcess', a1, a2)
export const ListServer         = ()                     => call('ListServer')
export const MakeDir            = (a1)                   => call('MakeDir', a1)
export const MoveFile           = (a1, a2)               => call('MoveFile', a1, a2)
export const OpenDir            = (a1)                   => call('OpenDir', a1)
export const OpenMMDB           = (a1, a2)               => call('OpenMMDB', a1, a2)
export const OpenURI            = (a1)                   => call('OpenURI', a1)
export const ProcessInfo        = (a1)                   => call('ProcessInfo', a1)
export const ProcessMemory      = (a1)                   => call('ProcessMemory', a1)
export const QueryMMDB          = (a1, a2, a3)           => call('QueryMMDB', a1, a2, a3)
export const ReadDir            = (a1)                   => call('ReadDir', a1)
export const ReadFile           = (a1, a2)               => call('ReadFile', a1, a2)
export const RemoveFile         = (a1)                   => call('RemoveFile', a1)
export const Requests           = (a1, a2, a3, a4, a5)  => call('Requests', a1, a2, a3, a4, a5)
export const RestartApp         = ()                     => call('RestartApp')
export const ShowMainWindow     = ()                     => call('ShowMainWindow')
export const StartServer        = (a1, a2, a3)           => call('StartServer', a1, a2, a3)
export const StopServer         = (a1)                   => call('StopServer', a1)
export const UnzipGZFile        = (a1, a2)               => call('UnzipGZFile', a1, a2)
export const UnzipTarGZFile     = (a1, a2)               => call('UnzipTarGZFile', a1, a2)
export const UnzipZIPFile       = (a1, a2)               => call('UnzipZIPFile', a1, a2)
export const UpdateTray         = (a1)                   => call('UpdateTray', a1)
export const UpdateTrayAndMenus = (a1, a2)               => call('UpdateTrayAndMenus', a1, a2)
export const UpdateTrayMenus    = (a1)                   => call('UpdateTrayMenus', a1)
export const Upload             = (a1, a2, a3, a4, a5, a6) => call('Upload', a1, a2, a3, a4, a5, a6)
export const WriteFile          = (a1, a2, a3)           => call('WriteFile', a1, a2, a3)
