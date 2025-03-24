package main

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

const outputFile = "project_structure.txt"

// List of file extensions and names to skip
var skipExtensions = map[string]bool{
	".exe":    true,
	".dll":    true,
	".so":     true,
	".dylib":  true,
	".bin":    true,
	".dat":    true,
	".db":     true,
	".png":    true,
	".jpg":    true,
	".jpeg":   true,
	".gif":    true,
	".mp3":    true,
	".mp4":    true,
	".zip":    true,
	".tar":    true,
	".gz":     true,
	".md":     true, // Added .md files
	".log":    true,
	".json":   true,
	".txt":    true,
	".go_old": true,
}

var skipFiles = map[string]bool{
	outputFile: true,
	// "go.mod":                      true,
	"go.sum":                  true,
	".gitignore":              true,
	"getProjectStructure1.go": true,
}

// List of directories to skip
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"bin":          true,
	"obj":          true,
	"cache_data":   true,
	"testdata":     true,
	// "cmd":          true,
}

func main() {
	root := "." // Start from the current directory

	// Open the output file
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	// Write project structure
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip files and directories we don't want to include
		if shouldSkipFile(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		indent := strings.Repeat("  ", strings.Count(relPath, string(os.PathSeparator)))
		if info.IsDir() {
			fmt.Fprintf(file, "%s%s/\n", indent, info.Name())
		} else {
			fmt.Fprintf(file, "%s%s\n", indent, info.Name())
		}

		return nil
	})
	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		return
	}

	// Append file contents
	err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and files we don't want to include
		if info.IsDir() || shouldSkipFile(path) {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		fmt.Fprintf(file, "\n\n--- %s ---\n", relPath)
		if by, err := file.Write(content); err != nil {
			println(by)
			return err
		}

		return nil
	})
	if err != nil {
		fmt.Printf("Error writing file contents: %v\n", err)
		return
	}

	fmt.Println("Project structure and file contents written to", outputFile)
}

func shouldSkipFile(path string) bool {
	// Check if file or directory name is in the skip list
	base := filepath.Base(path)
	if skipFiles[base] || skipDirs[base] {
		return true
	}

	// Check if any parent directory is in the skip list
	for dir := filepath.Dir(path); dir != "." && dir != string(filepath.Separator); dir = filepath.Dir(dir) {
		if skipDirs[filepath.Base(dir)] {
			return true
		}
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(path))
	if skipExtensions[ext] {
		return true
	}

	// Check MIME type
	mimeType := mime.TypeByExtension(ext)
	if strings.HasPrefix(mimeType, "application/") && mimeType != "application/json" && mimeType != "application/xml" {
		return true
	}

	// Check if file is likely binary
	if isBinary(path) {
		return true
	}

	return false
}

func isBinary(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read first 512 bytes
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil {
		return false
	}

	// Check for null bytes (common in binary files)
	for _, b := range buffer[:n] {
		if b == 0 {
			return true
		}
	}

	return false
}
