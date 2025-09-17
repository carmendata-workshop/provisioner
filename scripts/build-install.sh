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
EXAMPLE_CONFIG_JSON=$(cat "$TEMPLATE_DIR/environments/simple-example/config.json")
EXAMPLE_MAIN_TF=$(cat "$TEMPLATE_DIR/environments/simple-example/main.tf")

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Generate install script by replacing placeholders
# Use a temporary file to handle multi-line replacements properly
TEMP_FILE=$(mktemp)
cp "$TEMPLATE_FILE" "$TEMP_FILE"

# Replace placeholders using awk for better multi-line handling
awk -v systemd_service="$SYSTEMD_SERVICE" \
    -v example_config="$EXAMPLE_CONFIG_JSON" \
    -v example_tf="$EXAMPLE_MAIN_TF" '
{
    gsub(/{{SYSTEMD_SERVICE}}/, systemd_service)
    gsub(/{{EXAMPLE_CONFIG_JSON}}/, example_config)
    gsub(/{{EXAMPLE_MAIN_TF}}/, example_tf)
    print
}' "$TEMP_FILE" > "$OUTPUT_FILE"

rm "$TEMP_FILE"

# Make executable
chmod +x "$OUTPUT_FILE"

echo "✅ Generated: $OUTPUT_FILE"
echo "📋 Template sources:"
echo "  - Service: $TEMPLATE_DIR/provisioner.service"
echo "  - Config: $TEMPLATE_DIR/environments/simple-example/config.json"
echo "  - Terraform: $TEMPLATE_DIR/environments/simple-example/main.tf"