package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var port = flag.String("port", "", "The port that should be used to create the WebSocket")
var pluginUUID = flag.String("pluginUUID", "", "A unique identifier string that should be used to register the plugin once the WebSocket is opened")
var registerEvent = flag.String("registerEvent", "", "Registration event")
var info = flag.String("info", "", "A stringified json containing the Stream Deck application information and devices information")

func main() {
	// make sure files are read relative to exe
	err := os.Chdir(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatalf("Unable to chdir: %v", err)
	}

	var appdata string
	var logpath string
	if runtime.GOOS == "windows" {
		appdata = os.Getenv("APPDATA")
		logpath = filepath.Join(appdata, "Elgato/StreamDeck/Plugins/com.exension.stocks.sdPlugin/stocks.log")
	} else {
		appdata = os.Getenv("HOME")
		logpath = filepath.Join(appdata, "Library/Application Support/com.elgato.StreamDeck/Plugins/com.exension.stocks.sdPlugin/stocks.log")
	}
	err = os.MkdirAll(filepath.Dir(logpath), 0755)
	if err != nil {
		log.Fatalf("Create log directory: %v", err)
	}
	f, err := os.OpenFile(logpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("OpenFile Log: %v", err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Fatalf("File Close: %v", err)
		}
	}()
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()

	p := newPlugin(*port, *pluginUUID, *registerEvent, *info)
	p.Run()
}
