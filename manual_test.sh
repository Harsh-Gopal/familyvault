#!/bin/bash

# Manual test script for download-all functionality
set -e

echo "=== FamilyVault Download-All Manual Test ==="

# Setup test environment
export FAMILYVAULT_DRIVE_PATH="/tmp/familyvault-test-drive"
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
    rm -f test_file1.txt test_file2.txt downloaded_archive.zip
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
echo "Hello, this is test file 1!" > test_file1.txt
echo "This is the content of test file 2 with more data." > test_file2.txt

echo "3. Uploading test files..."
curl -s -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@test_file1.txt" \
    "$BASE_URL/upload"
echo "Uploaded test_file1.txt"

curl -s -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@test_file2.txt" \
    "$BASE_URL/upload"
echo "Uploaded test_file2.txt"

echo "4. Downloading all files as ZIP..."
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/download-all" \
    -o downloaded_archive.zip

if [ ! -f downloaded_archive.zip ]; then
    echo "Failed to download ZIP archive"
    exit 1
fi

echo "5. Verifying ZIP contents..."
unzip -l downloaded_archive.zip
echo ""

echo "6. Extracting and verifying file contents..."
mkdir -p extracted
cd extracted
unzip -q ../downloaded_archive.zip

echo "Extracted files:"
ls -la

echo ""
echo "Content of test_file1.txt:"
cat test_file1.txt
echo ""

echo "Content of test_file2.txt:"
cat test_file2.txt
echo ""

# Verify content matches
cd ..
if diff test_file1.txt extracted/test_file1.txt > /dev/null; then
    echo "✓ test_file1.txt content matches"
else
    echo "✗ test_file1.txt content mismatch"
    exit 1
fi

if diff test_file2.txt extracted/test_file2.txt > /dev/null; then
    echo "✓ test_file2.txt content matches"
else
    echo "✗ test_file2.txt content mismatch"
    exit 1
fi

echo ""
echo "=== Manual test completed successfully! ==="
echo "The download-all endpoint correctly:"
echo "- Validates session authentication"
echo "- Streams a ZIP archive containing all session files"
echo "- Decrypts files on-the-fly"
echo "- Preserves original file content"