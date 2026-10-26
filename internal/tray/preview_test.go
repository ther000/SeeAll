package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"testing"
)

// TestIconSheet writes every icon state side by side into
// .preview/probe-icons.png, for checking them by eye.
func TestIconSheet(t *testing.T) {
	accent := func(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 255} }
	states := []State{
		{Level: -1, Waiting: true, Dark: true},
		{Level: -1, Offline: true, Dark: true},
		{Level: 5, Color: accent(0xF8, 0x71, 0x71), Dark: true},
		{Level: 24, Color: accent(0xFB, 0xBF, 0x24), Dark: true},
		{Level: 95, Color: accent(0x34, 0xD3, 0x99), Dark: true},
		{Level: 95, Color: accent(0x0F, 0x9D, 0x63), Dark: false},
	}
	const gap = 8
	const zoom = 4
	width := len(states)*(Size+gap) - gap
	row := image.NewNRGBA(image.Rect(0, 0, width, Size))
	draw.Draw(row, row.Bounds(), image.NewUniform(accent(0x18, 0x18, 0x1B)), image.Point{}, draw.Src)
	for i, st := range states {
		img, err := png.Decode(bytes.NewReader(Icon(st)))
		if err != nil {
			t.Fatal(err)
		}
		draw.Draw(row, image.Rect(i*(Size+gap), 0, i*(Size+gap)+Size, Size), img, image.Point{}, draw.Over)
	}
	// The last state is the light one: paint a light taskbar behind it, so
	// its dark ink shows the way Windows would draw it.
	lightTop := len(states)*(Size+gap) - gap - Size
	lightBack := accent(0xF4, 0xF4, 0xF5)
	draw.Draw(row, image.Rect(lightTop, 0, lightTop+Size, Size), image.NewUniform(lightBack), image.Point{}, draw.Src)
	img, err := png.Decode(bytes.NewReader(Icon(states[len(states)-1])))
	if err != nil {
		t.Fatal(err)
	}
	draw.Draw(row, image.Rect(lightTop, 0, lightTop+Size, Size), img, image.Point{}, draw.Over)
	sheet := image.NewNRGBA(image.Rect(0, 0, width*zoom, Size*zoom))
	for y := 0; y < Size*zoom; y++ {
		for x := 0; x < width*zoom; x++ {
			sheet.Set(x, y, row.At(x/zoom, y/zoom))
		}
	}
	if err := os.MkdirAll("../../.preview", 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("../../.preview/probe-icons.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
}
