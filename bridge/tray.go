package bridge

import "log"

// Tray is a no-op in web/headless mode.

func (a *App) UpdateTray(tray TrayContent) {
	log.Printf("UpdateTray (no-op in web mode)")
}

func (a *App) UpdateTrayMenus(menus []MenuItem) {
	log.Printf("UpdateTrayMenus (no-op in web mode)")
}

func (a *App) UpdateTrayAndMenus(tray TrayContent, menus []MenuItem) {
	log.Printf("UpdateTrayAndMenus (no-op in web mode)")
}
