package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	inputFile string
	outputDir string
	separator string
}

func main() {
	config := parseFlags()

	// Check if input file exists
	if _, err := os.Stat(config.inputFile); os.IsNotExist(err) {
		fmt.Printf("Error: Input file '%s' does not exist\n", config.inputFile)
		os.Exit(1)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(config.outputDir, 0755); err != nil {
		fmt.Printf("Error creating output directory '%s': %v\n", config.outputDir, err)
		os.Exit(1)
	}

	// Open input file
	file, err := os.Open(config.inputFile)
	if err != nil {
		fmt.Printf("Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentPath string
	var currentContent strings.Builder
	isReadingContent := false

	// Process the file line by line
	for scanner.Scan() {
		line := scanner.Text()

		// Check if line starts with separator (new file path)
		if strings.HasPrefix(line, config.separator) {
			// If we were reading content, write the previous file
			if isReadingContent && currentPath != "" {
				if err := writeFile(config.outputDir, currentPath, currentContent.String()); err != nil {
					fmt.Printf("Error writing file '%s': %v\n", currentPath, err)
				}
			}

			// Get new path and prepare for new content
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, config.separator))
			currentContent.Reset()
			isReadingContent = true
			continue
		}

		// Add line to current content if we're reading a file
		if isReadingContent {
			if currentContent.Len() > 0 {
				currentContent.WriteString("\n")
			}
			currentContent.WriteString(line)
		}
	}

	// Write the last file if any
	if isReadingContent && currentPath != "" {
		if err := writeFile(config.outputDir, currentPath, currentContent.String()); err != nil {
			fmt.Printf("Error writing file '%s': %v\n", currentPath, err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading input file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully reconstructed project files!")
}

func writeFile(baseDir, path, content string) error {
	fullPath := filepath.Join(baseDir, path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %v", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	fmt.Printf("Created: %s\n", path)
	return nil
}

func parseFlags() Config {
	config := Config{}

	// Define command line flags
	flag.StringVar(&config.inputFile, "input", "project.md", "Input markdown file")
	flag.StringVar(&config.outputDir, "output", "reconstructed", "Output directory for reconstructed project")
	flag.StringVar(&config.separator, "separator", "/==", "Path separator used in the markdown file (default: /==)")

	// Add custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -input docs/project.md -output ./reconstructed -separator '/=='\n", os.Args[0])
	}

	flag.Parse()
	return config
}
