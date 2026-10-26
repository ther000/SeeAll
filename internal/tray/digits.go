package tray

import (
	"image"
	"image/color"
	"math"
)

// digitGlyphs are 4×7 bitmaps for the characters the icon shows. The icon is
// 32 pixels across; a two-digit number at this size is 14 pixels tall, which
// is what makes it readable on a taskbar, and three digits shrink to fit
// inside the ring.
var digitGlyphs = map[rune][7]string{
	'0': {"1111", "1001", "1001", "1001", "1001", "1001", "1111"},
	'1': {"0010", "0110", "0010", "0010", "0010", "0010", "0111"},
	'2': {"1111", "0001", "0001", "1111", "1000", "1000", "1111"},
	'3': {"1111", "0001", "0001", "0111", "0001", "0001", "1111"},
	'4': {"1001", "1001", "1001", "1111", "0001", "0001", "0001"},
	'5': {"1111", "1000", "1000", "1111", "0001", "0001", "1111"},
	'6': {"1111", "1000", "1000", "1111", "1001", "1001", "1111"},
	'7': {"1111", "0001", "0001", "0010", "0010", "0010", "0010"},
	'8': {"1111", "1001", "1001", "1111", "1001", "1001", "1111"},
	'9': {"1111", "1001", "1001", "1111", "0001", "0001", "1111"},
}

const (
	glyphWidth  = 4
	glyphHeight = 7
)

// drawNumber centers a number in the ring's hole, sized to fit it.
func drawNumber(img *image.NRGBA, text string, ink color.NRGBA, size int) {
	if text == "" {
		return
	}
	// One column of gap between glyphs, and two pixels of margin inside the
	// ring.
	const gap = 1
	units := len(text)*glyphWidth + (len(text)-1)*gap
	avail := float64(size) - 2*innerMargin(size)
	scale := math.Floor(avail / float64(units))
	if maxScale := math.Floor(avail / glyphHeight); scale > maxScale {
		scale = maxScale
	}
	if scale < 1 {
		scale = 1
	}
	w := float64(units) * scale
	h := glyphHeight * scale
	ox := (float64(size) - w) / 2
	oy := (float64(size) - h) / 2

	for i, r := range text {
		glyph, ok := digitGlyphs[r]
		if !ok {
			continue
		}
		for row := 0; row < glyphHeight; row++ {
			for col := 0; col < glyphWidth; col++ {
				if glyph[row][col] != '1' {
					continue
				}
				x := ox + (float64(i*(glyphWidth+gap))+float64(col))*scale
				y := oy + float64(row)*scale
				fillBox(img, x, y, scale, scale, ink, size)
			}
		}
	}
}

// innerMargin is the room the ring takes on each side, so the digits stay in
// its hole: the arc's radius is size/2 - 3.5 and its stroke 2.8 wide, which
// leaves about a third of the icon for the number.
func innerMargin(size int) float64 { return float64(size)*0.175 + 1 }

func fillBox(img *image.NRGBA, x, y, w, h float64, c color.NRGBA, size int) {
	for py := int(math.Floor(y)); py < int(math.Ceil(y+h)); py++ {
		for px := int(math.Floor(x)); px < int(math.Ceil(x+w)); px++ {
			if px < 0 || py < 0 || px >= size || py >= size {
				continue
			}
			cx := math.Min(float64(px)+1, x+w) - math.Max(float64(px), x)
			cy := math.Min(float64(py)+1, y+h) - math.Max(float64(py), y)
			cover := cx * cy
			if cover <= 0 {
				continue
			}
			blendAt(img, px, py, c, math.Min(cover, 1))
		}
	}
}
