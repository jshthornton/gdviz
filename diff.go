package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
)

// DiffResult carries the outcome of comparing two PNGs.
type DiffResult struct {
	BaseWidth, BaseHeight int
	CurWidth, CurHeight   int
	Width, Height         int
	DiffPixels            int
	TotalPixels           int
	Ratio                 float64
	SizeMismatch          bool
}

// Pixel-diff modelled on the pixelmatch algorithm (Mapbox, ISC license):
// gamma-corrected RGB mapped to YIQ, per-pixel color distance with a
// threshold, and anti-aliased pixel detection so edge softening from
// resampling or AA changes does not count as a regression.

const (
	yiqMicroThreshold = 0.1
	dimAlpha          = 0.1
)

var (
	diffColor    = [4]uint8{255, 0, 0, 255}
	diffColorAlt = [4]uint8{0, 255, 0, 255}
	aaColor      = [4]uint8{255, 255, 0, 255}
)

func loadRGBA(path string) (*image.RGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	return toRGBA(img), nil
}

func toRGBA(img image.Image) *image.RGBA {
	if r, ok := img.(*image.RGBA); ok && r.Rect.Min == image.Pt(0, 0) {
		return r
	}
	b := img.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

func pngDims(path string) (image.Point, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Point{}, err
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return image.Point{}, err
	}
	return image.Pt(cfg.Width, cfg.Height), nil
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// ComparePNG diffs two PNG files and (optionally, always in practice)
// produces a diff visualization. threshold follows pixelmatch semantics
// (0..1, default 0.1): smaller is more sensitive.
func ComparePNG(pathA, pathB string, threshold float64) (DiffResult, *image.RGBA, error) {
	a, err := loadRGBA(pathA)
	if err != nil {
		return DiffResult{}, nil, fmt.Errorf("baseline: %w", err)
	}
	b, err := loadRGBA(pathB)
	if err != nil {
		return DiffResult{}, nil, fmt.Errorf("current: %w", err)
	}
	res := DiffResult{
		BaseWidth:  a.Bounds().Dx(),
		BaseHeight: a.Bounds().Dy(),
		CurWidth:   b.Bounds().Dx(),
		CurHeight:  b.Bounds().Dy(),
	}
	if res.BaseWidth != res.CurWidth || res.BaseHeight != res.CurHeight {
		res.SizeMismatch = true
		return res, nil, nil
	}
	diffImg, count := diffRGBA(a, b, threshold)
	res.Width = res.BaseWidth
	res.Height = res.BaseHeight
	res.DiffPixels = count
	res.TotalPixels = res.Width * res.Height
	res.Ratio = float64(count) / float64(res.TotalPixels)
	return res, diffImg, nil
}

func gamma255(v float64) float64 {
	return 255 * math.Pow(v/255.0, 2.2)
}

func rgb2y(r, g, b float64) float64 { return r*0.29889531 + g*0.58662247 + b*0.11448223 }
func rgb2i(r, g, b float64) float64 { return r*0.59597799 - g*0.27417610 - b*0.32180189 }
func rgb2q(r, g, b float64) float64 { return r*0.21147017 - g*0.52261711 + b*0.31114694 }

func maxDeltaFor(threshold float64) float64 {
	return 35250 * threshold
}

// pixel returns the RGBA sample at pixel index p (p = (y*w+x)*4), blending
// alpha < 255 onto white like pixelmatch.
func sampleRGBA(img []uint8, p int) (r, g, b float64) {
	r = float64(img[p])
	g = float64(img[p+1])
	b = float64(img[p+2])
	if img[p+3] < 255 {
		a := float64(img[p+3]) / 255.0
		keep := 1.0 - a
		r = a*r + keep*255
		g = a*g + keep*255
		b = a*b + keep*255
	}
	return
}

// colorDelta returns the signed YIQ delta between two pixels, or 0 when the
// difference is below the micro-threshold (sub-quantization noise). The real
// threshold check happens in the caller, exactly like pixelmatch. With yOnly
// it returns the raw luminance difference (used for AA detection).
func colorDelta(img1, img2 []uint8, p1, p2 int, yOnly bool) float64 {
	r1, g1, b1 := sampleRGBA(img1, p1)
	r2, g2, b2 := sampleRGBA(img2, p2)
	if img1[p1] == img2[p2] && img1[p1+1] == img2[p2+1] && img1[p1+2] == img2[p2+2] && img1[p1+3] == img2[p2+3] {
		return 0
	}
	y1 := rgb2y(gamma255(r1), gamma255(g1), gamma255(b1))
	y2 := rgb2y(gamma255(r2), gamma255(g2), gamma255(b2))
	if yOnly {
		return y1 - y2
	}
	y := y1 - y2
	i := rgb2i(gamma255(r1), gamma255(g1), gamma255(b1)) - rgb2i(gamma255(r2), gamma255(g2), gamma255(b2))
	q := rgb2q(gamma255(r1), gamma255(g1), gamma255(b1)) - rgb2q(gamma255(r2), gamma255(g2), gamma255(b2))
	delta := 0.5053*y*y + 0.299*i*i + 0.1957*q*q
	if delta <= yiqMicroThreshold {
		return 0
	}
	if y > 0 {
		return -delta
	}
	return delta
}

func diffRGBA(a, b *image.RGBA, threshold float64) (*image.RGBA, int) {
	w := a.Bounds().Dx()
	h := a.Bounds().Dy()
	maxDelta := maxDeltaFor(threshold)
	ab := a.Pix
	bb := b.Pix
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	ob := out.Pix
	diffCount := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pos := (y*w + x) * 4
			delta := colorDelta(ab, bb, pos, pos, false)
			drawn := false
			if math.Abs(delta) > maxDelta {
				if antialiased(ab, w, h, x, y) || antialiased(bb, w, h, x, y) {
					copy(ob[pos:pos+4], aaColor[:])
					drawn = true
				} else {
					diffCount++
					if delta < 0 {
						copy(ob[pos:pos+4], diffColor[:])
					} else {
						copy(ob[pos:pos+4], diffColorAlt[:])
					}
					drawn = true
				}
			}
			if !drawn {
				// dim unchanged pixels so differences stand out
				ob[pos] = dimChannel(ab[pos])
				ob[pos+1] = dimChannel(ab[pos+1])
				ob[pos+2] = dimChannel(ab[pos+2])
				ob[pos+3] = 255
			}
		}
	}
	return out, diffCount
}

