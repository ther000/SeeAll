// Command icon draws the app's icon, resources/icon.png, from the same code
// the tray uses: a rounded square in the panel's accent color with a lit
// ring in it, so the icon and the tray always agree.
//
//	go run ./cmd/icon
package main

import (
	"image/png"
	"log"
	"os"

	"seeall/internal/tray"
)

func main() {
	img := tray.AppIcon(1024)
	if err := os.MkdirAll("resources", 0o755); err != nil {
		log.Fatal(err)
	}
	f, err := os.Create("resources/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote resources/icon.png")
}
