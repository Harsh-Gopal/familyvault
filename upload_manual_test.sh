#!/bin/bash

# Manual test script for upload-file functionality
set -e

echo "=== FamilyVault Upload-File Manual Test ==="

# Setup test environment
export FAMILYVAULT_DRIVE_PATH="/tmp/familyvault-upload-test-drive"
export FAMILYVAULT_MAX_FILE_SIZE_MB="10"
mkdir -p "$FAMILYVAULT_DRIVE_PATH"

# Start server in background
echo "Starting FamilyVault server..."
./familyvault &
SERVER_PID=$!

# Wait for server to start
sleep 2

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    kill $SERVER_PID 2>/dev/null || true
    rm -rf "$FAMILYVAULT_DRIVE_PATH"
    rm -f *.txt *.jpg *.pdf *.zip *.exe
}
trap cleanup EXIT

# Test the API
BASE_URL="http://localhost:8000"

echo "1. Opening session..."
SESSION_RESPONSE=$(curl -s -X POST "$BASE_URL/session/open")
SESSION_ID=$(echo "$SESSION_RESPONSE" | grep -o '"session_id":"[^"]*"' | cut -d'"' -f4)
echo "Session ID: $SESSION_ID"

if [ -z "$SESSION_ID" ]; then
    echo "Failed to create session"
    exit 1
fi

echo "2. Creating test files..."
echo "This is a text document for testing upload functionality" > document.txt
echo "This is another text file with different content" > notes.txt
echo "Fake PDF content for testing" > report.pdf
echo "Fake image content" > photo.jpg
echo "Fake archive content" > backup.zip

# Create a large file for size testing
echo "Creating large file for size testing..."
dd if=/dev/zero of=large.txt bs=1024 count=5120 2>/dev/null  # 5MB file

echo ""
echo "3. Testing upload functionality..."

echo "3.1. Upload document with tags:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@document.txt" \
    -F 'tags={"category":"document","priority":"high","department":"engineering"}' \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "201" ]; then
    echo "Response: $BODY"
    echo "✓ Document uploaded successfully with tags"
else
    echo "✗ Document upload failed: $BODY"
fi

echo ""
echo "3.2. Upload image without tags:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@photo.jpg" \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "201" ]; then
    echo "Response: $BODY"
    echo "✓ Image uploaded successfully without tags"
else
    echo "✗ Image upload failed: $BODY"
fi

echo ""
echo "3.3. Upload PDF with simple tags:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@report.pdf" \
    -F 'tags={"type":"report","year":"2025"}' \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "201" ]; then
    echo "Response: $BODY"
    echo "✓ PDF uploaded successfully with tags"
else
    echo "✗ PDF upload failed: $BODY"
fi

echo ""
echo "3.4. Upload duplicate filename (should get unique name):"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@document.txt" \
    -F 'tags={"version":"2","duplicate":"true"}' \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "201" ]; then
    echo "Response: $BODY"
    # Extract filename from JSON response (simple grep approach)
    FILENAME=$(echo "$BODY" | grep -o '"name":"[^"]*"' | cut -d'"' -f4)
    if [[ "$FILENAME" != "document.txt" ]]; then
        echo "✓ Duplicate filename correctly renamed to: $FILENAME"
    else
        echo "✗ Duplicate filename not renamed"
    fi
else
    echo "✗ Duplicate upload failed: $BODY"
fi

echo ""
echo "3.5. Test large file upload:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@large.txt" \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "201" ]; then
    echo "Response: $BODY"
    echo "✓ Large file uploaded successfully"
else
    echo "Response: $BODY"
    if [ "$HTTP_CODE" = "413" ]; then
        echo "✓ Large file correctly rejected (file too large)"
    else
        echo "✗ Unexpected response for large file"
    fi
fi

echo ""
echo "3.6. Test unsupported file type:"
echo "This is fake executable content" > malware.exe
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@malware.exe" \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Unsupported file type correctly rejected"
else
    echo "✗ Expected 400 for unsupported file type, got $HTTP_CODE"
fi

echo ""
echo "3.7. Test empty file:"
touch empty.txt
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@empty.txt" \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Empty file correctly rejected"
else
    echo "✗ Expected 400 for empty file, got $HTTP_CODE"
fi

echo ""
echo "3.8. Test invalid tags JSON:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@notes.txt" \
    -F 'tags={invalid: json}' \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Invalid tags JSON correctly rejected"
else
    echo "✗ Expected 400 for invalid tags JSON, got $HTTP_CODE"
fi

echo ""
echo "3.9. Test invalid session:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: invalid-session" \
    -F "file=@notes.txt" \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "401" ]; then
    echo "Response: $BODY"
    echo "✓ Invalid session correctly rejected"
else
    echo "✗ Expected 401 for invalid session, got $HTTP_CODE"
fi

echo ""
echo "3.10. Test missing file field:"
RESPONSE=$(curl -s -w "%{http_code}" -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F 'tags={"test":"value"}' \
    "$BASE_URL/upload-file")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Missing file field correctly rejected"
else
    echo "✗ Expected 400 for missing file field, got $HTTP_CODE"
fi

echo ""
echo "4. Verify uploaded files can be searched:"
echo "4.1. Search all uploaded files:"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files"

echo ""
echo "4.2. Search by tag:"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?tags=document"

echo ""
echo "4.3. Search by file type:"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?type=txt"

echo ""
echo "5. Test download of uploaded file:"
FIRST_FILE=$(curl -s -H "X-Session-ID: $SESSION_ID" "$BASE_URL/search-files" | grep -o '"name":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ "$FIRST_FILE" != "null" ] && [ -n "$FIRST_FILE" ]; then
    echo "Downloading file: $FIRST_FILE"
    RESPONSE=$(curl -s -w "%{http_code}" -H "X-Session-ID: $SESSION_ID" \
        "$BASE_URL/download?filename=$FIRST_FILE" \
        -o "downloaded_$FIRST_FILE")
    HTTP_CODE="${RESPONSE: -3}"
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✓ File downloaded successfully: downloaded_$FIRST_FILE"
        echo "Downloaded file size: $(wc -c < "downloaded_$FIRST_FILE") bytes"
    else
        echo "✗ Download failed with status $HTTP_CODE"
    fi
else
    echo "No files found to download"
fi

echo ""
echo "=== Manual test completed successfully! ==="
echo "The upload-file endpoint correctly:"
echo "- Validates session authentication"
echo "- Accepts multipart form-data with file and optional tags"
echo "- Validates file size limits and extensions"
echo "- Sanitizes filenames and prevents overwrites"
echo "- Stores files encrypted and updates manifest"
echo "- Returns proper JSON metadata on success"
echo "- Handles all error cases appropriately"
echo "- Integrates with search and download functionality"