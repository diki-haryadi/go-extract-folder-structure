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
	sourcePath string
	outputPath string
}

func main() {
	config := parseFlags()

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
	fmt.Printf("Output file: %s\n\n", config.outputPath)

	// Walk through the directory
	err = filepath.Walk(config.sourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %v", path, err)
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

		// Get relative path for cleaner output
		relPath, err := filepath.Rel(config.sourcePath, path)
		if err != nil {
			relPath = path // Fallback to full path if relative path fails
		}

		// Write the path and content to the output file
		_, err = fmt.Fprintf(outputFile, "// %s\n%s\n\n", relPath, string(content))
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

func parseFlags() Config {
	config := Config{}

	// Define command line flags
	flag.StringVar(&config.sourcePath, "source", ".", "Source directory path to process")
	flag.StringVar(&config.outputPath, "output", "project.md", "Output markdown file path")

	// Add custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -source ./myproject -output docs/project.md\n", os.Args[0])
	}

	flag.Parse()
	return config
}

func isSkippableFile(ext string) bool {
	// List of extensions to skip
	skipExtensions := map[string]bool{
		".git":   true,
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
