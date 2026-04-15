package bridge

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ---- WebSocket Event Bus ----

type EventBus struct {
	mu      sync.RWMutex
	clients map[*wsClient]bool
}

type wsClient struct {
	conn     *websocket.Conn
	send     chan []byte
	mu       sync.Mutex
}

type wsMessage struct {
	Type string `json:"type"` // "event"
	Name string `json:"name"`
	Args []any  `json:"args"`
}

var globalBus = &EventBus{
	clients: make(map[*wsClient]bool),
}

func (b *EventBus) Register(c *wsClient) {
	b.mu.Lock()
	b.clients[c] = true
	b.mu.Unlock()
}

func (b *EventBus) Unregister(c *wsClient) {
	b.mu.Lock()
	delete(b.clients, c)
	b.mu.Unlock()
	close(c.send)
}

func (b *EventBus) Emit(name string, args ...any) {
	msg := wsMessage{Type: "event", Name: name, Args: args}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	b.mu.RLock()
	for c := range b.clients {
		select {
		case c.send <- data:
		default:
		}
	}
	b.mu.RUnlock()
}


// ---- WebSocket upgrader ----

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS upgrade error: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}
	globalBus.Register(client)

	// write pump
	go func() {
		defer conn.Close()
		for data := range client.send {
			client.mu.Lock()
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := conn.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()
			if err != nil {
				break
			}
		}
	}()

	// read pump (keep alive + receive EventsEmit from frontend)
	defer func() {
		globalBus.Unregister(client)
	}()
	conn.SetReadLimit(512 * 1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// ping ticker
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			client.mu.Lock()
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			client.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Frontend can emit events back (e.g. cancel download)
		var m wsMessage
		if json.Unmarshal(msg, &m) == nil && m.Type == "emit" {
			// Deliver to any pending listeners (used for cancel IDs etc.)
			globalBus.Emit(m.Name, m.Args...)
		}
	}
}

// ---- API handler ----

type apiRequest struct {
	Args []json.RawMessage `json:"args"`
}

type apiResponse struct {
	Flag bool   `json:"flag"`
	Data any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, msg string) {
	writeJSON(w, apiResponse{Flag: false, Data: msg})
}

func readArgs(r *http.Request) ([]json.RawMessage, error) {
	var req apiRequest
	body, err := io.ReadAll(io.LimitReader(r.Body, 8*1024*1024))
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return req.Args, nil
}

func mustString(raw json.RawMessage) string {
	var s string
	json.Unmarshal(raw, &s)
	return s
}

func mustStringSlice(raw json.RawMessage) []string {
	var s []string
	json.Unmarshal(raw, &s)
	return s
}

func mustInt(raw json.RawMessage) int {
	var v int
	json.Unmarshal(raw, &v)
	return v
}

func mustBool(raw json.RawMessage) bool {
	var v bool
	json.Unmarshal(raw, &v)
	return v
}

