package faces

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	// AlignedSize is the side in pixels of the aligned face crop SFace expects.
	AlignedSize = 112
	// EmbeddingSize is the length of the vector SFace produces.
	EmbeddingSize = 128
	// MatchThreshold is the cosine similarity above which two faces are
	// merged into the same person automatically.
	MatchThreshold = 0.4
	// SuggestThreshold is the cosine similarity (the value recommended for
	// SFace) above which two faces might be the same person. Between this and
	// MatchThreshold the pair should be suggested to the user, not merged.
	SuggestThreshold = 0.363
)

type Point struct {
	X, Y float32
}

type Rect struct {
	X1, Y1, X2, Y2 float32
}

// Detection is a face found in an image, in the coordinates of that image.
type Detection struct {
	Box Rect
	// Landmarks are the person's right eye, left eye, nose tip, right mouth
	// corner and left mouth corner. In the image they appear from left to right.
	Landmarks [5]Point
	Score     float32
}

// Face is a detection together with its embedding.
type Face struct {
	Detection
	Embedding []float32 // L2-normalized
}

// Similarity returns the cosine similarity of two embeddings, in [-1, 1].
func Similarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return float32(dot / (math.Sqrt(na) * math.Sqrt(nb)))
}

// Merge returns the centroid of a group of embeddings after adding one more.
// centroid is the normalized mean of the samples embeddings already in the
// group. The result is normalized too.
func Merge(centroid []float32, samples int, e []float32) []float32 {
	out := make([]float32, len(e))
	var norm float64
	for i := range e {
		var c float32
		if i < len(centroid) {
			c = centroid[i]
		}
		out[i] = c*float32(samples) + e[i]
		norm += float64(out[i]) * float64(out[i])
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return out
	}
	for i := range out {
		out[i] = float32(float64(out[i]) / norm)
	}
	return out
}

// EncodeEmbedding serializes an embedding for the Face.Embedding column.
func EncodeEmbedding(e []float32) []byte {
	out := make([]byte, 4*len(e))
	for i, v := range e {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(v))
	}
	return out
}

// DecodeEmbedding is the inverse of EncodeEmbedding.
func DecodeEmbedding(b []byte) ([]float32, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("embedding has %d bytes, not a multiple of 4", len(b))
	}
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return out, nil
}
