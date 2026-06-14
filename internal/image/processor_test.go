package image

import (
	"fmt"
	"testing"
)

func TestGenerateThumbnail(t *testing.T) {
    p := &Processor{
        thumbsDir:  "/home/user/Allium/allium-server/tests/test_thumbs",
        thumbWidth: 200,
    }

    filePath:= "/home/user/Allium/allium-server/tests/foto2.jpeg"
    jsonFilePath:= "/home/user/Allium/allium-server/tests/PXL_20240301_175526140.jpg.supplemental-metada.json"
    _, err := p.GenerateThumbnailAndBlurHash(filePath, "hash123")
    hashedString, err:=ComputeSHA256(filePath)
    if err != nil{
        fmt.Println("No funciono bro")
    }
    fmt.Println(hashedString)

    metadata, err:=ExtractEXIF(filePath)
    fmt.Println(metadata)

    metadata, err = ProcessMetadataJSON(jsonFilePath)
    fmt.Println("Json metadata:")
    fmt.Println(metadata)
    if err != nil {
        t.Fatalf("La función falló: %v", err)
    }
}