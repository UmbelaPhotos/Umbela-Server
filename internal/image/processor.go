package image

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// NewProcessor initializes Govips and prepares directories
func NewProcessor(thumbsDir string, thumbWidth int, thumbHeight int) (*Processor, error) {
	err:= vips.Startup(nil)
	vips.LoggingSettings(nil, vips.LogLevelError)
	if err!= nil{
		return nil, fmt.Errorf("starting new process of govips %w", err)
	}
	os.MkdirAll(thumbsDir, 0700)
	return &Processor{
		thumbsDir: thumbsDir, 
		thumbWidth: thumbWidth, 
		thumbHeight: thumbHeight,
		}, nil
}

// Shutdown cleans the resources of govips. Use with defer after initializing.
func (p *Processor) Shutdown() {
	vips.Shutdown()
}

func (p *Processor) SaveImage(filePath string, imageBytes []byte,  masterKey []byte) error {
	encryptedBytes, err := crypto.EncryptGCM(imageBytes, masterKey)
	if err!=nil{
		return fmt.Errorf("encrypting image: %w", err)
	}

	err = os.WriteFile(filePath, encryptedBytes, 0600)
	if err!=nil{
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

func (p *Processor) GenerateThumbnailAndBlurHash(srcPath, hash string, masterKey []byte) (*ProcessResult, error) {
	thumbPath := filepath.Join(p.thumbsDir, hash+".umb")

	vipsImage, err := vips.NewImageFromFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("opening image with govips: %w", err)
	}
	defer vipsImage.Close()
	
	_ = vipsImage.AutoRotate()

	// original dimensions
	originalWidth := vipsImage.Width()
	originalHeight := vipsImage.Height()

	imageBlurhash, err:=GenerateBlurhash(vipsImage, originalWidth)

	// Reducing the image with Lanczos3 algorithm
	scale := float64(p.thumbWidth) / float64(originalWidth)
	if err := vipsImage.Resize(scale, vips.KernelLanczos3); err != nil {
		return nil, fmt.Errorf("scaling thumbnail: %w", err)
	}

	ep := vips.NewWebpExportParams()
	ep.Quality = 80
	webpBytes, _, err := vipsImage.ExportWebp(ep)
	if err != nil {
		return nil, fmt.Errorf("exporting webp: %w", err)
	}

	if err:= p.SaveImage(thumbPath, webpBytes, masterKey); err!=nil{
		return nil, fmt.Errorf("saving encrypted thumbnail: %w", err)
	}

	return &ProcessResult{
		ThumbPath: thumbPath,
		Blurhash:  imageBlurhash, 
		Width:     originalWidth,
		Height:    originalHeight,
	}, nil
}

func GenerateBlurhash(vipsImage *vips.ImageRef, originalWidth int) (string, error) {
	blurImg, err := vipsImage.Copy()
	if err != nil {
		return "", fmt.Errorf("copying vips image for blurhash: %w", err)
	}
	defer blurImg.Close() // Se libera en CGO inmediatamente al salir de la función

	blurScale := 32.0 / float64(originalWidth)
	if err := blurImg.Resize(blurScale, vips.KernelNearest); err != nil {
		return "", fmt.Errorf("resizing image for blurhash: %w", err)
	}

	ep := vips.NewWebpExportParams()
	blurWebpBytes, _, err := blurImg.ExportWebp(ep)
	if err != nil {
		return "", fmt.Errorf("exporting webp buffer for blurhash: %w", err)
	}

	goImg, err := webp.Decode(bytes.NewReader(blurWebpBytes))
	if err != nil {
		return "", fmt.Errorf("decoding webp buffer for blurhash: %w", err)
	}

	bh, err := blurhash.Encode(4, 3, goImg)
	if err != nil {
		return "", fmt.Errorf("encoding blurhash: %w", err)
	}

	return bh, nil 
}

func ExtractEXIF(srcPath string) (*EXIFData, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open the file: %w", err)
	}
	defer f.Close()

	imageMetadata := &EXIFData{
		Title: filepath.Base(srcPath),
	}

	x, err := exif.Decode(f)
	if err != nil {
		if exif.IsCriticalError(err) {
			ext := strings.ToLower(filepath.Ext(srcPath))
			if ext == ".heic" {
				fmt.Println("Info: HEIC metadata extraction not supported yet")
			} else {
				fmt.Printf("Warning: Could not decode EXIF for %s: %v\n", srcPath, err)
			}
		}
		if info, statErr := f.Stat(); statErr == nil {
			imageMetadata.CapturedAt = info.ModTime().Unix()
		} else {
			imageMetadata.CapturedAt = time.Now().Unix()
		}
		return imageMetadata, nil
	}

	if lat, lon, err := x.LatLong(); err == nil {
		imageMetadata.Latitude = lat
		imageMetadata.Longitude = lon
	}

	if capturedAt, err := x.DateTime(); err == nil && !capturedAt.IsZero() {
		imageMetadata.CapturedAt = capturedAt.Unix()
	} else if info, err := f.Stat(); err == nil {
		imageMetadata.CapturedAt = info.ModTime().Unix()
	} else {
		imageMetadata.CapturedAt = time.Now().Unix()
	}

	if descTag, err := x.Get(exif.ImageDescription); err == nil {
		if val, err := descTag.StringVal(); err == nil {
			imageMetadata.Description = val
		}
	}

	if artistTag, err := x.Get(exif.Artist); err == nil {
		if val, err := artistTag.StringVal(); err == nil {
			imageMetadata.Artist = val
		}
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
