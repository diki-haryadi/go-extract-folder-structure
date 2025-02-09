# Basic configuration
GO=go
MAIN_FILE=main.go

.PHONY: all help convert convert-dir convert-custom

# Default target
all: help

# Convert current directory
convert:
	$(GO) run $(MAIN_FILE)

# Convert specific directory
convert-dir:
	@if [ -z "$(dir)" ]; then \
		echo "Usage: make convert-dir dir=./your-project-directory"; \
		exit 1; \
	fi
	$(GO) run $(MAIN_FILE) -source $(dir)

# Convert with custom source and output
convert-custom:
	@if [ -z "$(source)" ] || [ -z "$(output)" ]; then \
		echo "Usage: make convert-custom source=./your-project output=./docs/output.md"; \
		exit 1; \
	fi
	$(GO) run $(MAIN_FILE) -source $(source) -output $(output)

# Show help
help:
	@echo "Available commands:"
	@echo ""
	@echo "  make convert              : Convert current directory to project.md"
	@echo "  make convert-dir dir=PATH : Convert specific directory"
	@echo "  make convert-custom source=PATH output=PATH : Convert with custom source and output paths"
	@echo ""
	@echo "Examples:"
	@echo "  make convert"
	@echo "  make convert-dir dir=./myproject"
	@echo "  make convert-custom source=./myproject output=docs/documentation.md"
	@echo ""
	@echo "For help with the converter:"
	@$(GO) run $(MAIN_FILE) -help