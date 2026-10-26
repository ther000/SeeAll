// Package tray draws the notification-area icon and works out where the
// panel goes: next to the icon, inside the work area.
package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/egoist/mygo"
)

// Size is the icon's size in pixels: 32 is what Windows shows at 100% and
// stays sharp at 200%.
const Size = 32

// State is what the icon shows: a battery level, or that there is none.
type State struct {
	// Level is 0..100; a negative value means no reading yet.
	Level float64
	// Color is the ring's color: the reading's severity.
	Color color.NRGBA
	// Waiting draws three dots instead of a level: a source that has not
	// reported.
	Waiting bool
	// Offline draws an empty ring in gray.
	Offline bool
	// Dark tells whether the taskbar (or the macOS menu bar) is dark, so
	// the icon's ink stays visible on it.
	Dark bool
}

// Icon renders the tray icon: the number in the middle and a ring around it,
// lit to the level. On Windows the taskbar shows a PNG as it is, so the ink
// is chosen for the taskbar's appearance.
func Icon(s State) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, Size, Size))
	ink := color.NRGBA{R: 0xF4, G: 0xF4, B: 0xF5, A: 255}
	track := color.NRGBA{R: 0x5A, G: 0x5A, B: 0x64, A: 255}
	if !s.Dark {
		ink = color.NRGBA{R: 0x18, G: 0x18, B: 0x1B, A: 255}
		track = color.NRGBA{R: 0xBE, G: 0xBE, B: 0xC6, A: 255}
	}

	ringColor := s.Color
	if ringColor.A == 0 {
		ringColor = ink
	}
	cx, cy := float64(Size)/2, float64(Size)/2
	radius := float64(Size)/2 - 3.5
	const width = 2.8

	strokeArc(img, cx, cy, radius, 0, 360, width, track)

	switch {
	case s.Waiting:
		for i := -1; i <= 1; i++ {
			fillDisc(img, cx+float64(i)*5.2, cy, 1.8, ink)
		}
	case s.Offline || s.Level < 0:
		strokeArc(img, cx, cy, radius, 0, 360, width,
			color.NRGBA{R: 0x8A, G: 0x8A, B: 0x92, A: 255})
	default:
		level := math.Min(math.Max(s.Level, 0), 100)
		sweep := 360 * level / 100
		if sweep > 0 && sweep < 10 {
			sweep = 10
		}
		strokeArc(img, cx, cy, radius, 0, sweep, width, ringColor)
		drawNumber(img, numberText(level), ink, Size)
	}
	return encode(img)
}

// numberText writes the level the way the icon shows it: whole percent.
func numberText(level float64) string {
	n := int(math.Round(level))
	if n <= 0 {
		return "0"
	}
	if n > 100 {
		n = 100
	}
	return itoa(n)
}

// strokeArc paints an arc centered on (cx, cy), from startDeg to endDeg,
// clockwise from twelve o'clock.
func strokeArc(img *image.NRGBA, cx, cy, radius, startDeg, endDeg, width float64, c color.NRGBA) {
	steps := int(math.Max(24, math.Abs(endDeg-startDeg)))
	for i := 0; i <= steps; i++ {
		deg := startDeg + (endDeg-startDeg)*float64(i)/float64(steps)
		rad := deg * math.Pi / 180
		fillDisc(img, cx+radius*math.Sin(rad), cy-radius*math.Cos(rad), width/2, c)
	}
}

// fillDisc paints a round point with one pixel of antialiasing, which is
// what an arc stroke is made of.
func fillDisc(img *image.NRGBA, cx, cy, r float64, c color.NRGBA) {
	size := img.Rect.Dx()
	x0 := int(math.Floor(cx - r - 1))
	x1 := int(math.Ceil(cx + r + 1))
	y0 := int(math.Floor(cy - r - 1))
	y1 := int(math.Ceil(cy + r + 1))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			if x < 0 || y < 0 || x >= size || y >= img.Rect.Dy() {
				continue
			}
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			cover := r - math.Hypot(dx, dy) + 0.5
			if cover <= 0 {
				continue
			}
			blendAt(img, x, y, c, math.Min(cover, 1))
		}
	}
}

// blendAt paints c over the pixel at x, y with alpha, in straight alpha.
func blendAt(img *image.NRGBA, x, y int, c color.NRGBA, alpha float64) {
	if alpha <= 0 || x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		return
	}
	i := img.PixOffset(x, y)
	dst := img.Pix[i : i+4 : i+4]
	sa := alpha
	da := float64(dst[3]) / 255
	outA := sa + da*(1-sa)
	if outA <= 0 {
		return
	}
	src := [3]uint8{c.R, c.G, c.B}
	for k := 0; k < 3; k++ {
		out := (float64(src[k])*sa + float64(dst[k])*da*(1-sa)) / outA
		dst[k] = uint8(math.Round(out))
	}
	dst[3] = uint8(math.Round(outA * 255))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func encode(img *image.NRGBA) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

// AppIcon draws the app icon: a rounded square in the panel's accent color
// with the ring the tray shows, so the two never disagree.
func AppIcon(size int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	accent := color.NRGBA{R: 0x0D, G: 0x94, B: 0x88, A: 255}
	fillRoundRect(img, float64(size), accent)

	white := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	soft := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x3D}
	cx, cy := float64(size)/2, float64(size)/2
	radius := float64(size) * 0.30
	const width = 0.072

	strokeAppArc(img, cx, cy, radius, 0, 360, float64(size)*width, soft)
	strokeAppArc(img, cx, cy, radius, 0, 262, float64(size)*width, white)
	return img
}

// fillRoundRect fills a size×size square with rounded corners.
func fillRoundRect(img *image.NRGBA, size float64, c color.NRGBA) {
	radius := size * 0.22
	n := int(size)
	for py := 0; py < n; py++ {
		for px := 0; px < n; px++ {
			fx := float64(px) + 0.5
			fy := float64(py) + 0.5
			dx := math.Max(math.Abs(fx-size/2)-(size/2-radius), 0)
			dy := math.Max(math.Abs(fy-size/2)-(size/2-radius), 0)
			cover := 0.5 - (math.Hypot(dx, dy) - radius)
			if cover <= 0 {
				continue
			}
			blendAt(img, px, py, c, math.Min(cover, 1))
		}
	}
}

func strokeAppArc(img *image.NRGBA, cx, cy, radius, startDeg, endDeg, width float64, c color.NRGBA) {
	steps := int(math.Max(128, math.Abs(endDeg-startDeg)))
	for i := 0; i <= steps; i++ {
		deg := startDeg + (endDeg-startDeg)*float64(i)/float64(steps)
		rad := deg * math.Pi / 180
		fillDisc(img, cx+radius*math.Sin(rad), cy-radius*math.Cos(rad), width/2, c)
	}
}

var _ = mygo.App
