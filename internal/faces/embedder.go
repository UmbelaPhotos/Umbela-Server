package faces

import (
	"fmt"
	"image"
	"math"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// embedder runs SFace: an aligned 112x112 face in, a 128-number vector out.
type embedder struct {
	mu      sync.Mutex
	session *ort.AdvancedSession
	input   *ort.Tensor[float32]
	output  *ort.Tensor[float32]
}

func newEmbedder(model []byte) (*embedder, error) {
	inputs, outputs, err := ort.GetInputOutputInfoWithONNXData(model)
	if err != nil {
		return nil, fmt.Errorf("reading model info: %w", err)
	}
	// Some exports list the weights as graph inputs too, so pick the image
	// input by its shape instead of assuming it is the only one.
	input := -1
	for i, in := range inputs {
		if d := in.Dimensions; len(d) == 4 && d[1] == 3 && d[2] == AlignedSize && d[3] == AlignedSize {
			input = i
			break
		}
	}
	if input < 0 {
		return nil, fmt.Errorf("model has no 1x3x%dx%d input, is it SFace?", AlignedSize, AlignedSize)
	}
	if len(outputs) != 1 {
		return nil, fmt.Errorf("expected 1 output, the model has %d", len(outputs))
	}
	if n := elementCount(outputs[0].Dimensions); n != 0 && n != EmbeddingSize {
		return nil, fmt.Errorf("model outputs %d values, expected %d, is it SFace?", n, EmbeddingSize)
	}

	e := &embedder{}
	e.input, err = ort.NewEmptyTensor[float32](ort.NewShape(1, 3, AlignedSize, AlignedSize))
	if err != nil {
		return nil, fmt.Errorf("creating input tensor: %w", err)
	}
	e.output, err = ort.NewEmptyTensor[float32](ort.NewShape(1, EmbeddingSize))
	if err != nil {
		e.close()
		return nil, fmt.Errorf("creating output tensor: %w", err)
	}
	e.session, err = ort.NewAdvancedSessionWithONNXData(
		model,
		[]string{inputs[input].Name},
		[]string{outputs[0].Name},
		[]ort.ArbitraryTensor{e.input},
		[]ort.ArbitraryTensor{e.output},
		nil,
	)
	if err != nil {
		e.close()
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return e, nil
}

func (e *embedder) close() {
	if e.session != nil {
		e.session.Destroy()
	}
	if e.input != nil {
		e.input.Destroy()
	}
	if e.output != nil {
		e.output.Destroy()
	}
}

// embed returns the L2-normalized embedding of an aligned face.
func (e *embedder) embed(aligned *image.RGBA) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// SFace wants RGB, 0-255, planar.
	data := e.input.GetData()
	plane := AlignedSize * AlignedSize
	for y := 0; y < AlignedSize; y++ {
		row := aligned.Pix[y*aligned.Stride:]
		for x := 0; x < AlignedSize; x++ {
			i := y*AlignedSize + x
			data[i] = float32(row[4*x])
			data[plane+i] = float32(row[4*x+1])
			data[2*plane+i] = float32(row[4*x+2])
		}
	}

	if err := e.session.Run(); err != nil {
		return nil, fmt.Errorf("running embedder: %w", err)
	}

	out := make([]float32, EmbeddingSize)
	copy(out, e.output.GetData())

	var norm float64
	for _, v := range out {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return nil, fmt.Errorf("embedder returned a zero vector")
	}
	for i := range out {
		out[i] = float32(float64(out[i]) / norm)
	}
	return out, nil
}
