package logos

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/dnexdev/openintern/internal/fetch"
)

func ptr(s string) *string { return &s }

func TestHomepage(t *testing.T) {
	cases := []struct {
		website, careers *string
		want             string
	}{
		{ptr("https://www.stripe.com/jobs"), nil, "https://www.stripe.com/"},
		{nil, ptr("https://careers.example.com/x"), "https://careers.example.com/"},
		{nil, ptr("https://job-boards.greenhouse.io/acme"), ""},
		{nil, ptr("https://acme.wd5.myworkdayjobs.com/External"), ""},
		{nil, ptr("https://ats.rippling.com/acme/jobs"), ""},
		// An ATS vendor's own homepage is a company homepage, not a job board.
		{ptr("https://www.rippling.com"), ptr("https://ats.rippling.com/rippling"), "https://www.rippling.com/"},
		{ptr("https://lever.co/"), nil, "https://lever.co/"},
		{ptr("https://www.ashbyhq.com"), nil, "https://www.ashbyhq.com/"},
		{ptr("https://jobs.lever.co/acme"), ptr("https://acme.com/careers"), "https://acme.com/"},
		{ptr("  https://acme.io  "), nil, "https://acme.io/"},
		{ptr("ftp://acme.io"), nil, ""},
		{ptr("not a url"), nil, ""},
		{nil, nil, ""},
	}
	for _, c := range cases {
		if got := Homepage(c.website, c.careers); got != c.want {
			t.Errorf("Homepage(%v, %v) = %q, want %q", deref(c.website), deref(c.careers), got, c.want)
		}
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestIconLinks(t *testing.T) {
	page := `<!doctype html><html><head>
<link rel="stylesheet" href="/a.css">
<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32.png">
<link rel='icon' sizes='192x192' href='https://cdn.example.net/icon-192.png?a=1&amp;b=2'>
<LINK REL="apple-touch-icon" HREF="touch.png">
<link rel="icon" type="image/svg+xml" href="/icon.svg">
<link rel="mask-icon" href="/mask.svg" color="#000">
<link rel="shortcut icon" href="data:image/png;base64,AAAA">
<link rel="icon" href="/favicon-32.png">
</head><body><link rel="icon" href="/body.png"></body></html>`
	got := IconLinks([]byte(page), "https://www.example.com/en/home")
	want := []Candidate{
		{URL: "https://cdn.example.net/icon-192.png?a=1&b=2", Rel: "icon", Size: 192},
		{URL: "https://www.example.com/en/touch.png", Rel: "apple-touch-icon", Size: 180},
		{URL: "https://www.example.com/favicon-32.png", Rel: "icon", Size: 32},
		{URL: "https://www.example.com/favicon.ico", Rel: "guess"},
	}
	if len(got) != len(want) {
		t.Fatalf("IconLinks = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	// <base href> moves relative links; a page with no icons still gets the
	// /favicon.ico guess.
	got = IconLinks([]byte(`<head><base href="https://static.example.org/s/"><link rel="icon" href="i.png"></head>`), "https://example.org/")
	if len(got) != 3 || got[0].URL != "https://static.example.org/s/i.png" || got[1].URL != "https://static.example.org/apple-touch-icon.png" || got[2].URL != "https://static.example.org/favicon.ico" {
		t.Errorf("base href: %+v", got)
	}
	if got := IconLinks([]byte("<p>hi</p>"), "https://example.org/x"); len(got) != 2 || got[0].URL != "https://example.org/apple-touch-icon.png" || got[1].URL != "https://example.org/favicon.ico" {
		t.Errorf("no icons: %+v", got)
	}
	// The manifest link is reported separately; a declared
	// /apple-touch-icon.png is not guessed again.
	got, manifest := pageIcons([]byte(`<head><link rel="icon" href="/i.svg"><link rel="manifest" href="/site.webmanifest"><link rel="manifest" href="/second.json"><link rel="apple-touch-icon" href="/apple-touch-icon.png"></head>`), "https://example.org/")
	if manifest != "https://example.org/site.webmanifest" {
		t.Errorf("manifest %q", manifest)
	}
	if len(got) != 2 || got[0].URL != "https://example.org/apple-touch-icon.png" || got[0].Rel != "apple-touch-icon" || got[1].URL != "https://example.org/favicon.ico" {
		t.Errorf("declared touch icon: %+v", got)
	}
	if _, manifest := pageIcons([]byte(`<head><link rel="manifest" href=""></head>`), "https://example.org/"); manifest != "" {
		t.Errorf("empty manifest href: %q", manifest)
	}
}

func TestManifestIcons(t *testing.T) {
	m := `{"name":"Acme","icons":[
		{"src":"icons/192.png","sizes":"192x192","type":"image/png"},
		{"src":"/icons/logo.svg","sizes":"any","type":"image/svg+xml"},
		{"src":"icons/512-mask.png","sizes":"512x512","purpose":"maskable"},
		{"src":"icons/mono.png","sizes":"512x512","purpose":"monochrome"},
		{"src":"https://cdn.acme.test/512.png","sizes":"512x512","purpose":"any maskable"},
		{"src":"icons/192.png","sizes":"192x192"}
	]}`
	got := ManifestIcons([]byte(m), "https://acme.test/static/site.webmanifest")
	want := []Candidate{
		{URL: "https://cdn.acme.test/512.png", Rel: "manifest", Size: 512},
		{URL: "https://acme.test/static/icons/192.png", Rel: "manifest", Size: 192},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("ManifestIcons = %+v", got)
	}
	// Only maskable icons: better than nothing.
	got = ManifestIcons([]byte(`{"icons":[{"src":"/m.png","sizes":"256x256","purpose":"maskable"}]}`), "https://acme.test/m.json")
	if len(got) != 1 || got[0].URL != "https://acme.test/m.png" {
		t.Errorf("maskable only: %+v", got)
	}
	if got := ManifestIcons([]byte("<html>not json"), "https://acme.test/m.json"); got != nil {
		t.Errorf("not JSON: %+v", got)
	}
}

// testIcon is a w×h icon: a red disc on transparency, so it is not blank.
func testIcon(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(2*x+1-w), float64(2*y+1-h)
			if dx*dx+dy*dy <= float64(w*w)/2 {
				img.SetNRGBA(x, y, color.NRGBA{200, 30, 30, 255})
			}
		}
	}
	return img
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// icoFile wraps entries (already encoded) in an ICO directory.
func icoFile(entries ...icoEntry) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, [3]uint16{0, 1, uint16(len(entries))})
	off := 6 + 16*len(entries)
	for _, e := range entries {
		buf.WriteByte(byte(e.w % 256))
		buf.WriteByte(byte(e.h % 256))
		buf.Write([]byte{0, 0})
		_ = binary.Write(&buf, binary.LittleEndian, [2]uint16{1, uint16(e.bpp)})
		_ = binary.Write(&buf, binary.LittleEndian, [2]uint32{uint32(len(e.data)), uint32(off)})
		off += len(e.data)
	}
	for _, e := range entries {
		buf.Write(e.data)
	}
	return buf.Bytes()
}

type icoEntry struct {
	w, h, bpp int
	data      []byte
}

// dib32 encodes img as a 32-bit ICO bitmap (bottom-up BGRA, empty AND mask).
func dib32(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, struct {
		Size         uint32
		W, H         int32
		Planes, Bits uint16
		Compression  uint32
		ImageSize    uint32
		XPPM, YPPM   int32
		Used, Imp    uint32
	}{40, int32(w), int32(2 * h), 1, 32, 0, 0, 0, 0, 0, 0})
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			buf.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	buf.Write(make([]byte, ((w+31)/32)*4*h))
	return buf.Bytes()
}

