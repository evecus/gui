package bridge

import (
	"context"
	"embed"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"slices"
	"strings"

	sysruntime "runtime"

	"gopkg.in/yaml.v3"
)

var Config = &AppConfig{}

var Env = &EnvResult{
	IsStartup:    true,
	PreventExit:  true,
	FromTaskSch:  false,
	WebviewPath:  "",
	AppName:      "",
	AppVersion:   "v1.23.1",
	BasePath:     "",
	OS:           sysruntime.GOOS,
	ARCH:         sysruntime.GOARCH,
	IsPrivileged: false,
}

func NewApp() *App {
	return &App{
		Ctx: context.Background(),
	}
}

func CreateApp(fs embed.FS) *App {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	Env.BasePath = filepath.ToSlash(filepath.Dir(exePath))
	Env.AppName = filepath.Base(exePath)

	if slices.Contains(os.Args, "tasksch") {
		Env.FromTaskSch = true
	}

	if priv, err := IsPrivileged(); err == nil {
		Env.IsPrivileged = priv
	}

	app := NewApp()

	if Env.OS == "darwin" {
		createMacOSSymlink()
	}

	extractEmbeddedFiles(fs)
	loadConfig()

	return app
}

func (a *App) IsStartup() bool {
	if Env.IsStartup {
		Env.IsStartup = false
		return true
	}
	return false
}

func (a *App) ExitApp() {
	log.Printf("ExitApp (web mode: no-op)")
}

func (a *App) RestartApp() FlagResult {
	log.Printf("RestartApp")
	exePath := Env.BasePath + "/" + Env.AppName

	cmd := exec.Command(exePath)
	SetCmdWindowHidden(cmd)

	if err := cmd.Start(); err != nil {
		return FlagResult{false, err.Error()}
	}

	return FlagResult{true, "Success"}
}

func (a *App) GetEnv(key string) any {
	log.Printf("GetEnv: %s", key)
	if key != "" {
		return os.Getenv(key)
	}
	return EnvResult{
		AppName:      Env.AppName,
		AppVersion:   Env.AppVersion,
		BasePath:     Env.BasePath,
		OS:           Env.OS,
		ARCH:         Env.ARCH,
		IsPrivileged: Env.IsPrivileged,
	}
}

func (a *App) GetInterfaces() FlagResult {
	log.Printf("GetInterfaces")

	interfaces, err := net.Interfaces()
	if err != nil {
		return FlagResult{false, err.Error()}
	}

	var interfaceNames []string
	for _, inter := range interfaces {
		interfaceNames = append(interfaceNames, inter.Name)
	}

	return FlagResult{true, strings.Join(interfaceNames, "|")}
}

func (a *App) ShowMainWindow() {
	log.Printf("ShowMainWindow (no-op in web mode)")
}

func createMacOSSymlink() {
	user, _ := user.Current()
	linkPath := Env.BasePath + "/data"
	appPath := "/Users/" + user.Username + "/Library/Application Support/" + Env.AppName
	os.MkdirAll(appPath, os.ModePerm)
	os.Symlink(appPath, linkPath)
}

func extractEmbeddedFiles(fs embed.FS) {
	iconSrc := "frontend/dist/icons"
	iconDst := "data/.cache/icons"
	imgSrc := "frontend/dist/imgs"
	imgDst := "data/.cache/imgs"

	os.MkdirAll(GetPath(iconDst), os.ModePerm)
	os.MkdirAll(GetPath(imgDst), os.ModePerm)

	extractFiles(fs, iconSrc, iconDst)
	extractFiles(fs, imgSrc, imgDst)
}

func extractFiles(fs embed.FS, srcDir, dstDir string) {
	files, _ := fs.ReadDir(srcDir)
	for _, file := range files {
		fileName := file.Name()
		dstPath := GetPath(dstDir + "/" + fileName)
		if _, err := os.Stat(dstPath); os.IsNotExist(err) {
			log.Printf("InitResources [%s]: %s", dstDir, fileName)
			data, _ := fs.ReadFile(srcDir + "/" + fileName)
			if err := os.WriteFile(dstPath, data, os.ModePerm); err != nil {
				log.Printf("Error writing file %s: %v", dstPath, err)
			}
		}
	}
}

func loadConfig() {
	b, err := os.ReadFile(Env.BasePath + "/data/user.yaml")
	if err == nil {
		yaml.Unmarshal(b, &Config)
	}

	if Config.Width == 0 {
		Config.Width = 800
	}
	if Config.Height == 0 {
		Config.Height = 540
	}
}
