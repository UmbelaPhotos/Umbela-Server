// Package faces detects faces in photos and turns them into embeddings, using
// YuNet (detection) and SFace (recognition) through ONNX Runtime.
//
// The models are embedded in the binary. ONNX Runtime itself is a shared
// library that must be available at run time.
package faces

import (
	_ "embed"
	"fmt"
	"image"

	ort "github.com/yalue/onnxruntime_go"
)

//go:embed models/yunet.onnx
var yunetModel []byte

//go:embed models/sface.onnx
var sfaceModel []byte

// Engine holds the loaded models. It is safe for concurrent use, although
// calls to the same model are serialized.
type Engine struct {
	detector *detector
	embedder *embedder
}

// NewEngine loads the models. ortLibPath is the path to the ONNX Runtime
// shared library (libonnxruntime.so, .dylib or onnxruntime.dll). If empty,
// the library is looked up by its default name.
func NewEngine(ortLibPath string) (*Engine, error) {
	if !ort.IsInitialized() {
		if ortLibPath != "" {
			ort.SetSharedLibraryPath(ortLibPath)
		}
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("initializing ONNX Runtime: %w", err)
		}
	}

	det, err := newDetector(yunetModel)
	if err != nil {
		return nil, fmt.Errorf("loading face detector: %w", err)
	}
	emb, err := newEmbedder(sfaceModel)
	if err != nil {
		det.close()
		return nil, fmt.Errorf("loading face embedder: %w", err)
	}
	return &Engine{detector: det, embedder: emb}, nil
}

func (e *Engine) Close() error {
	e.detector.close()
	e.embedder.close()
	return ort.DestroyEnvironment()
}

// Detect returns the faces found in img, best score first.
func (e *Engine) Detect(img image.Image) ([]Detection, error) {
	return e.detector.detect(img)
}

// Embed aligns the detected face and returns its embedding.
func (e *Engine) Embed(img image.Image, d Detection) ([]float32, error) {
	return e.embedder.embed(alignFace(img, d.Landmarks))
}

// Analyze detects every face in img and computes its embedding.
func (e *Engine) Analyze(img image.Image) ([]Face, error) {
	dets, err := e.Detect(img)
	if err != nil {
		return nil, err
	}
	faces := make([]Face, 0, len(dets))
	for _, d := range dets {
		emb, err := e.Embed(img, d)
		if err != nil {
			return nil, err
		}
		faces = append(faces, Face{Detection: d, Embedding: emb})
	}
	return faces, nil
}
