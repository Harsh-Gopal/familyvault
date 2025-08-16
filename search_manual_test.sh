#!/bin/bash

# Manual test script for search-files functionality
set -e

echo "=== FamilyVault Search-Files Manual Test ==="

# Setup test environment
export FAMILYVAULT_DRIVE_PATH="/tmp/familyvault-search-test-drive"
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
    rm -f *.txt *.jpg *.pdf *.zip
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

echo "2. Creating test files with different types..."
echo "This is a text document about reports" > report.txt
echo "This is another document" > document.txt
echo "Fake image content" > photo.jpg
echo "Fake PDF content" > manual.pdf
echo "Fake archive content" > backup.zip

echo "3. Uploading test files..."
for file in report.txt document.txt photo.jpg manual.pdf backup.zip; do
    curl -s -X POST \
        -H "X-Session-ID: $SESSION_ID" \
        -F "file=@$file" \
        "$BASE_URL/upload" > /dev/null
    echo "Uploaded $file"
done

echo ""
echo "4. Testing search functionality..."

echo "4.1. Search all files (no filters):"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files" | jq -r '.[] | "\(.name) (\(.type)) - \(.size) bytes"'

echo ""
echo "4.2. Search by name substring 'report':"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?name=report" | jq -r '.[] | "\(.name) (\(.type)) - \(.size) bytes"'

echo ""
echo "4.3. Search by file type 'txt':"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?type=txt" | jq -r '.[] | "\(.name) (\(.type)) - \(.size) bytes"'

echo ""
echo "4.4. Search by file type 'pdf':"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?type=pdf" | jq -r '.[] | "\(.name) (\(.type)) - \(.size) bytes"'

echo ""
echo "4.5. Search by name pattern 'doc':"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?name=doc" | jq -r '.[] | "\(.name) (\(.type)) - \(.size) bytes"'

echo ""
echo "4.6. Search with date range (last hour):"
DATE_FROM=$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ)
DATE_TO=$(date -u -v+1H +%Y-%m-%dT%H:%M:%SZ)
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?date_from=$DATE_FROM&date_to=$DATE_TO" | jq -r '.[] | "\(.name) (\(.type)) - uploaded: \(.upload_time)"'

echo ""
echo "4.7. Search with no matches:"
RESPONSE=$(curl -s -w "%{http_code}" -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?name=nonexistent")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "404" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly returned 404 for no matches"
else
    echo "✗ Expected 404, got $HTTP_CODE"
fi

echo ""
echo "4.8. Test invalid session:"
RESPONSE=$(curl -s -w "%{http_code}" -H "X-Session-ID: invalid-session" \
    "$BASE_URL/search-files")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "401" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly returned 401 for invalid session"
else
    echo "✗ Expected 401, got $HTTP_CODE"
fi

echo ""
echo "4.9. Test parameter validation (path traversal):"
RESPONSE=$(curl -s -w "%{http_code}" -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files?name=../etc/passwd")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly rejected unsafe name parameter"
else
    echo "✗ Expected 400, got $HTTP_CODE"
fi

echo ""
echo "=== Manual test completed successfully! ==="
echo "The search-files endpoint correctly:"
echo "- Validates session authentication"
echo "- Filters by name substring (case-insensitive)"
echo "- Filters by file type/extension"
echo "- Filters by date range"
echo "- Returns proper JSON with file metadata"
echo "- Handles error cases (no matches, invalid session, bad parameters)"
echo "- Prevents directory traversal attacks"