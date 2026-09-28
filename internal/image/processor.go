// Package image provee las funciones de procesamiento de imágenes de umbela.
// Utiliza govips (binding de libvips) para operaciones de alta performance
// como generación de thumbnails y conversión a WebP.
//
// PREREQUISITO: libvips debe estar instalado en el sistema.
//
//	Ubuntu/Debian: apt-get install libvips-dev
//	macOS:         brew install vips
package image

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"umbela-server/internal/crypto"

	blurhash "github.com/buckket/go-blurhash"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/webp"
)

// ProcessResult has the result of processing an image.
type ProcessResult struct {
	ThumbPath string // Absolute route to the thumbnail
	Blurhash  string // Blurhash string
	Width     int    // Original width of the image
	Height    int    // Original height of the image
}

// The processor saves the configuration of Govips, so it only starts once.
type Processor struct {
	copyPath 	string
	thumbsDir   string // Thumbnails dir
	thumbWidth  int    // Target width in pixels
	thumbHeight int    // Target height (0 = proportional to width)
}

type EXIFData struct {
	Title       string
	Description string
	CapturedAt  int64
	Latitude    float64
	Longitude   float64
	Artist      string
}

type GoogleMetadata struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	PhotoTakenTime struct {
		Timestamp string `json:"timestamp"` // Google saves it like "1619998800"
		Formatted string `json:"formatted"`
	} `json:"photoTakenTime"`
	GeoData struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"geoData"`
}

// NewProcessor creates a new Processor and creates the Govips runtime.
// Called once at the start
// Input:  thumbsDir string, thumbWidth int, thumbHeight int
// Output: *Processor ready, or an error if there was an issue with govips
func NewProcessor(thumbsDir string, thumbWidth, thumbHeight int) (*Processor, error) {
	err:= vips.Startup(nil)
	if err!= nil{
		return nil, fmt.Errorf("starting new process of govips %w", err)
	}
	os.MkdirAll(thumbsDir, 0755)
	return &Processor{thumbsDir: thumbsDir, thumbWidth: thumbWidth, thumbHeight: thumbHeight}, nil
}

// Shutdown cleans the resources of govips. Use with defer after initializing.
func (p *Processor) Shutdown() {
	vips.Shutdown()
}

func (p *Processor) SaveImage(imageBytes []byte, filePath string){
	// TODO: encryptedBytes, err := crypto.EncryptGCM()
	return 
}

// GenerateThumbnail creates the thumbnail of theimage in the srcPath,
// It saves it in thumbsDir/<hash>.webp and returns ProcessResult.
// Input:  srcPath string — og image
//  	   hash string    — hash SHA256 of the file (used as name of the thumbnail)
//
// Output: ProcessResult with ThumbPath and dimensions, or an error
func (p *Processor) GenerateThumbnailAndBlurHash(srcPath, hash string) (*ProcessResult, error) {
	thumbPath := filepath.Join(p.thumbsDir, hash+".webp")
	vipsImage, err := vips.NewImageFromFile(srcPath)

	if err != nil {
		return nil, fmt.Errorf("opening image with govips: %w", err)
	}
	defer vipsImage.Close()

	// original dimensions
	originalWidth := vipsImage.Width()
	originalHeight := vipsImage.Height()

	// Reducing the image with Lanczos3 algorithm
	scale := float64(p.thumbWidth) / float64(originalWidth)
	err = vipsImage.Resize(scale, vips.KernelLanczos3)
	if err != nil {
		return nil, fmt.Errorf("scailing image: %w", err)
	}

	// export to webp
	webpBytes, _, err := vipsImage.ExportWebp(vips.NewWebpExportParams())
	if err != nil {
		return nil, fmt.Errorf("exporting to webp: %w", err)
	}

	goImg, err := webp.Decode(bytes.NewReader(webpBytes))
	if err != nil {
		return nil, fmt.Errorf("decoding webp: %w", err)
	}

	// Blurhash
	imageBlurhash, err := blurhash.Encode(4, 3, goImg)
	
	// save webp image
	err = os.WriteFile(thumbPath, webpBytes, 0644)
	if err != nil {
		return nil, fmt.Errorf("writing files on disk: %w", err)
	}

	return &ProcessResult{
		ThumbPath: thumbPath,
		Blurhash:  imageBlurhash, 
		Width:     originalWidth,
		Height:    originalHeight,
	}, nil
}

// Make the SHA256 hash of the file 
// Input:  filePath string — ruta al archivo a hashear
// Output: string hexadecimal del hash (64 caracteres), o error de I/O
func ComputeSHA256(filePath string) (string, error) {
	openedFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening file %w", err)
	}
	defer openedFile.Close()

	hasher := sha256.New()
	_, err = io.Copy(hasher, openedFile)
	if err != nil {
		return "", fmt.Errorf("Error hashing the file: %w", err)
	}
	hashString := fmt.Sprintf("%x", hasher.Sum(nil))
	return hashString, nil
}

func ExtractEXIF(srcPath string) (*EXIFData, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open the file: %w", err)
	}
	defer f.Close()

	imageMetadata := &EXIFData{}

	x, err := exif.Decode(f)
	if err != nil {
		if exif.IsCriticalError(err) {
			if filepath.Ext(srcPath) == ".HEIC" {
				fmt.Println("Can't get metadata from HEIC images at the moment")
				return imageMetadata, nil
			}
			fmt.Printf("Warning: Could not decode EXIF for %s: %v\n", srcPath, err)
			return imageMetadata, nil
		}
	}

	lat, lon, _ := x.LatLong()
	capturedAt, _ := x.DateTime()
	description, _ := x.Get(exif.ImageDescription)
	artist, _ := x.Get(exif.Artist)

	imageMetadata.Latitude = lat
	imageMetadata.Longitude = lon
	imageMetadata.Title = filepath.Base(srcPath)

	if description != nil {
		if val, err := description.StringVal(); err == nil {
			imageMetadata.Description = val
		}
	}
	if artist != nil {
		if val, err := artist.StringVal(); err == nil {
			imageMetadata.Artist = val
		}
	}
	if !capturedAt.IsZero() {
		imageMetadata.CapturedAt = capturedAt.Unix()
	}

	return imageMetadata, nil
}

// This only works with Google Takout's json format
func ProcessMetadataJSON(srcPath string) (*EXIFData, error) {
	imageMetadata := &EXIFData{}

	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("openning the json file: %w", err)
	}
	defer f.Close()

	var jsonMetadata GoogleMetadata

	err = json.NewDecoder(f).Decode(&jsonMetadata)

	if err!=nil{
		fmt.Printf("Warning: Could not decode JSON: %s: %v\n", srcPath, err)
		return imageMetadata, nil
	}

	timestamp, err := strconv.ParseInt(jsonMetadata.PhotoTakenTime.Timestamp, 10, 64)

	if err!= nil{
		timestamp=time.Now().Unix()
	}

	imageMetadata.Title = jsonMetadata.Title
	imageMetadata.Description = jsonMetadata.Description
	imageMetadata.CapturedAt = timestamp 
	imageMetadata.Latitude = jsonMetadata.GeoData.Latitude
	imageMetadata.Longitude = jsonMetadata.GeoData.Longitude
	imageMetadata.Artist = "user" 

	return imageMetadata, nil
}

// {
// 	Title       string
// 	Description string
// 	CapturedAt  int64
// 	Latitude    float64
// 	Longitude   float64
// 	Artist      string
// }