// RunWebServer starts the headless HTTP server on addr, serving the Vue SPA
// and a REST+WebSocket API that replaces Wails IPC.
func (a *App) RunWebServer(addr string, assets embed.FS) error {
	mux := http.NewServeMux()

	// WebSocket
	mux.HandleFunc("/ws", wsHandler)

	// API
	mux.HandleFunc("/api/", a.apiRouter)

	// Static frontend (embedded)
	distFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return err
	}
	fileServer := http.FileServer(http.FS(distFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		RollingRelease(fileServer).ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: cors(mux),
	}
	return srv.ListenAndServe()
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// apiRouter dispatches /api/<method> to the corresponding App method
func (a *App) apiRouter(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	method := r.URL.Path[len("/api/"):]
	args, err := readArgs(r)
	if err != nil {
		writeErr(w, "bad request: "+err.Error())
		return
	}

	switch method {

	// ---- env / app ----
	case "GetEnv":
		key := ""
		if len(args) > 0 {
			key = mustString(args[0])
		}
		writeJSON(w, a.GetEnv(key))

	case "IsStartup":
		writeJSON(w, a.IsStartup())

	case "ExitApp":
		// no-op in web mode
		writeJSON(w, FlagResult{true, "Success"})

	case "RestartApp":
		writeJSON(w, a.RestartApp())

	case "ShowMainWindow":
		writeJSON(w, FlagResult{true, "Success"})

	case "GetInterfaces":
		writeJSON(w, a.GetInterfaces())

	case "UpdateTray", "UpdateTrayMenus", "UpdateTrayAndMenus":
		writeJSON(w, FlagResult{true, "Success"})

	// ---- IO ----
	case "WriteFile":
		if len(args) < 3 {
			writeErr(w, "need 3 args")
			return
		}
		var opts IOOptions
		json.Unmarshal(args[2], &opts)
		writeJSON(w, a.WriteFile(mustString(args[0]), mustString(args[1]), opts))

	case "ReadFile":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		var opts IOOptions
		json.Unmarshal(args[1], &opts)
		writeJSON(w, a.ReadFile(mustString(args[0]), opts))

	case "MoveFile":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.MoveFile(mustString(args[0]), mustString(args[1])))

	case "RemoveFile":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.RemoveFile(mustString(args[0])))

	case "CopyFile":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.CopyFile(mustString(args[0]), mustString(args[1])))

	case "MakeDir":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.MakeDir(mustString(args[0])))

	case "ReadDir":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.ReadDir(mustString(args[0])))

	case "OpenDir":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.OpenDir(mustString(args[0])))

	case "OpenURI":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.OpenURI(mustString(args[0])))

	case "AbsolutePath":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.AbsolutePath(mustString(args[0])))

	case "FileExists":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.FileExists(mustString(args[0])))

	case "UnzipZIPFile":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.UnzipZIPFile(mustString(args[0]), mustString(args[1])))

	case "UnzipTarGZFile":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.UnzipTarGZFile(mustString(args[0]), mustString(args[1])))

	case "UnzipGZFile":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.UnzipGZFile(mustString(args[0]), mustString(args[1])))

	// ---- Exec ----
	case "Exec":
		if len(args) < 3 {
			writeErr(w, "need 3 args")
			return
		}
		var opts ExecOptions
		json.Unmarshal(args[2], &opts)
		writeJSON(w, a.Exec(mustString(args[0]), mustStringSlice(args[1]), opts))

	case "ExecBackground":
		if len(args) < 5 {
			writeErr(w, "need 5 args")
			return
		}
		var opts ExecOptions
		json.Unmarshal(args[4], &opts)
		writeJSON(w, a.ExecBackground(
			mustString(args[0]),
			mustStringSlice(args[1]),
			mustString(args[2]),
			mustString(args[3]),
			opts,
		))

	case "ProcessInfo":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		var pid int32
		json.Unmarshal(args[0], &pid)
		writeJSON(w, a.ProcessInfo(pid))

	case "ProcessMemory":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		var pid int32
		json.Unmarshal(args[0], &pid)
		writeJSON(w, a.ProcessMemory(pid))

	case "KillProcess":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.KillProcess(mustInt(args[0]), mustInt(args[1])))

	// ---- Net ----
	case "Requests":
		if len(args) < 5 {
			writeErr(w, "need 5 args")
			return
		}
		var headers map[string]string
		json.Unmarshal(args[3], &headers)
		var opts RequestOptions
		json.Unmarshal(args[4], &opts)
		writeJSON(w, a.Requests(mustString(args[0]), mustString(args[1]), headers, mustString(args[2]), opts))

	case "Download":
		if len(args) < 6 {
			writeErr(w, "need 6 args")
			return
		}
		var headers map[string]string
		json.Unmarshal(args[3], &headers)
		var opts RequestOptions
		json.Unmarshal(args[5], &opts)
		writeJSON(w, a.Download(mustString(args[0]), mustString(args[1]), mustString(args[2]), headers, mustString(args[4]), opts))

	case "Upload":
		if len(args) < 6 {
			writeErr(w, "need 6 args")
			return
		}
		var headers map[string]string
		json.Unmarshal(args[3], &headers)
		var opts RequestOptions
		json.Unmarshal(args[5], &opts)
		writeJSON(w, a.Upload(mustString(args[0]), mustString(args[1]), mustString(args[2]), headers, mustString(args[4]), opts))

	// ---- MMDB ----
	case "OpenMMDB":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.OpenMMDB(mustString(args[0]), mustString(args[1])))

	case "CloseMMDB":
		if len(args) < 2 {
			writeErr(w, "need 2 args")
			return
		}
		writeJSON(w, a.CloseMMDB(mustString(args[0]), mustString(args[1])))

	case "QueryMMDB":
		if len(args) < 3 {
			writeErr(w, "need 3 args")
			return
		}
		writeJSON(w, a.QueryMMDB(mustString(args[0]), mustString(args[1]), mustString(args[2])))

	// ---- Server ----
	case "StartServer":
		if len(args) < 3 {
			writeErr(w, "need 3 args")
			return
		}
		var opts ServerOptions
		json.Unmarshal(args[2], &opts)
		writeJSON(w, a.StartServer(mustString(args[0]), mustString(args[1]), opts))

	case "StopServer":
		if len(args) < 1 {
			writeErr(w, "need 1 arg")
			return
		}
		writeJSON(w, a.StopServer(mustString(args[0])))

	case "ListServer":
		writeJSON(w, a.ListServer())

	default:
		http.Error(w, "unknown method: "+method, 404)
	}
}

// ---- Event listener registry (replaces wails runtime.EventsOn/Off) ----

type listenerFunc func(data ...any)

type listenerRegistry struct {
	mu        sync.RWMutex
	listeners map[string][]listenerFunc
}

var globalListeners = &listenerRegistry{
	listeners: make(map[string][]listenerFunc),
}

func EventsOn(name string, fn listenerFunc) {
	globalListeners.mu.Lock()
	globalListeners.listeners[name] = append(globalListeners.listeners[name], fn)
	globalListeners.mu.Unlock()
}

func EventsOff(name string) {
	globalListeners.mu.Lock()
	delete(globalListeners.listeners, name)
	globalListeners.mu.Unlock()
}

func EventsEmit(name string, args ...any) {
	// Notify WebSocket clients
	globalBus.Emit(name, args...)
	// Notify in-process listeners
	globalListeners.mu.RLock()
	fns := globalListeners.listeners[name]
	globalListeners.mu.RUnlock()
	for _, fn := range fns {
		go fn(args...)
	}
}
