# Basic configuration
GO=go
CONVERT_FILE=cmd/convert/main.go
REVERSE_FILE=cmd/reverse/main.go
DEFAULT_SEPARATOR=/==
DEFAULT_SKIP=node_modules,vendor,build

.PHONY: all help convert convert-dir convert-custom reconstruct reconstruct-custom

# Default target
all: help

###################
# Convert Commands #
###################

# Convert current directory
convert:
	$(GO) run $(CONVERT_FILE) -separator $(DEFAULT_SEPARATOR) -skip $(if $(skip),$(skip),$(DEFAULT_SKIP))

# Convert specific directory
convert-dir:
	@if [ -z "$(dir)" ]; then \
		echo "Usage: make convert-dir dir=./your-project-directory [separator=/==] [skip=node_modules,vendor]"; \
		exit 1; \
	fi
	$(GO) run $(CONVERT_FILE) \
		-source $(dir) \
		-separator $(if $(separator),$(separator),$(DEFAULT_SEPARATOR)) \
		-skip $(if $(skip),$(skip),$(DEFAULT_SKIP))

# Convert with custom source and output
convert-custom:
	@if [ -z "$(source)" ] || [ -z "$(output)" ]; then \
		echo "Usage: make convert-custom source=./your-project output=./docs/output.md [separator=/==] [skip=node_modules,vendor]"; \
		exit 1; \
	fi
	$(GO) run $(CONVERT_FILE) \
		-source $(source) \
		-output $(output) \
		-separator $(if $(separator),$(separator),$(DEFAULT_SEPARATOR)) \
		-skip $(if $(skip),$(skip),$(DEFAULT_SKIP))

######################
# Reconstruct Commands #
######################

# Reconstruct with default settings
reconstruct:
	$(GO) run $(REVERSE_FILE) -separator $(DEFAULT_SEPARATOR)

# Reconstruct with custom settings
reconstruct-custom:
	@if [ -z "$(input)" ]; then \
		echo "Usage: make reconstruct-custom input=./project.md [output=./reconstructed] [separator=/==]"; \
		exit 1; \
	fi
	$(GO) run $(REVERSE_FILE) \
		-input $(input) \
		$(if $(output),-output $(output),) \
		$(if $(separator),-separator $(separator),)

# Show help
help:
	@echo "Project Documentation Tool"
	@echo ""
	@echo "Convert Commands:"
	@echo "----------------"
	@echo "  make convert              : Convert current directory to project.md"
	@echo "  make convert-dir dir=PATH : Convert specific directory"
	@echo "  make convert-custom source=PATH output=PATH : Convert with custom paths"
	@echo ""
	@echo "Convert Parameters:"
	@echo "  separator=SYMBOL         : Custom separator (default: $(DEFAULT_SEPARATOR))"
	@echo "  skip=FOLDERS            : Comma-separated list of folders to skip"
	@echo "                            (default: $(DEFAULT_SKIP))"
	@echo ""
	@echo "Convert Examples:"
	@echo "  make convert"
	@echo "  make convert skip=test,docs,vendor"
	@echo "  make convert-dir dir=./myproject"
	@echo "  make convert-dir dir=./myproject separator='/**' skip=node_modules,dist"
	@echo "  make convert-custom source=./myproject output=docs/documentation.md"
	@echo ""
	@echo "Reconstruct Commands:"
	@echo "-------------------"
	@echo "  make reconstruct         : Reconstruct from project.md (default settings)"
	@echo "  make reconstruct-custom  : Reconstruct with custom settings"
	@echo ""
	@echo "Reconstruct Parameters:"
	@echo "  input=PATH              : Input markdown file path"
	@echo "  output=PATH             : Output directory (default: ./reconstructed)"
	@echo "  separator=SYMBOL        : Path separator (default: $(DEFAULT_SEPARATOR))"
	@echo ""
	@echo "Reconstruct Examples:"
	@echo "  make reconstruct"
	@echo "  make reconstruct-custom input=./docs/project.md"
	@echo "  make reconstruct-custom input=./docs/project.md output=./src separator='/**'"
	@echo ""
	@echo "For detailed help:"
	@echo "  Convert help:     $(GO) run $(CONVERT_FILE) -help"
	@echo "  Reconstruct help: $(GO) run $(REVERSE_FILE) -help"