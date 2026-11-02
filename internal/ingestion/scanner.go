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

	"github.com/schollz/progressbar/v3"
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
	Album      string
}

func ScanDirectory(root string, lastDirIsAlbum bool) ([]*FileGroup, error) {
	var groups []*FileGroup
	groupMap := make(map[string]*FileGroup)
	extractDir, err := os.MkdirTemp(root, "umbela-extract-*")
	if err != nil {
		return nil, fmt.Errorf("creating directory: %w", err)
	}
	defer os.RemoveAll(extractDir)

	zips, err := SearchZips(root)
	if err != nil {
		return nil, err
	}

	for _, z := range zips {
		_, err := ExtractZip(z, root, extractDir)
		if err != nil {
			fmt.Printf("Error extracting zip %v: %v\n", z, err)
			continue
		}
	}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))

		// Process supported images
		if supportedImageExts[ext] {
			baseName := strings.TrimSuffix(filepath.Base(path), ext)
			group, exists := groupMap[baseName]
			if !exists {
				group = &FileGroup{}
				groupMap[baseName] = group
				groups = append(groups, group)
			}
			group.ImagePaths = append(group.ImagePaths, path)

			if dir := filepath.Dir(path); dir != root && dir != extractDir {
				group.Album = filepath.Base(dir)
			}
			return nil
		}

		// Process metadata in .json
		if ext == ".json" {
			fileName := strings.TrimSuffix(filepath.Base(path), ".json")

			var validNames []string
			for partialName := range strings.SplitSeq(fileName, ".") {
				extWithDot := "." + strings.ToLower(partialName)
				if _, ok := supportedImageExts[extWithDot]; ok {
					break
				}
				validNames = append(validNames, partialName)
			}
			baseName := strings.Join(validNames, ".")

			if group, exists := groupMap[baseName]; exists {
				group.JSONPath = path
			} else {
				// We create a File Group, if there are no matching images it won't be counted.
				group := &FileGroup{JSONPath: path}
				groupMap[baseName] = group
				groups = append(groups, group)
			}
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

	return validGroups, nil
}

func SearchZips(dir string) ([]string, error) {
	var zips []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if strings.EqualFold(filepath.Ext(d.Name()), ".zip") {
			zips = append(zips, path)
		}
		return nil
	})
	return zips, err
}

func ExtractZip(zipPath string, dir string, extractDir string) (string, error) {
	batchID := filepath.Base(zipPath)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	total := len(r.File)
	fmt.Printf("\nExtracting Zip: %s\n", batchID)
	bar := progressbar.NewOptions(total,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("file"),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetWidth(20),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[magenta]=[reset]",
			SaucerHead:    "[magenta]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	const numWorkers = 4

	filesChan := make(chan *zip.File, 100)
	var wg sync.WaitGroup
	var extractErr error
	var errOnce sync.Once

	for range numWorkers {
		wg.Go(func() {
			for f := range filesChan {
				if err := extractSingleFile(f, extractDir); err != nil {
					errOnce.Do(func() { extractErr = err })
				}
				_ = bar.Add(1)
			}
		})
	}

	for _, f := range r.File {
		filesChan <- f
	}
	close(filesChan)

	wg.Wait()

	fmt.Printf("\nZip extracted in: %s\n", extractDir)
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
