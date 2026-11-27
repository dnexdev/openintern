package logos

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
)

// ErrUnsupported is a body that is not a raster image we can decode: SVG,
// WebP, an HTML error page served with a 200, and so on.
var ErrUnsupported = errors.New("logos: not a PNG, JPEG, GIF or ICO image")

// maxEdge rejects absurd declared dimensions before allocating.
const maxEdge = 4096

// Decode reads PNG, JPEG, GIF and ICO (PNG or BMP entries; the largest entry
// wins). Anything else is ErrUnsupported.
func Decode(b []byte) (image.Image, error) {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return checked(png.DecodeConfig, png.Decode, b)
	case bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff}):
		return checked(jpeg.DecodeConfig, jpeg.Decode, b)
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return checked(gif.DecodeConfig, gif.Decode, b)
	case len(b) >= 6 && binary.LittleEndian.Uint16(b[0:]) == 0 && binary.LittleEndian.Uint16(b[2:]) == 1:
		return decodeICO(b)
	}
	return nil, ErrUnsupported
}

func checked(cfg func(r io.Reader) (image.Config, error), dec func(r io.Reader) (image.Image, error), b []byte) (image.Image, error) {
	c, err := cfg(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	if c.Width <= 0 || c.Height <= 0 || c.Width > maxEdge || c.Height > maxEdge {
		return nil, fmt.Errorf("logos: image is %dx%d", c.Width, c.Height)
	}
	return dec(bytes.NewReader(b))
}

// decodeICO picks the largest entry (then the deepest colour) and decodes it.
func decodeICO(b []byte) (image.Image, error) {
	n := int(binary.LittleEndian.Uint16(b[4:]))
	if n == 0 || len(b) < 6+16*n {
		return nil, errors.New("logos: truncated ICO directory")
	}
	type entry struct{ w, h, bpp, size, off int }
	var best *entry
	for i := 0; i < n; i++ {
		d := b[6+16*i:]
		e := entry{
			w:    int(d[0]),
			h:    int(d[1]),
			bpp:  int(binary.LittleEndian.Uint16(d[6:])),
			size: int(binary.LittleEndian.Uint32(d[8:])),
			off:  int(binary.LittleEndian.Uint32(d[12:])),
		}
		if e.w == 0 {
			e.w = 256
		}
		if e.h == 0 {
			e.h = 256
		}
		if e.off < 0 || e.size <= 0 || e.off+e.size > len(b) || e.off+e.size < e.off {
			continue
		}
		if best == nil || e.w*e.h > best.w*best.h || (e.w*e.h == best.w*best.h && e.bpp > best.bpp) {
			e := e
			best = &e
		}
	}
	if best == nil {
		return nil, errors.New("logos: ICO has no readable entry")
	}
	data := b[best.off : best.off+best.size]
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return checked(png.DecodeConfig, png.Decode, data)
	}
	return decodeDIB(data)
}

// decodeDIB reads the BMP body of an ICO entry: a BITMAPINFOHEADER, a
// palette for 1/4/8-bit images, the colour rows bottom-up, then a 1-bit AND
// mask (1 = transparent). 32-bit entries carry alpha; when every alpha byte is
// zero the AND mask is used instead, as Windows does.
func decodeDIB(d []byte) (image.Image, error) {
	if len(d) < 40 {
		return nil, errors.New("logos: truncated ICO bitmap header")
	}
	hdr := int(binary.LittleEndian.Uint32(d[0:]))
	w := int(int32(binary.LittleEndian.Uint32(d[4:])))
	h := int(int32(binary.LittleEndian.Uint32(d[8:]))) / 2 // colour rows + mask rows
	bpp := int(binary.LittleEndian.Uint16(d[14:]))
	compression := binary.LittleEndian.Uint32(d[16:])
	colorsUsed := int(binary.LittleEndian.Uint32(d[32:]))
	if hdr < 40 || hdr > len(d) || w <= 0 || h <= 0 || w > 256 || h > 256 {
		return nil, fmt.Errorf("logos: bad ICO bitmap header (%dx%d)", w, h)
	}
	if compression != 0 && !(compression == 3 && bpp == 32) {
		return nil, fmt.Errorf("logos: compressed ICO bitmap (%d)", compression)
	}
	off := hdr
	if compression == 3 {
		off += 12 // BI_BITFIELDS masks; 32-bit icons use the standard BGRA order
	}
	var palette []color.NRGBA
	switch bpp {
	case 1, 4, 8:
		count := colorsUsed
		if count == 0 {
			count = 1 << bpp
		}
		if off+4*count > len(d) {
			return nil, errors.New("logos: truncated ICO palette")
		}
		for i := 0; i < count; i++ {
			p := d[off+4*i:]
			palette = append(palette, color.NRGBA{R: p[2], G: p[1], B: p[0], A: 255})
		}
		off += 4 * count
	case 24, 32:
	default:
		return nil, fmt.Errorf("logos: %d-bit ICO bitmap", bpp)
	}
	stride := ((w*bpp + 31) / 32) * 4
	maskStride := ((w + 31) / 32) * 4
	if off+stride*h > len(d) {
		return nil, errors.New("logos: truncated ICO pixels")
	}
	mask := d[off+stride*h:]
	hasMask := len(mask) >= maskStride*h
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	anyAlpha := false
	for y := 0; y < h; y++ {
		row := d[off+(h-1-y)*stride:]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			switch bpp {
			case 32:
				p := row[4*x:]
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: p[3]}
				if p[3] != 0 {
					anyAlpha = true
				}
			case 24:
				p := row[3*x:]
				c = color.NRGBA{R: p[2], G: p[1], B: p[0], A: 255}
			default:
				bit := x * bpp
				v := int(row[bit/8]>>(8-bpp-bit%8)) & (1<<bpp - 1)
				if v < len(palette) {
					c = palette[v]
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	if bpp != 32 || !anyAlpha {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := img.NRGBAAt(x, y)
				c.A = 255
				if hasMask {
					m := mask[(h-1-y)*maskStride+x/8]
					if m>>(7-x%8)&1 == 1 {
						c.A = 0
					}
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}
	return img, nil
}
