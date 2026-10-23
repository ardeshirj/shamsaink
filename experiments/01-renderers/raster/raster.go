// Package raster is a tiny greyscale drawing surface for the trial: paths filled with
// golang.org/x/image/vector, glyph masks cached per (font, glyph, size, subpixel), pages saved
// as 16-grey PNGs (an e-ink panel's depth).
package raster

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"

	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
	"golang.org/x/image/vector"
)

// Page is a white greyscale page.
type Page struct {
	Img *image.Gray
}

func NewPage(w, h int) *Page {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	return &Page{Img: img}
}

// FillMask paints grey level `ink` (0 black) through an alpha mask placed at (x, y).
func (p *Page) FillMask(mask *image.Alpha, x, y int, ink uint8, clip image.Rectangle) {
	r := mask.Bounds().Add(image.Pt(x, y)).Intersect(clip).Intersect(p.Img.Bounds())
	if r.Empty() {
		return
	}
	src := image.NewUniform(color.Gray{ink})
	draw.DrawMask(p.Img, r, src, image.Point{}, mask, r.Min.Sub(image.Pt(x, y)), draw.Over)
}

// Path is a device-space path (y down), filled non-zero.
type Path struct {
	segs                   []seg
	minX, minY, maxX, maxY float32
	empty                  bool
}

type seg struct {
	op  byte // 'M', 'L', 'Q', 'C', 'Z'
	pts [3][2]float32
}

func NewPath() *Path { return &Path{empty: true} }

func (p *Path) grow(x, y float32) {
	if p.empty {
		p.minX, p.minY, p.maxX, p.maxY, p.empty = x, y, x, y, false
		return
	}
	p.minX, p.minY = min(p.minX, x), min(p.minY, y)
	p.maxX, p.maxY = max(p.maxX, x), max(p.maxY, y)
}

func (p *Path) MoveTo(x, y float32) {
	p.segs = append(p.segs, seg{op: 'M', pts: [3][2]float32{{x, y}}})
	p.grow(x, y)
}
func (p *Path) LineTo(x, y float32) {
	p.segs = append(p.segs, seg{op: 'L', pts: [3][2]float32{{x, y}}})
	p.grow(x, y)
}
func (p *Path) QuadTo(x1, y1, x, y float32) {
	p.segs = append(p.segs, seg{op: 'Q', pts: [3][2]float32{{x1, y1}, {x, y}}})
	p.grow(x1, y1)
	p.grow(x, y)
}
func (p *Path) CubeTo(x1, y1, x2, y2, x, y float32) {
	p.segs = append(p.segs, seg{op: 'C', pts: [3][2]float32{{x1, y1}, {x2, y2}, {x, y}}})
	p.grow(x1, y1)
	p.grow(x2, y2)
	p.grow(x, y)
}
func (p *Path) Close()      { p.segs = append(p.segs, seg{op: 'Z'}) }
func (p *Path) Empty() bool { return p.empty }

// Bounds is the integer pixel box covering the path.
func (p *Path) Bounds() image.Rectangle {
	if p.empty {
		return image.Rectangle{}
	}
	return image.Rect(int(math.Floor(float64(p.minX))), int(math.Floor(float64(p.minY))),
		int(math.Ceil(float64(p.maxX)))+1, int(math.Ceil(float64(p.maxY)))+1)
}

