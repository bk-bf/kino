package preview

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// renderKitty encodes an image using the Kitty terminal graphics protocol.
// cols and rows are the terminal cell dimensions of the display area; they are
// passed as c= and r= parameters so Kitty allocates the correct cell space.
func renderKitty(img image.Image, cols, rows int) string {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ""
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	var sb strings.Builder
	// Kitty APC: \x1b_G<params>;<data>\x1b\\
	// a=T → transmit+display, f=100 → PNG, m=0 → final chunk
	// c=cols,r=rows → number of terminal cells to occupy
	chunkSize := 4096
	first := true
	for len(encoded) > 0 {
		chunk := encoded
		more := 0
		if len(encoded) > chunkSize {
			chunk = encoded[:chunkSize]
			encoded = encoded[chunkSize:]
			more = 1
		} else {
			encoded = ""
		}

		var params string
		if first {
			params = fmt.Sprintf("a=T,f=100,c=%d,r=%d,m=%d", cols, rows, more)
			first = false
		} else {
			params = fmt.Sprintf("m=%d", more)
		}
		sb.WriteString("\x1b_G")
		sb.WriteString(params)
		sb.WriteString(";")
		sb.WriteString(chunk)
		sb.WriteString("\x1b\\")
	}
	return sb.String()
}

// renderITerm2 encodes image bytes using the iTerm2 inline image protocol.
// cols and rows are passed as width/height hints (in character cells).
func renderITerm2(data []byte, cols, rows int) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	// \x1b]1337;File=inline=1;width=Nc;height=Nc:<base64>\x07
	return fmt.Sprintf("\x1b]1337;File=inline=1;width=%dc;height=%dc:%s\x07", cols, rows, encoded)
}

// renderSixel converts an image to a Sixel escape sequence.
// This is a minimal implementation: quantise to 256 colours, emit DCS header.
func renderSixel(img image.Image) string {
	bounds := img.Bounds()
	w := bounds.Max.X - bounds.Min.X
	h := bounds.Max.Y - bounds.Min.Y
	if w == 0 || h == 0 {
		return ""
	}

	// Quantise to a fixed palette of 256 colours (6×6×6 + 40 greys)
	palette := buildPalette()
	paletteMap := make(map[color.Color]int, len(palette))
	for i, c := range palette {
		paletteMap[c] = i
	}

	// Map each pixel to a palette index
	indexed := make([][]int, h)
	for y := range h {
		indexed[y] = make([]int, w)
		for x := range w {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			indexed[y][x] = nearestPalette(c, palette)
		}
	}

	var sb strings.Builder
	// DCS intro: P0;0;8q (aspect ratio 1:1, background transparent, pixel size 8)
	sb.WriteString("\x1bPq")

	// emit palette definitions
	for i, c := range palette {
		r32, g32, b32, _ := c.RGBA()
		r := int(r32 * 100 / 0xFFFF)
		g := int(g32 * 100 / 0xFFFF)
		b := int(b32 * 100 / 0xFFFF)
		sb.WriteString(fmt.Sprintf("#%d;2;%d;%d;%d", i, r, g, b))
	}

	// Emit image data: 6 rows per sixel band
	for band := 0; band*6 < h; band++ {
		// For each colour, build a sixel string for this band
		colorData := make(map[int][]byte, len(palette))
		for ci := range palette {
			colorData[ci] = make([]byte, w)
		}
		for row := 0; row < 6; row++ {
			y := band*6 + row
			if y >= h {
				break
			}
			for x := 0; x < w; x++ {
				ci := indexed[y][x]
				colorData[ci][x] |= 1 << row
			}
		}
		// emit each colour that has any set pixels
		first := true
		for ci, pixels := range colorData {
			hasPixel := false
			for _, p := range pixels {
				if p != 0 {
					hasPixel = true
					break
				}
			}
			if !hasPixel {
				continue
			}
			if !first {
				sb.WriteByte('$') // carriage-return within band
			}
			first = false
			sb.WriteString(fmt.Sprintf("#%d", ci))
			// RLE-encode the sixel values
			prev := pixels[0]
			count := 1
			for x := 1; x < w; x++ {
				if pixels[x] == prev {
					count++
				} else {
					writeSixelRun(&sb, prev, count)
					prev = pixels[x]
					count = 1
				}
			}
			writeSixelRun(&sb, prev, count)
		}
		sb.WriteByte('-') // next band
	}

	sb.WriteString("\x1b\\") // ST: string terminator
	sb.WriteByte('\n')
	return sb.String()
}

func writeSixelRun(sb *strings.Builder, val byte, count int) {
	ch := byte('?') + val
	if count == 1 {
		sb.WriteByte(ch)
	} else {
		sb.WriteString(fmt.Sprintf("!%d%c", count, ch))
	}
}

// buildPalette returns a 216-colour cube palette.
func buildPalette() []color.Color {
	palette := make([]color.Color, 0, 216)
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				palette = append(palette, color.RGBA{
					R: uint8(r * 51),
					G: uint8(g * 51),
					B: uint8(b * 51),
					A: 255,
				})
			}
		}
	}
	return palette
}

func nearestPalette(c color.Color, palette []color.Color) int {
	r0, g0, b0, _ := c.RGBA()
	best := 0
	bestDist := uint32(1<<31 - 1)
	for i, pc := range palette {
		r1, g1, b1, _ := pc.RGBA()
		dr := diff(r0, r1)
		dg := diff(g0, g1)
		db := diff(b0, b1)
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			best = i
		}
	}
	return best
}

func diff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
