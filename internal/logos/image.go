package logos

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// MinEdge is the smallest icon worth keeping (a 16 px favicon, drawn 4x).
const MinEdge = 16

// ErrBlank is an icon with nothing on it: fully transparent, or one flat
// colour. Some sites serve that as a placeholder.
var ErrBlank = errors.New("logos: icon is blank")

// ErrInvisible is an icon that would vanish on the white logo ground: white
// or near-white marks on transparency, the variant some sites make for dark
// browser tabs.
var ErrInvisible = errors.New("logos: icon is white on transparency and would vanish on the white logo ground")

// minContrast is how far (0-255) from white the darkest channel of at least
// one pixel, composited over white, must be for an icon to show.
const minContrast = 48

// Check rejects icons that are too small, blank, or invisible on white.
func Check(img image.Image) error {
	b := img.Bounds()
	if b.Dx() < MinEdge || b.Dy() < MinEdge {
		return fmt.Errorf("logos: icon is %dx%d, under %d px", b.Dx(), b.Dy(), MinEdge)
	}
	first := color.NRGBAModel.Convert(img.At(b.Min.X, b.Min.Y)).(color.NRGBA)
	visible, varied, shows := false, false, false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A >= 16 {
				visible = true
			}
			if c != first {
				varied = true
			}
			// Over white, a channel v at alpha a shows as 255-a*(255-v).
			if int(255-min(c.R, c.G, c.B))*int(c.A)/255 >= minContrast {
				shows = true
			}
			if visible && varied && shows {
				return nil
			}
		}
	}
	if !visible || !varied {
		return ErrBlank
	}
	return ErrInvisible
}

// Normalize fits img inside a Size×Size transparent square, centred, with
// area-weighted resampling (exact pixel repetition for integer upscales such
// as a 16 or 32 px favicon).
func Normalize(img image.Image) *image.NRGBA {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	scale := math.Min(float64(Size)/float64(sw), float64(Size)/float64(sh))
	dw := max(1, min(Size, int(math.Round(float64(sw)*scale))))
	dh := max(1, min(Size, int(math.Round(float64(sh)*scale))))
	// Premultiplied source, so transparent pixels do not bleed their colour.
	src := make([][4]float64, sw*sh)
	for y := 0; y < sh; y++ {
		for x := 0; x < sw; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			a := float64(c.A) / 255
			src[y*sw+x] = [4]float64{float64(c.R) * a, float64(c.G) * a, float64(c.B) * a, a}
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, Size, Size))
	ox, oy := (Size-dw)/2, (Size-dh)/2
	fx, fy := float64(sw)/float64(dw), float64(sh)/float64(dh)
	for ty := 0; ty < dh; ty++ {
		y0, y1 := float64(ty)*fy, float64(ty+1)*fy
		for tx := 0; tx < dw; tx++ {
			x0, x1 := float64(tx)*fx, float64(tx+1)*fx
			var acc [4]float64
			var total float64
			for sy := int(y0); sy < sh && float64(sy) < y1; sy++ {
				wy := math.Min(y1, float64(sy+1)) - math.Max(y0, float64(sy))
				if wy <= 0 {
					continue
				}
				for sx := int(x0); sx < sw && float64(sx) < x1; sx++ {
					wx := math.Min(x1, float64(sx+1)) - math.Max(x0, float64(sx))
					if wx <= 0 {
						continue
					}
					w := wx * wy
					p := src[sy*sw+sx]
					for i := range acc {
						acc[i] += p[i] * w
					}
					total += w
				}
			}
			if total == 0 {
				continue
			}
			a := acc[3] / total
			var c color.NRGBA
			if a > 0 {
				c = color.NRGBA{
					R: clamp8(acc[0] / total / a),
					G: clamp8(acc[1] / total / a),
					B: clamp8(acc[2] / total / a),
					A: clamp8(a * 255),
				}
			}
			out.SetNRGBA(ox+tx, oy+ty, c)
		}
	}
	return out
}

func clamp8(v float64) uint8 {
	v = math.Round(v)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// Encode writes the normalized logo as a compact PNG.
func Encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Prepare is Decode, Check, Normalize and Encode in one step. It also returns
// the edge of the source icon (the smaller side), so callers can prefer the
// sharpest of several candidates.
func Prepare(body []byte) (pngBytes []byte, edge int, err error) {
	img, err := Decode(body)
	if err != nil {
		return nil, 0, err
	}
	if err := Check(img); err != nil {
		return nil, 0, err
	}
	b := img.Bounds()
	out, err := Encode(Normalize(img))
	if err != nil {
		return nil, 0, err
	}
	return out, min(b.Dx(), b.Dy()), nil
}
