package main

import (
	"embed"
	"guiforcores/bridge"
	"log"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed frontend/dist/favicon.ico
var icon []byte

func main() {
	app := bridge.CreateApp(assets)
	log.Printf("GUI.for.SingBox Web Server starting on :9080")
	if err := app.RunWebServer(":9080", assets); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
