// Package image provee las funciones de procesamiento de imágenes de Allium.
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
	"time"
	"strconv"
	"path/filepath"

	blurhash "github.com/buckket/go-blurhash"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/rwcarlsen/goexif/exif"
	"golang.org/x/image/webp"
)

// ProcessResult contiene el resultado de procesar una imagen.
type ProcessResult struct {
	ThumbPath string // Ruta absoluta al thumbnail generado
	Blurhash  string // Cadena blurhash para placeholder de carga
	Width     int    // Ancho original de la imagen en píxeles
	Height    int    // Alto original de la imagen en píxeles
}

// Processor encapsula la configuración para el procesamiento de imágenes.
// Inicializar una vez y reutilizar (govips no es barato de arrancar).
type Processor struct {
	copyPath 	string
	thumbsDir   string // Directorio donde guardar los thumbnails
	thumbWidth  int    // Ancho objetivo del thumbnail en píxeles
	thumbHeight int    // Alto objetivo (0 = proporcional al ancho)
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
		Timestamp string `json:"timestamp"` // Google lo manda como texto "1619998800"
		Formatted string `json:"formatted"`
	} `json:"photoTakenTime"`
	GeoData struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"geoData"`
}

// NewProcessor crea un Processor y arranca el runtime de govips.
// Debe llamarse una sola vez al inicio de la aplicación.
//
// Input:  thumbsDir string, thumbWidth int, thumbHeight int
// Output: *Processor listo, o error si govips no puede iniciar
func NewProcessor(thumbsDir string, thumbWidth, thumbHeight int) (*Processor, error) {
	// TODO: Implementar
	// 1. Llamar govips.Startup(nil) para iniciar libvips
	// 2. os.MkdirAll(thumbsDir, 0755) para asegurar el directorio
	// 3. Devolver &Processor{...}
	vips.Startup(nil)
	os.MkdirAll(thumbsDir, 0755)
	return &Processor{thumbsDir: thumbsDir, thumbWidth: thumbWidth, thumbHeight: thumbHeight}, nil
}

// Shutdown limpia los recursos de govips. Llamar con defer al cerrar la app.
func (p *Processor) Shutdown() {
	// TODO: Implementar
	// 1. Llamar govips.Shutdown()
	vips.Shutdown()
}

// GenerateThumbnail crea un thumbnail a partir de la imagen en srcPath,
// la guarda en thumbsDir/<hash>.webp y devuelve el ProcessResult.
//
// La conversión a WebP es obligatoria para eficiencia en la galería web.
//
// Input:  srcPath string — ruta al archivo de imagen original
//
//	hash string    — hash SHA256 del archivo (usado como nombre del thumb)
//
// Output: ProcessResult con ThumbPath y dimensiones, o error
func (p *Processor) GenerateThumbnailAndBlurHash(srcPath, hash string) (*ProcessResult, error) {
	// 1. Definir la ruta final donde se guardará el archivo (.webp)
	thumbPath := filepath.Join(p.thumbsDir, hash+".webp")

	// 2. Cargar la imagen original en memoria a través del motor global de vips
	vipsImage, err := vips.NewImageFromFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir imagen original: %w", err)
	}
	// Nos aseguramos de liberar la memoria de esta imagen individual al terminar la función
	defer vipsImage.Close()

	// Guardamos las dimensiones originales antes de cambiarles el tamaño
	originalWidth := vipsImage.Width()
	originalHeight := vipsImage.Height()

	// 3. CORRECCIÓN DEL RESIZE: Convertimos a float64 para obtener el porcentaje decimal exacto
	// Ejemplo: 200.0 / 1920.0 = 0.1041 (Reducir al 10.4%)
	scale := float64(p.thumbWidth) / float64(originalWidth)

	// Aplicamos el redimensionado usando el algoritmo matemático Lanczos
	err = vipsImage.Resize(scale, vips.KernelLanczos3)
	if err != nil {
		return nil, fmt.Errorf("error al redimensionar imagen: %w", err)
	}

	// 4. Exportar la imagen procesada a formato WebP (esto nos da una lista de bytes)
	webpBytes, _, err := vipsImage.ExportWebp(vips.NewWebpExportParams())

	if err != nil {
		return nil, fmt.Errorf("error al exportar a WebP: %w", err)
	}

	goImg, err := webp.Decode(bytes.NewReader(webpBytes))
	if err != nil {
		return nil, fmt.Errorf("error decodificando webp para blurhash: %w", err)
	}

	imageBlurhash, err := blurhash.Encode(4, 3, goImg)
	fmt.Println(imageBlurhash)
	// 5. Guardar esos bytes en el disco duro en la ruta que armamos al principio
	err = os.WriteFile(thumbPath, webpBytes, 0644)
	if err != nil {
		return nil, fmt.Errorf("error al guardar el archivo en disco: %w", err)
	}

	// 6. Devolver el resultado con los datos de la imagen ORIGINAL y la ruta del thumbnail
	return &ProcessResult{
		ThumbPath: thumbPath,
		Blurhash:  imageBlurhash, // (Por ahora vacío como dice tu struct)
		Width:     originalWidth,
		Height:    originalHeight,
	}, nil
}

// Input:  filePath string — ruta al archivo a hashear
// Output: string hexadecimal del hash (64 caracteres), o error de I/O
func ComputeSHA256(filePath string) (string, error) {
	// TODO: Implementar
	// 1. os.Open(filePath)
	// 2. sha256.New() + io.Copy(hasher, file)
	// 3. fmt.Sprintf("%x", hasher.Sum(nil))
	openedFile, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("Error : %w", err)
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
			return &EXIFData{}, nil
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

func ProcessMetadataJSON(srcPath string) (*EXIFData, error) {
	// This only works with Google Takout's json format
	imageMetadata := &EXIFData{}

	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("Coudnt open the file: %w", err)
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