// dib1 is a 1-bit 16×16 bitmap: palette [white, blue], a blue left half, and
// an AND mask that makes the top row transparent.
func dib1() []byte {
	const w, h = 16, 16
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.LittleEndian, struct {
		Size         uint32
		W, H         int32
		Planes, Bits uint16
		Rest         [24]byte
	}{Size: 40, W: w, H: 2 * h, Planes: 1, Bits: 1})
	buf.Write([]byte{255, 255, 255, 0, 255, 0, 0, 0}) // BGRx: white, blue
	for y := 0; y < h; y++ {
		buf.Write([]byte{0xff, 0x00, 0, 0}) // 8 blue, 8 white, padded to 4 bytes
	}
	for y := h - 1; y >= 0; y-- {
		if y == 0 {
			buf.Write([]byte{0xff, 0xff, 0, 0})
		} else {
			buf.Write([]byte{0, 0, 0, 0})
		}
	}
	return buf.Bytes()
}

func TestDecode(t *testing.T) {
	src := testIcon(48, 48)
	img, err := Decode(pngBytes(t, src))
	if err != nil || img.Bounds().Dx() != 48 {
		t.Fatalf("png: %v", err)
	}

	// The largest ICO entry wins, PNG or BMP.
	ico := icoFile(icoEntry{16, 16, 32, dib32(testIcon(16, 16))}, icoEntry{32, 32, 32, dib32(testIcon(32, 32))}, icoEntry{48, 48, 32, pngBytes(t, src)})
	img, err = Decode(ico)
	if err != nil || img.Bounds().Dx() != 48 {
		t.Fatalf("ico with png entry: %v %v", err, img)
	}
	ico = icoFile(icoEntry{16, 16, 32, dib32(testIcon(16, 16))}, icoEntry{32, 32, 32, dib32(testIcon(32, 32))})
	img, err = Decode(ico)
	if err != nil || img.Bounds().Dx() != 32 {
		t.Fatalf("ico 32-bit: %v", err)
	}
	want := testIcon(32, 32)
	for _, p := range []image.Point{{0, 0}, {16, 16}, {3, 16}, {31, 31}} {
		if got := color.NRGBAModel.Convert(img.At(p.X, p.Y)); got != want.NRGBAAt(p.X, p.Y) {
			t.Errorf("ico 32-bit pixel %v = %v, want %v", p, got, want.NRGBAAt(p.X, p.Y))
		}
	}

	img, err = Decode(icoFile(icoEntry{16, 16, 1, dib1()}))
	if err != nil {
		t.Fatalf("ico 1-bit: %v", err)
	}
	for _, c := range []struct {
		x, y int
		want color.NRGBA
	}{{0, 0, color.NRGBA{}}, {0, 5, color.NRGBA{0, 0, 255, 255}}, {12, 5, color.NRGBA{255, 255, 255, 255}}} {
		got := color.NRGBAModel.Convert(img.At(c.x, c.y)).(color.NRGBA)
		if got.A == 0 && c.want.A == 0 {
			continue
		}
		if got != c.want {
			t.Errorf("ico 1-bit pixel (%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}

	for name, body := range map[string][]byte{
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`),
		"webp":      []byte("RIFF\x00\x00\x00\x00WEBPVP8L"),
		"html":      []byte("<!doctype html><title>404</title>"),
		"empty":     nil,
		"truncated": {0, 0, 1, 0, 1, 0},
	} {
		if _, err := Decode(body); err == nil {
			t.Errorf("%s: decoded", name)
		}
	}
	if _, err := Decode([]byte("<svg/>")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("svg error = %v", err)
	}
}

func TestCheck(t *testing.T) {
	if err := Check(testIcon(16, 16)); err != nil {
		t.Errorf("16px icon: %v", err)
	}
	if err := Check(testIcon(15, 15)); err == nil {
		t.Error("15px icon accepted")
	}
	if err := Check(image.NewNRGBA(image.Rect(0, 0, 32, 32))); !errors.Is(err, ErrBlank) {
		t.Errorf("transparent icon: %v", err)
	}
	flat := image.NewUniform(color.NRGBA{10, 20, 30, 255})
	if err := Check(&boundedUniform{flat, image.Rect(0, 0, 32, 32)}); !errors.Is(err, ErrBlank) {
		t.Errorf("flat icon: %v", err)
	}
	// A white mark on transparency (made for dark tabs) vanishes on the
	// white logo ground; so does a faint one.
	for name, c := range map[string]color.NRGBA{"white": {255, 255, 255, 255}, "near-white": {240, 238, 236, 255}, "faint grey": {0, 0, 0, 40}} {
		img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
		for y := 8; y < 24; y++ {
			for x := 8; x < 24; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
		if err := Check(img); !errors.Is(err, ErrInvisible) {
			t.Errorf("%s mark: %v", name, err)
		}
	}
	// A white tile with a dark mark shows.
	tile := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i := range tile.Pix {
		tile.Pix[i] = 255
	}
	tile.SetNRGBA(16, 16, color.NRGBA{30, 30, 30, 255})
	if err := Check(tile); err != nil {
		t.Errorf("white tile with a mark: %v", err)
	}
}

type boundedUniform struct {
	*image.Uniform
	r image.Rectangle
}

func (b *boundedUniform) Bounds() image.Rectangle { return b.r }

func TestNormalize(t *testing.T) {
	// A 16px favicon becomes exact 4×4 blocks.
	src := testIcon(16, 16)
	out := Normalize(src)
	if out.Bounds() != image.Rect(0, 0, Size, Size) {
		t.Fatalf("bounds %v", out.Bounds())
	}
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			if out.NRGBAAt(x, y) != src.NRGBAAt(x/4, y/4) {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, out.NRGBAAt(x, y), src.NRGBAAt(x/4, y/4))
			}
		}
	}
	// 128 → 64 averages 2×2 blocks; transparent neighbours do not darken an
	// edge pixel's colour.
	big := testIcon(128, 128)
	out = Normalize(big)
	for y := 0; y < Size; y++ {
		for x := 0; x < Size; x++ {
			c := out.NRGBAAt(x, y)
			if c.A != 0 && (c.R != 200 || c.G != 30 || c.B != 30) {
				t.Fatalf("pixel (%d,%d) = %v: colour bled", x, y, c)
			}
		}
	}
	// A wide wordmark is fitted and centred vertically.
	wide := image.NewNRGBA(image.Rect(0, 0, 200, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 200; x++ {
			wide.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255})
		}
	}
	wide.SetNRGBA(100, 25, color.NRGBA{255, 255, 255, 255}) // not one flat colour
	out = Normalize(wide)
	if out.NRGBAAt(32, 0).A != 0 || out.NRGBAAt(32, 31).A != 255 || out.NRGBAAt(32, 63).A != 0 {
		t.Errorf("wide image not centred: top %v mid %v bottom %v", out.NRGBAAt(32, 0), out.NRGBAAt(32, 31), out.NRGBAAt(32, 63))
	}
	b, edge, err := Prepare(pngBytes(t, wide))
	if err != nil || edge != 50 {
		t.Fatalf("Prepare: %v edge %d", err, edge)
	}
	if cfg, err := png.DecodeConfig(bytes.NewReader(b)); err != nil || cfg.Width != Size || cfg.Height != Size {
		t.Errorf("Prepare output %v %+v", err, cfg)
	}
}

// fakeSite serves canned responses and records what was asked.
type fakeSite struct {
	mu     sync.Mutex
	routes map[string]func() (int, string, []byte)
	asked  []string
	agents []string
}

func (f *fakeSite) RoundTrip(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.asked = append(f.asked, r.URL.String())
	f.agents = append(f.agents, r.Header.Get("User-Agent"))
	route, ok := f.routes[r.URL.String()]
	f.mu.Unlock()
	status, typ, body := 404, "text/plain", []byte("not found")
	if ok {
		status, typ, body = route()
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {typ}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    r,
	}, nil
}

func TestFetch(t *testing.T) {
	icon192 := pngBytes(t, testIcon(192, 192))
	icon32 := pngBytes(t, testIcon(32, 32))
	site := &fakeSite{routes: map[string]func() (int, string, []byte){
		"https://acme.test/": func() (int, string, []byte) {
			return 200, "text/html", []byte(`<head><link rel="icon" sizes="32x32" href="/f32.png"><link rel="apple-touch-icon" href="/touch.png"></head>`)
		},
		"https://acme.test/touch.png": func() (int, string, []byte) { return 200, "image/png", icon192 },
		"https://acme.test/f32.png":   func() (int, string, []byte) { return 200, "image/png", icon32 },
	}}
	ctx := fetch.WithRoundTripper(context.Background(), site)
	res, err := Fetch(ctx, "https://acme.test/", Options{Fallback: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "https://acme.test/touch.png" || res.Edge != 192 {
		t.Errorf("picked %s (%dpx)", res.Source, res.Edge)
	}
	// It stops at the first sharp icon: no favicon.ico, no favicon service.
	for _, u := range site.asked {
		if strings.Contains(u, "favicon") || strings.Contains(u, "f32") {
			t.Errorf("asked for %s after a 192px icon", u)
		}
	}
	for _, ua := range site.agents {
		if ua != fetch.UserAgent {
			t.Errorf("User-Agent %q", ua)
		}
	}
}

// A site whose <link rel="icon"> is SVG-only gets its logo from the PNG in
// its web app manifest.
func TestFetchManifest(t *testing.T) {
	icon512 := pngBytes(t, testIcon(512, 512))
	site := &fakeSite{routes: map[string]func() (int, string, []byte){
		"https://svgco.test/": func() (int, string, []byte) {
			return 200, "text/html", []byte(`<head><link rel="icon" type="image/svg+xml" href="/icon.svg"><link rel="manifest" href="/manifest.json"></head>`)
		},
		"https://svgco.test/manifest.json": func() (int, string, []byte) {
			return 200, "application/manifest+json", []byte(`{"icons":[{"src":"/android-chrome-512x512.png","sizes":"512x512","type":"image/png"}]}`)
		},
		"https://svgco.test/android-chrome-512x512.png": func() (int, string, []byte) { return 200, "image/png", icon512 },
	}}
	ctx := fetch.WithRoundTripper(context.Background(), site)
	res, err := Fetch(ctx, "https://svgco.test/", Options{Fallback: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "https://svgco.test/android-chrome-512x512.png" || res.Edge != 512 {
		t.Errorf("picked %s (%dpx)", res.Source, res.Edge)
	}
	for _, u := range site.asked {
		if strings.Contains(u, ".svg") || strings.Contains(u, "google.com") {
			t.Errorf("asked for %s", u)
		}
	}
}

// An undeclared /apple-touch-icon.png beats a 16 px /favicon.ico.
func TestFetchTouchIconGuess(t *testing.T) {
	touch := pngBytes(t, testIcon(180, 180))
	tiny := pngBytes(t, testIcon(16, 16))
	site := &fakeSite{routes: map[string]func() (int, string, []byte){
		"https://plain.test/":                     func() (int, string, []byte) { return 200, "text/html", []byte(`<head><title>Plain</title></head>`) },
		"https://plain.test/apple-touch-icon.png": func() (int, string, []byte) { return 200, "image/png", touch },
		"https://plain.test/favicon.ico":          func() (int, string, []byte) { return 200, "image/png", tiny },
	}}
	ctx := fetch.WithRoundTripper(context.Background(), site)
	res, err := Fetch(ctx, "https://plain.test/", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "https://plain.test/apple-touch-icon.png" || res.Edge != 180 {
		t.Errorf("picked %s (%dpx)", res.Source, res.Edge)
	}
}

func TestFetchFallback(t *testing.T) {
	service := pngBytes(t, testIcon(128, 128))
	site := &fakeSite{routes: map[string]func() (int, string, []byte){
		"https://blocked.test/":            func() (int, string, []byte) { return 403, "text/html", []byte("no bots") },
		"https://blocked.test/favicon.ico": func() (int, string, []byte) { return 403, "text/html", []byte("no bots") },
		"https://blocked.test/apple-touch-icon.png": func() (int, string, []byte) {
			return 403, "text/html", []byte("no bots")
		},
		"https://www.google.com/s2/favicons?domain=blocked.test&sz=128": func() (int, string, []byte) {
			return 200, "image/png", service
		},
	}}
	ctx := fetch.WithRoundTripper(context.Background(), site)
	if _, err := Fetch(ctx, "https://blocked.test/", Options{}); err == nil {
		t.Error("blocked site without fallback: no error")
	}
	res, err := Fetch(ctx, "https://blocked.test/", Options{Fallback: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Source, "https://www.google.com/s2/favicons") {
		t.Errorf("fallback source %s", res.Source)
	}
}

func TestFetchRejectsPlatformIcons(t *testing.T) {
	site := &fakeSite{routes: map[string]func() (int, string, []byte){
		"https://oldco.test/": func() (int, string, []byte) {
			return 200, "text/html", []byte(`<head><link rel="icon" href="https://s0.wp.com/i/webclip.png"></head>`)
		},
	}}
	ctx := fetch.WithRoundTripper(context.Background(), site)
	if res, err := Fetch(ctx, "https://oldco.test/", Options{}); err == nil {
		t.Errorf("platform icon accepted: %+v", res)
	}
	for _, u := range site.asked {
		if strings.Contains(u, "wp.com") {
			t.Errorf("fetched %s", u)
		}
	}
	if !genericHost("https://minogamesonline.wordpress.com/") || genericHost("https://wordpress.org/") {
		t.Error("genericHost")
	}
}

// A parked domain's /lander stub is not the company's site, and its icons are
// the parking service's.
func TestFetchRejectsParkedDomain(t *testing.T) {
	site := &fakeSite{routes: map[string]func() (int, string, []byte){
		"https://gone.test/": func() (int, string, []byte) {
			return 200, "text/html", []byte(`<!DOCTYPE html><html><head><script>window.onload=function(){window.location.href="/lander"}</script></head></html>`)
		},
		"https://gone.test/favicon.ico": func() (int, string, []byte) { return 200, "image/png", pngBytes(t, testIcon(32, 32)) },
	}}
	ctx := fetch.WithRoundTripper(context.Background(), site)
	if res, err := Fetch(ctx, "https://gone.test/", Options{Fallback: true}); err == nil || !strings.Contains(err.Error(), "parked") {
		t.Errorf("parked domain: %+v, %v", res, err)
	}
	if len(site.asked) != 1 {
		t.Errorf("asked %v after a parked page", site.asked)
	}
}
