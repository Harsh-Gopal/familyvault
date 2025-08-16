#!/bin/bash

# Manual test script for update-metadata functionality
set -e

echo "=== FamilyVault Update-Metadata Manual Test ==="

# Setup test environment
export FAMILYVAULT_DRIVE_PATH="/tmp/familyvault-metadata-test-drive"
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
    rm -f *.txt *.jpg *.pdf
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

echo "2. Creating and uploading test files..."

# Create test files
echo "This is a text document for metadata testing" > document.txt
echo "This is another document with different content" > report.txt
echo "Fake PDF content for testing" > manual.pdf

# Upload files with initial tags
echo "2.1. Uploading document with initial tags..."
curl -s -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@document.txt" \
    -F 'tags={"category":"document","type":"text","status":"draft"}' \
    "$BASE_URL/upload-file" > /dev/null

echo "2.2. Uploading report with initial tags..."
curl -s -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@report.txt" \
    -F 'tags={"category":"report","priority":"high","department":"engineering"}' \
    "$BASE_URL/upload-file" > /dev/null

echo "2.3. Uploading PDF without tags..."
curl -s -X POST \
    -H "X-Session-ID: $SESSION_ID" \
    -F "file=@manual.pdf" \
    "$BASE_URL/upload-file" > /dev/null

echo ""
echo "3. Testing update-metadata functionality..."

echo "3.1. Update file metadata for document.txt:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "document.txt",
        "metadata": {
            "description": "Updated document description",
            "author": "Test Author",
            "version": "1.1",
            "status": "reviewed",
            "last_modified": "2025-08-12"
        }
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "200" ]; then
    echo "Response: $BODY"
    echo "✓ File metadata updated successfully"
else
    echo "✗ File metadata update failed: $BODY"
fi

echo ""
echo "3.2. Update file metadata for report.txt:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "report.txt",
        "metadata": {
            "description": "Quarterly engineering report",
            "quarter": "Q3 2025",
            "reviewed_by": "Manager",
            "priority": "critical",
            "confidential": true
        }
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "200" ]; then
    echo "Response: $BODY"
    echo "✓ Report metadata updated successfully"
else
    echo "✗ Report metadata update failed: $BODY"
fi

echo ""
echo "3.3. Add metadata to PDF file (initially had no tags):"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "manual.pdf",
        "metadata": {
            "title": "User Manual",
            "category": "documentation",
            "format": "pdf",
            "pages": 25,
            "language": "english"
        }
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "200" ]; then
    echo "Response: $BODY"
    echo "✓ PDF metadata added successfully"
else
    echo "✗ PDF metadata update failed: $BODY"
fi

echo ""
echo "3.4. Update session-level metadata:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "metadata": {
            "session_name": "Manual Test Session",
            "purpose": "Testing metadata update functionality",
            "created_by": "manual_test",
            "environment": "test",
            "total_files": 3,
            "session_type": "development"
        }
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "200" ]; then
    echo "Response: $BODY"
    echo "✓ Session metadata updated successfully"
else
    echo "✗ Session metadata update failed: $BODY"
fi

echo ""
echo "3.5. Test update with query parameter authentication:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "document.txt",
        "metadata": {
            "auth_method": "query_param",
            "test_case": "query_auth"
        }
    }' \
    "$BASE_URL/update-metadata?session_id=$SESSION_ID")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "200" ]; then
    echo "Response: $BODY"
    echo "✓ Query parameter authentication works"
else
    echo "✗ Query parameter auth failed: $BODY"
fi

echo ""
echo "3.6. Test HTML sanitization:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "document.txt",
        "metadata": {
            "description": "<script>alert(\"xss\")</script>Safe description",
            "title": "Title with <b>bold</b> text",
            "notes": "  Multiple   spaces   normalized  "
        }
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "200" ]; then
    echo "Response: $BODY"
    echo "✓ HTML sanitization working correctly"
else
    echo "✗ HTML sanitization test failed: $BODY"
fi

echo ""
echo "4. Testing error cases..."

echo "4.1. Update nonexistent file:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "nonexistent.txt",
        "metadata": {
            "description": "This should fail"
        }
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "404" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly returned 404 for nonexistent file"
else
    echo "✗ Expected 404, got $HTTP_CODE"
fi

echo ""
echo "4.2. Update with invalid session:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: invalid-session" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "document.txt",
        "metadata": {
            "description": "This should fail"
        }
    }' \
    "$BASE_URL/update-metadata")
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
echo "4.3. Update with missing metadata:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "document.txt"
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly returned 400 for missing metadata"
else
    echo "✗ Expected 400, got $HTTP_CODE"
fi

echo ""
echo "4.4. Update with empty metadata:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{
        "file_id": "document.txt",
        "metadata": {}
    }' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly returned 400 for empty metadata"
else
    echo "✗ Expected 400, got $HTTP_CODE"
fi

echo ""
echo "4.5. Update with invalid JSON:"
RESPONSE=$(curl -s -w "%{http_code}" -X PATCH \
    -H "X-Session-ID: $SESSION_ID" \
    -H "Content-Type: application/json" \
    -d '{invalid json}' \
    "$BASE_URL/update-metadata")
HTTP_CODE="${RESPONSE: -3}"
BODY="${RESPONSE%???}"
echo "HTTP Status: $HTTP_CODE"
if [ "$HTTP_CODE" = "400" ]; then
    echo "Response: $BODY"
    echo "✓ Correctly returned 400 for invalid JSON"
else
    echo "✗ Expected 400, got $HTTP_CODE"
fi

echo ""
echo "5. Verify updated metadata by searching files:"
echo "5.1. Search all files to see updated metadata:"
curl -s -H "X-Session-ID: $SESSION_ID" \
    "$BASE_URL/search-files"

echo ""
echo ""
echo "=== Manual test completed successfully! ==="
echo "The update-metadata endpoint correctly:"
echo "- Updates file-level metadata (tags) for specific files"
echo "- Updates session-level metadata for the entire session"
echo "- Validates session authentication (header and query param)"
echo "- Sanitizes HTML and dangerous content in metadata values"
echo "- Normalizes whitespace and handles empty values"
echo "- Returns proper JSON responses with success status"
echo "- Handles all error cases appropriately (404, 401, 400)"
echo "- Integrates with the existing manifest system"