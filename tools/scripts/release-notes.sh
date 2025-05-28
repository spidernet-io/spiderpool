#!/bin/bash
# Copyright 2025 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -x

# Check for required environment variables
if [ -z "$PROJECT_NAME" ]; then
  echo "Error: PROJECT_NAME environment variable is required"
  exit 1
fi

if [ -z "$NEW_RELEASE_NOTES_INSERT_LINE" ]; then
  # Default value if not provided
  NEW_RELEASE_NOTES_INSERT_LINE=4
  echo "Using default NEW_RELEASE_NOTES_INSERT_LINE=$NEW_RELEASE_NOTES_INSERT_LINE"
fi

if [ -z "$TAG_VERSION" ]; then
  echo "Error: TAG_VERSION environment variable is required"
  exit 1
fi

# Check for English and Chinese paths and changelogs
if [ -z "$EN_PROJECT_RELEASE_NOTES_PATH" ]; then
  echo "Error: EN_PROJECT_RELEASE_NOTES_PATH environment variable is required"
  exit 1
fi

if [ -z "$ZH_PROJECT_RELEASE_NOTES_PATH" ]; then
  echo "Error: ZH_PROJECT_RELEASE_NOTES_PATH environment variable is required"
  exit 1
fi

# Create target directories if they don't exist
mkdir -p "$(dirname "$EN_PROJECT_RELEASE_NOTES_PATH")"
mkdir -p "$(dirname "$ZH_PROJECT_RELEASE_NOTES_PATH")"

# Get the current date in YYYY-MM-DD format
CURRENT_DATE=$(date +"%Y-%m-%d")

# Use the tag as version
VERSION="$TAG_VERSION"
# Remove 'v' prefix if present
VERSION=${VERSION#v}

# Function to update release notes for a specific language
update_release_notes() {
  local changelog_file=$1
  local release_notes_path=$2
  local language=$3
  local temp_release_notes="temp_${language}_release_notes.md"

  echo "Updating $language release notes at $release_notes_path"

  # Create a temporary file with the new content
  echo "## $CURRENT_DATE" >"$temp_release_notes"
  echo "" >>"$temp_release_notes"
  echo "### $VERSION" >>"$temp_release_notes"
  echo "" >>"$temp_release_notes"
  cat "$changelog_file" | grep -v "^# " | sed '/^$/N;/^\n$/D' >>"$temp_release_notes"
  echo "" >>"$temp_release_notes"

  # Check if the release-notes.md file exists
  if [ -f "$release_notes_path" ]; then
    # Check if the current version already exists in the document
    if grep -q "### $VERSION" "$release_notes_path"; then
      echo "Version $VERSION already exists in $language release notes, updating it"
      # Create a temporary file to store the updated content
      local temp_updated_file="temp_${language}_updated_file.md"
      touch "$temp_updated_file"
      # Extract the part of the existing document before the current version
      sed -n "1,/### $VERSION/p" "$release_notes_path" | sed '$d' >"$temp_updated_file"
      # Add the new version content
      echo "### $VERSION" >>"$temp_updated_file"
      echo "" >>"$temp_updated_file"
      cat "$changelog_file" | grep -v "^# " | sed '/^$/N;/^\n$/D' >>"$temp_updated_file"
      echo "" >>"$temp_updated_file"

      # Extract the part of the existing document after the current version
      sed -n "/### $VERSION/,\$p" "$release_notes_path" | sed '1,/^$/d' >>"$temp_updated_file"
      # Replace the original file
      mv "$temp_updated_file" "$release_notes_path"
    else
      echo "Version $VERSION not found in $language release notes, adding it"
      # Create a temporary file with the first N lines (title, blank line, description, blank line)
      local temp_new_file="temp_${language}_new_file.md"
      head -n $NEW_RELEASE_NOTES_INSERT_LINE "$release_notes_path" >"$temp_new_file"
      # Add the new release notes
      cat "$temp_release_notes" >>"$temp_new_file"
      echo "" >>"$temp_new_file"

      # Add the rest of the original file
      tail -n +$NEW_RELEASE_NOTES_INSERT_LINE "$release_notes_path" >>"$temp_new_file"
      # Replace the original file with the new one
      mv "$temp_new_file" "$release_notes_path"
    fi
  else
    # Create a new file with the title and the new content
    local title="$PROJECT_NAME Release Notes"
    if [ "$language" == "zh" ]; then
      title="$PROJECT_NAME 发布说明"
    fi
    echo "# $title" >"$release_notes_path"
    echo "" >>"$release_notes_path"
    cat "$temp_release_notes" >>"$release_notes_path"
  fi

  # Clean up
  rm -f "$temp_release_notes"
  echo "Updated $language release notes at $release_notes_path"
}

# Update English release notes
update_release_notes "$EN_CHANGELOG" "$EN_PROJECT_RELEASE_NOTES_PATH" "en"

# Update Chinese release notes
update_release_notes "$ZH_CHANGELOG" "$ZH_PROJECT_RELEASE_NOTES_PATH" "zh"

echo "Release notes update completed successfully"
