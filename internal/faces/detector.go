package faces

import (
	"fmt"
	"image"
	"math"
	"sort"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
	"golang.org/x/image/draw"
)

const (
	defaultInputSize = 640
	scoreThreshold   = 0.9
	nmsThreshold     = 0.3
)

var strides = [3]int{8, 16, 32}

// detector runs YuNet. The model has three output scales (strides 8, 16, 32)
// with four tensors each: classification, objectness, box and landmarks.
type detector struct {
	mu            sync.Mutex
	session       *ort.AdvancedSession
	input         *ort.Tensor[float32]
	cls, obj      [3]*ort.Tensor[float32]
	bbox, kps     [3]*ort.Tensor[float32]
	width, height int
}

func newDetector(model []byte) (*detector, error) {
	inputs, outputs, err := ort.GetInputOutputInfoWithONNXData(model)
	if err != nil {
		return nil, fmt.Errorf("reading model info: %w", err)
	}
	if len(inputs) != 1 {
		return nil, fmt.Errorf("expected 1 input, the model has %d", len(inputs))
	}

	d := &detector{width: defaultInputSize, height: defaultInputSize}
	// A fixed input size is baked into some exports of the model.
	if dims := inputs[0].Dimensions; len(dims) == 4 && dims[2] > 0 && dims[3] > 0 {
		d.height, d.width = int(dims[2]), int(dims[3])
	}
	if d.width%32 != 0 || d.height%32 != 0 {
		return nil, fmt.Errorf("input size %dx%d is not a multiple of 32", d.width, d.height)
	}

	byName := make(map[string]ort.InputOutputInfo, len(outputs))
	for _, o := range outputs {
		byName[o.Name] = o
	}

	d.input, err = ort.NewEmptyTensor[float32](ort.NewShape(1, 3, int64(d.height), int64(d.width)))
	if err != nil {
		return nil, fmt.Errorf("creating input tensor: %w", err)
	}

	var names []string
	var tensors []ort.ArbitraryTensor
	kinds := []struct {
		prefix string
		width  int64
		dst    *[3]*ort.Tensor[float32]
	}{
		{"cls", 1, &d.cls},
		{"obj", 1, &d.obj},
		{"bbox", 4, &d.bbox},
		{"kps", 10, &d.kps},
	}
	for _, k := range kinds {
		for i, s := range strides {
			name := fmt.Sprintf("%s_%d", k.prefix, s)
			info, ok := byName[name]
			if !ok {
				d.close()
				return nil, fmt.Errorf("model has no output %q, is it YuNet?", name)
			}
			n := int64(d.height/s) * int64(d.width/s)
			if got := elementCount(info.Dimensions); got > 0 && got != n*k.width {
				d.close()
				return nil, fmt.Errorf("output %q has %d values, expected %d", name, got, n*k.width)
			}
			t, err := ort.NewEmptyTensor[float32](ort.NewShape(1, n, k.width))
			if err != nil {
				d.close()
				return nil, fmt.Errorf("creating tensor %q: %w", name, err)
			}
			k.dst[i] = t
			names = append(names, name)
			tensors = append(tensors, t)
		}
	}

	d.session, err = ort.NewAdvancedSessionWithONNXData(
		model,
		[]string{inputs[0].Name},
		names,
		[]ort.ArbitraryTensor{d.input},
		tensors,
		nil,
	)
	if err != nil {
		d.close()
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return d, nil
}

func (d *detector) close() {
	if d.session != nil {
		d.session.Destroy()
	}
	if d.input != nil {
		d.input.Destroy()
	}
	for i := range strides {
		for _, t := range []*ort.Tensor[float32]{d.cls[i], d.obj[i], d.bbox[i], d.kps[i]} {
			if t != nil {
				t.Destroy()
			}
		}
	}
}

// detect finds faces in img. Coordinates are in img's own pixel space.
func (d *detector) detect(img image.Image) ([]Detection, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	scale := d.fill(img)
	if err := d.session.Run(); err != nil {
		return nil, fmt.Errorf("running detector: %w", err)
	}

	dets := nms(d.decode(), nmsThreshold)

	// Back from the letterboxed input to the original image.
	min := img.Bounds().Min
	back := func(p Point) Point {
		return Point{p.X/scale + float32(min.X), p.Y/scale + float32(min.Y)}
	}
	for i := range dets {
		tl := back(Point{dets[i].Box.X1, dets[i].Box.Y1})
		br := back(Point{dets[i].Box.X2, dets[i].Box.Y2})
		dets[i].Box = Rect{tl.X, tl.Y, br.X, br.Y}
		for j := range dets[i].Landmarks {
			dets[i].Landmarks[j] = back(dets[i].Landmarks[j])
		}
	}
	return dets, nil
}

// fill resizes img into the top-left of the input tensor, keeping the aspect
// ratio, and returns the scale that was applied. The model wants BGR, 0-255.
func (d *detector) fill(img image.Image) float32 {
	b := img.Bounds()
	scale := float32(math.Min(float64(d.width)/float64(b.Dx()), float64(d.height)/float64(b.Dy())))
	w := max(1, int(float32(b.Dx())*scale))
	h := max(1, int(float32(b.Dy())*scale))

	resized := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.BiLinear.Scale(resized, resized.Bounds(), img, b, draw.Src, nil)

	data := d.input.GetData()
	clear(data)
	plane := d.width * d.height
	for y := 0; y < h; y++ {
		row := resized.Pix[y*resized.Stride:]
		for x := 0; x < w; x++ {
			i := y*d.width + x
			data[i] = float32(row[4*x+2])       // B
			data[plane+i] = float32(row[4*x+1]) // G
			data[2*plane+i] = float32(row[4*x]) // R
		}
	}
	return scale
}

// decode turns the raw model outputs into detections in input coordinates.
func (d *detector) decode() []Detection {
	var out []Detection
	for i, s := range strides {
		cls := d.cls[i].GetData()
		obj := d.obj[i].GetData()
		bbox := d.bbox[i].GetData()
		kps := d.kps[i].GetData()
		stride := float32(s)
		cols := d.width / s
		rows := d.height / s

		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				idx := r*cols + c
				score := float32(math.Sqrt(float64(clamp01(cls[idx]) * clamp01(obj[idx]))))
				if score < scoreThreshold {
					continue
				}

				cx := (float32(c) + bbox[idx*4]) * stride
				cy := (float32(r) + bbox[idx*4+1]) * stride
				w := float32(math.Exp(float64(bbox[idx*4+2]))) * stride
				h := float32(math.Exp(float64(bbox[idx*4+3]))) * stride

				det := Detection{
					Box:   Rect{cx - w/2, cy - h/2, cx + w/2, cy + h/2},
					Score: score,
				}
				for n := 0; n < 5; n++ {
					det.Landmarks[n] = Point{
						(kps[idx*10+2*n] + float32(c)) * stride,
						(kps[idx*10+2*n+1] + float32(r)) * stride,
					}
				}
				out = append(out, det)
			}
		}
	}
	return out
}

// nms keeps the best-scoring detection of every group of overlapping boxes.
func nms(dets []Detection, iouThreshold float32) []Detection {
	sort.Slice(dets, func(i, j int) bool { return dets[i].Score > dets[j].Score })
	var keep []Detection
	for _, d := range dets {
		overlaps := false
		for _, k := range keep {
			if iou(d.Box, k.Box) > iouThreshold {
				overlaps = true
				break
			}
		}
		if !overlaps {
			keep = append(keep, d)
		}
	}
	return keep
}

func iou(a, b Rect) float32 {
	ix := max(0, min(a.X2, b.X2)-max(a.X1, b.X1))
	iy := max(0, min(a.Y2, b.Y2)-max(a.Y1, b.Y1))
	inter := ix * iy
	union := (a.X2-a.X1)*(a.Y2-a.Y1) + (b.X2-b.X1)*(b.Y2-b.Y1) - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

func clamp01(v float32) float32 {
	return max(0, min(1, v))
}

func elementCount(dims []int64) int64 {
	n := int64(1)
	for _, d := range dims {
		if d <= 0 {
			return 0 // dynamic dimension, unknown
		}
		n *= d
	}
	return n
}
