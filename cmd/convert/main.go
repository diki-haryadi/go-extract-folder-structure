package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	sourcePath    string
	outputPath    string
	separator     string
	skipFolders   string
	skipFolderMap map[string]bool
}

func main() {
	config := parseFlags()

	// Process skip folders into a map for faster lookup
	config.skipFolderMap = make(map[string]bool)
	if config.skipFolders != "" {
		for _, folder := range strings.Split(config.skipFolders, ",") {
			config.skipFolderMap[strings.TrimSpace(folder)] = true
		}
	}

	// Validate source path exists
	if _, err := os.Stat(config.sourcePath); os.IsNotExist(err) {
		fmt.Printf("Error: Source directory '%s' does not exist\n", config.sourcePath)
		os.Exit(1)
	}

	// Create or open project.md file
	outputFile, err := os.Create(config.outputPath)
	if err != nil {
		fmt.Printf("Error creating output file '%s': %v\n", config.outputPath, err)
		os.Exit(1)
	}
	defer outputFile.Close()

	// Get absolute path for better error messages
	absSourcePath, err := filepath.Abs(config.sourcePath)
	if err != nil {
		fmt.Printf("Error getting absolute path: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processing directory: %s\n", absSourcePath)
	fmt.Printf("Output file: %s\n", config.outputPath)
	if len(config.skipFolderMap) > 0 {
		fmt.Printf("Skipping folders: %s\n", config.skipFolders)
	}
	fmt.Println()

	// Walk through the directory
	err = filepath.Walk(config.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %v", path, err)
		}

		// Get relative path for cleaner output and checking skip folders
		relPath, err := filepath.Rel(config.sourcePath, path)
		if err != nil {
			relPath = path // Fallback to full path if relative path fails
		}

		// Check if this path should be skipped
		if shouldSkip(relPath, config.skipFolderMap) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip the output file itself and hidden files/directories
		if info.Name() == filepath.Base(config.outputPath) || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip directories themselves
		if info.IsDir() {
			return nil
		}

		// Skip binary files and certain extensions
		ext := strings.ToLower(filepath.Ext(path))
		if isSkippableFile(ext) {
			return nil
		}

		// Read file content
		content, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Printf("Warning: Error reading file %s: %v\n", path, err)
			return nil
		}

		// Write the path and content to the output file using the specified separator
		_, err = fmt.Fprintf(outputFile, "%s %s\n%s\n\n", config.separator, relPath, string(content))
		if err != nil {
			return fmt.Errorf("error writing to output file: %v", err)
		}

		fmt.Printf("Processed: %s\n", relPath)
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nSuccessfully created project documentation!")
}

func shouldSkip(path string, skipFolders map[string]bool) bool {
	parts := strings.Split(path, string(os.PathSeparator))
	for i := range parts {
		if skipFolders[parts[i]] {
			return true
		}
	}
	return false
}

func parseFlags() Config {
	config := Config{}

	// Define command line flags
	flag.StringVar(&config.sourcePath, "source", ".", "Source directory path to process")
	flag.StringVar(&config.outputPath, "output", "project.md", "Output markdown file path")
	flag.StringVar(&config.separator, "separator", "/==", "Path separator symbol (default: /==)")
	flag.StringVar(&config.skipFolders, "skip", "", "Comma-separated list of folders to skip (e.g., 'node_modules,vendor,build')")

	// Add custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s -source ./myproject -output docs/project.md\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -source ./myproject -separator '/**' -skip 'node_modules,vendor'\n", os.Args[0])
	}

	flag.Parse()
	return config
}

func isSkippableFile(ext string) bool {
	// List of extensions to skip
	skipExtensions := map[string]bool{
		".exe":   true,
		".dll":   true,
		".so":    true,
		".dylib": true,
		".bin":   true,
		".obj":   true,
		".o":     true,
		".a":     true,
		".lib":   true,
		".pyc":   true,
		".pyo":   true,
		".jpg":   true,
		".jpeg":  true,
		".png":   true,
		".gif":   true,
		".pdf":   true,
		".zip":   true,
		".tar":   true,
		".gz":    true,
		".rar":   true,
	}

	return skipExtensions[ext]
}
