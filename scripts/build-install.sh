#!/bin/bash

# Build script to generate install.sh from templates
set -e

TEMPLATE_DIR="templates"
OUTPUT_DIR="scripts"
TEMPLATE_FILE="$TEMPLATE_DIR/install.sh"
OUTPUT_FILE="$OUTPUT_DIR/install.sh"

echo "🔧 Building install.sh from templates..."

# Check if template exists
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "❌ Template file not found: $TEMPLATE_FILE"
    exit 1
fi

# Read template files
SYSTEMD_SERVICE=$(cat "$TEMPLATE_DIR/provisioner.service")
EXAMPLE_CONFIG_JSON=$(cat "examples/environments/simple-example/config.json")
EXAMPLE_MAIN_TF=$(cat "examples/environments/simple-example/main.tf")

# Create embedded examples archives
echo "📦 Creating embedded examples archive..."
EXAMPLES_TAR=$(mktemp)
tar -czf "$EXAMPLES_TAR" -C examples environments
EXAMPLES_BASE64=$(base64 -w 0 < "$EXAMPLES_TAR")
rm "$EXAMPLES_TAR"

echo "📦 Creating embedded templates archive..."
TEMPLATES_TAR=$(mktemp)
tar -czf "$TEMPLATES_TAR" -C examples templates
TEMPLATES_BASE64=$(base64 -w 0 < "$TEMPLATES_TAR")
rm "$TEMPLATES_TAR"

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Generate install script by replacing placeholders
# Use a temporary file to handle multi-line replacements properly
TEMP_FILE=$(mktemp)
cp "$TEMPLATE_FILE" "$TEMP_FILE"

# Replace placeholders using awk for better multi-line handling
awk -v systemd_service="$SYSTEMD_SERVICE" \
    -v example_config="$EXAMPLE_CONFIG_JSON" \
    -v example_tf="$EXAMPLE_MAIN_TF" \
    -v examples_base64="$EXAMPLES_BASE64" \
    -v templates_base64="$TEMPLATES_BASE64" '
{
    gsub(/{{SYSTEMD_SERVICE}}/, systemd_service)
    gsub(/{{EXAMPLE_CONFIG_JSON}}/, example_config)
    gsub(/{{EXAMPLE_MAIN_TF}}/, example_tf)
    gsub(/{{EXAMPLES_BASE64}}/, examples_base64)
    gsub(/{{TEMPLATES_BASE64}}/, templates_base64)
    print
}' "$TEMP_FILE" > "$OUTPUT_FILE"

rm "$TEMP_FILE"

# Make executable
chmod +x "$OUTPUT_FILE"

echo "✅ Generated: $OUTPUT_FILE"
echo "📋 Template sources:"
echo "  - Service: $TEMPLATE_DIR/provisioner.service"
echo "  - Config: examples/environments/simple-example/config.json"
echo "  - Terraform: examples/environments/simple-example/main.tf"