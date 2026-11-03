package faces

import (
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"testing"
)

func TestSimilarityTransformIdentity(t *testing.T) {
	a, b, tx, ty := similarityTransform(arcfaceTemplate, arcfaceTemplate)
	if math.Abs(a-1) > 1e-9 || math.Abs(b) > 1e-9 || math.Abs(tx) > 1e-6 || math.Abs(ty) > 1e-6 {
		t.Fatalf("expected identity, got a=%v b=%v tx=%v ty=%v", a, b, tx, ty)
	}
}

func TestSimilarityTransformRecoversKnownTransform(t *testing.T) {
	// Scale 2, rotation 30 degrees, translation (10, -5).
	wantA := 2 * math.Cos(math.Pi/6)
	wantB := 2 * math.Sin(math.Pi/6)
	var src [5]Point
	for i, p := range arcfaceTemplate {
		x, y := float64(p.X), float64(p.Y)
		src[i] = Point{float32(wantA*x - wantB*y + 10), float32(wantB*x + wantA*y - 5)}
	}

	// Mapping src back onto the template must undo that transform.
	a, b, tx, ty := similarityTransform(src, arcfaceTemplate)
	for i, p := range src {
		u := a*float64(p.X) - b*float64(p.Y) + tx
		v := b*float64(p.X) + a*float64(p.Y) + ty
		if math.Abs(u-float64(arcfaceTemplate[i].X)) > 1e-2 || math.Abs(v-float64(arcfaceTemplate[i].Y)) > 1e-2 {
			t.Fatalf("point %d maps to (%v, %v), want %v", i, u, v, arcfaceTemplate[i])
		}
	}
}

func TestAlignFaceWithTemplateLandmarksKeepsImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, AlignedSize, AlignedSize))
	for y := 0; y < AlignedSize; y++ {
		for x := 0; x < AlignedSize; x++ {
			src.SetRGBA(x, y, color.RGBA{uint8(x * 2), uint8(y * 2), 100, 255})
		}
	}

	got := alignFace(src, arcfaceTemplate)
	for _, p := range [][2]int{{10, 10}, {56, 56}, {100, 80}} {
		want := src.RGBAAt(p[0], p[1])
		have := got.RGBAAt(p[0], p[1])
		if absDiff(want.R, have.R) > 1 || absDiff(want.G, have.G) > 1 {
			t.Fatalf("pixel %v changed: want %v, got %v", p, want, have)
		}
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

func TestNMSKeepsBestOfOverlappingBoxes(t *testing.T) {
	dets := []Detection{
		{Box: Rect{0, 0, 100, 100}, Score: 0.91},
		{Box: Rect{5, 5, 105, 105}, Score: 0.99},
		{Box: Rect{300, 300, 400, 400}, Score: 0.95},
	}
	got := nms(dets, 0.3)
	if len(got) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(got))
	}
	if got[0].Score != 0.99 || got[1].Score != 0.95 {
		t.Fatalf("unexpected scores: %v, %v", got[0].Score, got[1].Score)
	}
}

func TestSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	if s := Similarity(a, a); math.Abs(float64(s)-1) > 1e-6 {
		t.Fatalf("identical vectors: %v", s)
	}
	if s := Similarity(a, []float32{0, 1, 0}); math.Abs(float64(s)) > 1e-6 {
		t.Fatalf("orthogonal vectors: %v", s)
	}
	if s := Similarity(a, []float32{1}); s != 0 {
		t.Fatalf("different lengths should give 0, got %v", s)
	}
}

func TestEmbeddingRoundTrip(t *testing.T) {
	in := []float32{0.5, -1.25, 3, 0}
	out, err := DecodeEmbedding(EncodeEmbedding(in))
	if err != nil {
		t.Fatal(err)
	}
	for i := range in {
		if in[i] != out[i] {
			t.Fatalf("index %d: %v != %v", i, in[i], out[i])
		}
	}
	if _, err := DecodeEmbedding([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected an error for a length that is not a multiple of 4")
	}
}

// TestEngineOnPhoto runs the real models. It needs:
//
//	UMBELA_ORT_LIB    path to libonnxruntime.so (or .dylib / .dll)
//	UMBELA_TEST_PHOTO a photo with at least one face
//
// and optionally UMBELA_TEST_PHOTO2 to compare the first face of both photos.
func TestEngineOnPhoto(t *testing.T) {
	lib := os.Getenv("UMBELA_ORT_LIB")
	photo := os.Getenv("UMBELA_TEST_PHOTO")
	if lib == "" || photo == "" {
		t.Skip("set UMBELA_ORT_LIB and UMBELA_TEST_PHOTO to run")
	}

	engine, err := NewEngine(lib)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	first := analyzeFile(t, engine, photo)
	if len(first) == 0 {
		t.Fatal("no faces found")
	}
	for i, f := range first {
		t.Logf("face %d: score=%.2f box=%+v", i, f.Score, f.Box)
	}

	if photo2 := os.Getenv("UMBELA_TEST_PHOTO2"); photo2 != "" {
		second := analyzeFile(t, engine, photo2)
		if len(second) == 0 {
			t.Fatal("no faces found in the second photo")
		}
		s := Similarity(first[1].Embedding, second[0].Embedding)
		t.Logf("similarity of photo1 face 1 and photo2 face 0: %.3f (same person above %.3f)", s, MatchThreshold)

		// Every pair of faces across both photos, to spot false matches.
		for i, a := range first {
			for j, b := range second {
				t.Logf("photo1 face %d vs photo2 face %d: %.3f", i, j, Similarity(a.Embedding, b.Embedding))
			}
		}
	}

	// Faces within the same photo are usually different people, so these
	// should fall below the threshold.
	for i := range first {
		for j := i + 1; j < len(first); j++ {
			t.Logf("photo1 face %d vs photo1 face %d: %.3f", i, j, Similarity(first[i].Embedding, first[j].Embedding))
		}
	}
}

func analyzeFile(t *testing.T, e *Engine, path string) []Face {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", path, err)
	}
	faces, err := e.Analyze(img)
	if err != nil {
		t.Fatal(err)
	}
	return faces
}