func dimChannel(v uint8) uint8 {
	return uint8(float64(v)*(1.0-dimAlpha) + 255.0*dimAlpha)
}

func antialiased(img []uint8, w, h, x1, y1 int) bool {
	x0 := maxI(x1-1, 0)
	y0 := maxI(y1-1, 0)
	x2 := minI(x1+1, w-1)
	y2 := minI(y1+1, h-1)
	pos := (y1*w + x1) * 4
	zeroes := 0
	if x1 == x0 || x1 == x2 || y1 == y0 || y1 == y2 {
		zeroes = 1
	}
	minDelta, maxDelta := 0.0, 0.0
	minX, minY, maxX, maxY := 0, 0, 0, 0
	for y := y0; y <= y2; y++ {
		for x := x0; x <= x2; x++ {
			if x == x1 && y == y1 {
				continue
			}
			delta := colorDelta(img, img, pos, (y*w+x)*4, true)
			if delta == 0 {
				// match JS `if (zeroes++ > 2)` post-increment semantics
				old := zeroes
				zeroes++
				if old > 2 {
					return false
				}
			} else if delta < minDelta {
				minDelta = delta
				minX, minY = x, y
			} else if delta > maxDelta {
				maxDelta = delta
				maxX, maxY = x, y
			}
		}
	}
	if minDelta == 0 || maxDelta == 0 {
		return false
	}
	return hasManySiblings(img, w, h, minX, minY) && hasManySiblings(img, w, h, maxX, maxY)
}

func hasManySiblings(img []uint8, w, h, x1, y1 int) bool {
	x0 := maxI(x1-1, 0)
	y0 := maxI(y1-1, 0)
	x2 := minI(x1+1, w-1)
	y2 := minI(y1+1, h-1)
	pos := (y1*w + x1) * 4
	siblings := 0
	for y := y0; y <= y2; y++ {
		for x := x0; x <= x2; x++ {
			if x == x1 && y == y1 {
				continue
			}
			p := (y*w + x) * 4
			if img[p] == img[pos] && img[p+1] == img[pos+1] && img[p+2] == img[pos+2] && img[p+3] == img[pos+3] {
				siblings++
			}
		}
	}
	return siblings >= 2
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minI(a, b int) int {
	if a < b {
		return a
	}
	return b
}
