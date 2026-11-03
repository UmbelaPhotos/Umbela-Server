package faces

import (
	"image"
	"math"
)

// arcfaceTemplate is where the five landmarks must land inside the
// 112x112 aligned crop (the standard template used by ArcFace and SFace).
var arcfaceTemplate = [5]Point{
	{38.2946, 51.6963},
	{73.5318, 51.5014},
	{56.0252, 71.7366},
	{41.5493, 92.3655},
	{70.7299, 92.2041},
}

// similarityTransform finds the scale, rotation and translation that best
// map src onto dst (least squares). The transform is:
//
//	u = a*x - b*y + tx
//	v = b*x + a*y + ty
func similarityTransform(src, dst [5]Point) (a, b, tx, ty float64) {
	var sx, sy, dx, dy float64
	for i := range src {
		sx += float64(src[i].X)
		sy += float64(src[i].Y)
		dx += float64(dst[i].X)
		dy += float64(dst[i].Y)
	}
	n := float64(len(src))
	sx, sy, dx, dy = sx/n, sy/n, dx/n, dy/n

	var num1, num2, den float64
	for i := range src {
		x := float64(src[i].X) - sx
		y := float64(src[i].Y) - sy
		u := float64(dst[i].X) - dx
		v := float64(dst[i].Y) - dy
		num1 += x*u + y*v
		num2 += x*v - y*u
		den += x*x + y*y
	}
	if den == 0 {
		return 0, 0, 0, 0
	}
	a = num1 / den
	b = num2 / den
	tx = dx - (a*sx - b*sy)
	ty = dy - (b*sx + a*sy)
	return a, b, tx, ty
}

// alignFace warps the face described by its landmarks into the canonical
// AlignedSize x AlignedSize crop.
func alignFace(img image.Image, landmarks [5]Point) *image.RGBA {
	out := image.NewRGBA(image.Rect(0, 0, AlignedSize, AlignedSize))
	a, b, tx, ty := similarityTransform(landmarks, arcfaceTemplate)
	den := a*a + b*b
	if den == 0 {
		return out
	}

	min := img.Bounds().Min
	for v := 0; v < AlignedSize; v++ {
		for u := 0; u < AlignedSize; u++ {
			du := float64(u) - tx
			dv := float64(v) - ty
			// Inverse of the transform: where does this output pixel come from?
			x := (a*du+b*dv)/den + float64(min.X)
			y := (-b*du+a*dv)/den + float64(min.Y)
			r, g, bl := sampleBilinear(img, x, y)
			i := out.PixOffset(u, v)
			out.Pix[i] = uint8(r + 0.5)
			out.Pix[i+1] = uint8(g + 0.5)
			out.Pix[i+2] = uint8(bl + 0.5)
			out.Pix[i+3] = 255
		}
	}
	return out
}

// sampleBilinear returns the interpolated RGB (0-255) at a fractional
// position. Positions outside the image are black.
func sampleBilinear(img image.Image, x, y float64) (r, g, b float64) {
	x0 := math.Floor(x)
	y0 := math.Floor(y)
	fx := x - x0
	fy := y - y0
	ix, iy := int(x0), int(y0)

	weights := [4]float64{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy}
	offsets := [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}}
	for i, o := range offsets {
		pr, pg, pb := pixelAt(img, ix+o[0], iy+o[1])
		r += weights[i] * pr
		g += weights[i] * pg
		b += weights[i] * pb
	}
	return r, g, b
}

func pixelAt(img image.Image, x, y int) (r, g, b float64) {
	if !(image.Point{X: x, Y: y}).In(img.Bounds()) {
		return 0, 0, 0
	}
	pr, pg, pb, _ := img.At(x, y).RGBA()
	return float64(pr >> 8), float64(pg >> 8), float64(pb >> 8)
}
