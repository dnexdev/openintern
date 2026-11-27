package site

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"net/http"
	"sync"
)

func (h *handler) serveStatic(w http.ResponseWriter, name, contentType, cache string) {
	data, err := fs.ReadFile(Static(), name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", cache)
	_, _ = w.Write(data)
}

func (h *handler) iconSVG(w http.ResponseWriter, r *http.Request) {
	h.serveStatic(w, "icon.svg", "image/svg+xml", "public, max-age=14400, must-revalidate")
}

// ogImage serves the 1200×630 card used by Open Graph and Twitter. It is a
// static PNG rendered once from static/og.html (see the README there).
func (h *handler) ogImage(w http.ResponseWriter, r *http.Request) {
	h.serveStatic(w, "og.png", "image/png", "public, max-age=0, must-revalidate")
}

var (
	faviconOnce sync.Once
	faviconICO  []byte
)

func (h *handler) favicon(w http.ResponseWriter, r *http.Request) {
	faviconOnce.Do(func() { faviconICO = drawFavicon() })
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "public, max-age=14400, must-revalidate")
	_, _ = w.Write(faviconICO)
}

// drawFavicon draws the brand mark of static/icon.svg (a sheet with two green
// bars) at 32×32 and wraps the PNG in an ICO container for old clients.
func drawFavicon() []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	paper := color.NRGBA{0xfb, 0xfc, 0xfa, 0xff}
	ink := color.NRGBA{0x1e, 0x24, 0x20, 0xff}
	bar := color.NRGBA{0x4f, 0x9a, 0x5a, 0xff}
	fill := func(x0, y0, x1, y1 int, c color.NRGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
	}
	fill(3, 3, 29, 29, ink)
	fill(6, 6, 26, 26, paper)
	fill(6, 10, 26, 14, bar)
	fill(6, 18, 26, 22, bar)
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)

	var out bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&out, le, [3]uint16{0, 1, 1}) // reserved, type icon, one image
	out.Write([]byte{size, size, 0, 0})            // width, height, palette, reserved
	_ = binary.Write(&out, le, uint16(1))          // colour planes
	_ = binary.Write(&out, le, uint16(32))         // bits per pixel
	_ = binary.Write(&out, le, uint32(pngBuf.Len()))
	_ = binary.Write(&out, le, uint32(6+16))
	out.Write(pngBuf.Bytes())
	return out.Bytes()
}
