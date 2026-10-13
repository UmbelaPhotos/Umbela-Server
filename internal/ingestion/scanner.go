package ingestion

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var supportedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".heic": true,
	".webp": true,
}

// FileGroup joins JSON with image
type FileGroup struct {
	ImagePaths []string
	JSONPath   string
}

func ScanDirectory(root string, depth int, extractDir string) ([]*FileGroup, error) {
	var groups []*FileGroup
	var zipDirs []string
	groupMap := make(map[string]*FileGroup)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Process supported images
		if supportedImageExts[ext] {
			baseName := strings.TrimSuffix(path, ext)
			if group, exists := groupMap[baseName]; exists {
				group.ImagePaths = append(group.ImagePaths, path)
			} else {
				newGroup := &FileGroup{ImagePaths: []string{path}}
				groupMap[baseName] = newGroup
				groups = append(groups, newGroup)
			}
			return nil
		}

		// Process metadata in .json
		if ext == ".json" {
			fileName := strings.TrimSuffix(filepath.Base(path), ".json")
			
			var validNames []string
			for partialName := range strings.SplitSeq(fileName, ".") {
				extWithDot := "."+ strings.ToLower(partialName)
				if _, ok := supportedImageExts[extWithDot]; ok{
					break
				}
				validNames = append(validNames, partialName)
			}
			baseName:=strings.Join(validNames,".")

			if group, exists := groupMap[baseName]; exists {
				group.JSONPath = path
			} else {
				// We create a File Group, if there are no matching images it won't be counted.
				newGroup := &FileGroup{JSONPath: path}
				groupMap[baseName] = newGroup
			}
		}
		if ext == ".zip" && depth>0 {
			zipDirs = append(zipDirs, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("scanning directory: %w", err)
	}

	// Removing empty FileGroups
	var validGroups []*FileGroup
	for _, g := range groups {
		if len(g.ImagePaths) > 0 {
			validGroups = append(validGroups, g)
		}
	}

	for _, z := range zipDirs{
		zipExtractedPath, err :=ExtractZip(z, extractDir)
		if err!=nil{
			fmt.Printf("Error extracting zip %v: %v\n", z, err)
			continue
		}
		zipFileGroup, err := ScanDirectory(zipExtractedPath, depth-1, extractDir)
		validGroups = append(validGroups, zipFileGroup...)
	}

	return validGroups, nil
}

func ExtractZip(zipPath string, tempDir string) (string, error) {
	batchID := filepath.Base(zipPath)
	extractDir := filepath.Join(tempDir, batchID)
	
	if err := os.MkdirAll(extractDir, 0700); err != nil {
		return "", fmt.Errorf("creating temporal directory: %w", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	const numWorkers = 4

	filesChan := make(chan *zip.File, 100)
	var wg sync.WaitGroup
	var extractErr error
	var errOnce sync.Once

	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range filesChan {
				if err := extractSingleFile(f, extractDir); err != nil {
					errOnce.Do(func() { extractErr = err })
				}
			}
		}()
	}

	for _, f := range r.File {
		filesChan <- f
	}
	close(filesChan)

	wg.Wait()

	fmt.Printf("zip extracted in: %s\n", extractDir)
	return extractDir, extractErr
}

func extractSingleFile(f *zip.File, extractDir string) error {
	fpath := filepath.Join(extractDir, f.Name)
	if !strings.HasPrefix(fpath, filepath.Clean(extractDir)+string(os.PathSeparator)) {
		return nil 
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(fpath, 0700)
	}
	if err := os.MkdirAll(filepath.Dir(fpath), 0700); err != nil {
		return err
	}

	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(outFile, rc)
	return err
}