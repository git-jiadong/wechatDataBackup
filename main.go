package main

import (
	"embed"
	"io"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"gopkg.in/natefinch/lumberjack.v2"
)

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// log output format
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

func main() {
	logJack := &lumberjack.Logger{
		Filename:   "./app.log",
		MaxSize:    5,
		MaxBackups: 1,
		MaxAge:     30,
		Compress:   false,
	}
	defer logJack.Close()

	multiWriter := io.MultiWriter(logJack, os.Stdout)
	// 设置日志输出目标为文件
	log.SetOutput(multiWriter)
	log.Println("====================== wechatDataBackup ======================")
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "wechatDataBackup",
		MinWidth:  800,
		MinHeight: 600,
		Width:     1024,
		Height:    768,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.FLoader,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Frameless: true,
	})

	if err != nil {
		log.Println("Error:", err.Error())
	}
}