// Mask rasterizes the path into an alpha mask whose bounds are the path's pixel box.
func (p *Path) Mask() *image.Alpha {
	b := p.Bounds()
	if b.Empty() {
		return nil
	}
	z := vector.NewRasterizer(b.Dx(), b.Dy())
	ox, oy := float32(b.Min.X), float32(b.Min.Y)
	open := false
	for _, s := range p.segs {
		switch s.op {
		case 'M':
			if open {
				z.ClosePath()
			}
			z.MoveTo(s.pts[0][0]-ox, s.pts[0][1]-oy)
			open = true
		case 'L':
			z.LineTo(s.pts[0][0]-ox, s.pts[0][1]-oy)
		case 'Q':
			z.QuadTo(s.pts[0][0]-ox, s.pts[0][1]-oy, s.pts[1][0]-ox, s.pts[1][1]-oy)
		case 'C':
			z.CubeTo(s.pts[0][0]-ox, s.pts[0][1]-oy, s.pts[1][0]-ox, s.pts[1][1]-oy, s.pts[2][0]-ox, s.pts[2][1]-oy)
		case 'Z':
			if open {
				z.ClosePath()
				open = false
			}
		}
	}
	if open {
		z.ClosePath()
	}
	m := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	z.Draw(m, m.Bounds(), image.Opaque, image.Point{})
	m.Rect = m.Rect.Add(b.Min)
	return m
}

// Fill paints the path in grey `ink`.
func (p *Page) Fill(path *Path, ink uint8, clip image.Rectangle) {
	m := path.Mask()
	if m == nil {
		return
	}
	p.FillMask(m, 0, 0, ink, clip)
}

// GlyphCache rasterizes a glyph once per (face, gid, pixel size, quarter-pixel x phase).
type GlyphCache struct {
	m            map[glyphKey]*image.Alpha
	Hits, Misses int
}

type glyphKey struct {
	face  *font.Face
	gid   font.GID
	size  float32
	phase uint8
}

func NewGlyphCache() *GlyphCache { return &GlyphCache{m: map[glyphKey]*image.Alpha{}} }

// Draw paints glyph `gid` of `face` at pixel size `size` with its origin (baseline, pen) at (x, y),
// y down, flipped horizontally if scaleX is negative (not used here).
func (c *GlyphCache) Draw(p *Page, face *font.Face, gid font.GID, size, x, y float32, ink uint8, clip image.Rectangle) {
	ix := float32(math.Floor(float64(x)))
	phase := uint8((x - ix) * 4)
	iy := float32(math.Round(float64(y)))
	k := glyphKey{face, gid, size, phase}
	m, ok := c.m[k]
	if !ok {
		c.Misses++
		m = glyphMask(face, gid, size, float32(phase)/4)
		c.m[k] = m
	} else {
		c.Hits++
	}
	if m == nil {
		return
	}
	p.FillMask(m, int(ix), int(iy), ink, clip)
}

// glyphMask returns a mask in coordinates relative to the glyph origin; its Rect.Min is the offset.
func glyphMask(face *font.Face, gid font.GID, size, dx float32) *image.Alpha {
	out, ok := face.GlyphData(gid).(font.GlyphOutline)
	if !ok || len(out.Segments) == 0 {
		return nil
	}
	s := size / float32(face.Upem())
	path := NewPath()
	for _, sg := range out.Segments {
		a := sg.Args
		switch sg.Op {
		case ot.SegmentOpMoveTo:
			path.MoveTo(a[0].X*s+dx, -a[0].Y*s)
		case ot.SegmentOpLineTo:
			path.LineTo(a[0].X*s+dx, -a[0].Y*s)
		case ot.SegmentOpQuadTo:
			path.QuadTo(a[0].X*s+dx, -a[0].Y*s, a[1].X*s+dx, -a[1].Y*s)
		case ot.SegmentOpCubeTo:
			path.CubeTo(a[0].X*s+dx, -a[0].Y*s, a[1].X*s+dx, -a[1].Y*s, a[2].X*s+dx, -a[2].Y*s)
		}
	}
	m := path.Mask()
	if m == nil {
		return nil
	}
	return m // its Rect is relative to the glyph origin
}

// Save writes the page quantized to 16 grey levels.
func (p *Page) Save(path string) error {
	q := image.NewGray(p.Img.Bounds())
	for i, v := range p.Img.Pix {
		l := (int(v) + 8) / 17 // 0..15
		if l > 15 {
			l = 15
		}
		q.Pix[i] = uint8(l * 17)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, q)
}
