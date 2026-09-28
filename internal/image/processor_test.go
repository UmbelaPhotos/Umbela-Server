package image

import (
	"fmt"
	"testing"
)

func TestGenerateThumbnail(t *testing.T) {
    p := &Processor{
        thumbsDir:  "/home/user/umbela/umbela-server/tests/test_thumbs",
        thumbWidth: 200,
    }

    filePath:= "/home/user/umbela/umbela-server/tests/foto3.jpg"
    jsonFilePath:= "/home/user/umbela/umbela-server/tests/PXL_20240301_175526140.jpg.supplemental-metada.json"
    processResult, err := p.GenerateThumbnailAndBlurHash(filePath, "hash123")

    if err != nil{
        fmt.Printf("No such file %s", filePath)
        return
    }

    fmt.Println(processResult.ThumbPath)
    fmt.Println(processResult.Blurhash)
    
    hashedString, err:=ComputeSHA256(filePath)
    if err != nil{
        fmt.Println("No funciono bro")
    }
    fmt.Println(hashedString)

    metadata, err:=ExtractEXIF(filePath)
    fmt.Println(metadata)

    metadata, err = ProcessMetadataJSON(jsonFilePath)
    fmt.Println("Json metadata:")
    fmt.Println(metadata.Latitude)
    fmt.Println(metadata.Longitude)
    fmt.Println(metadata.Artist)
    if err != nil {
        t.Fatalf("La función falló: %v", err)
    }
}