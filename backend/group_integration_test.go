package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"familyvault/internal/auth/localjwt"
	"familyvault/internal/config"
	"familyvault/internal/core/drive"
	"familyvault/internal/core/groups"
	"familyvault/internal/http/handlers"
	"familyvault/internal/notify"
)

func TestGroupIntegrationFlow(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	driveDir := t.TempDir()

	// Set environment variables
	os.Setenv("FAMILYVAULT_DATA_PATH", tempDir)
	os.Setenv("FAMILYVAULT_DRIVE_PATH", driveDir)
	defer func() {
		os.Unsetenv("FAMILYVAULT_DATA_PATH")
		os.Unsetenv("FAMILYVAULT_DRIVE_PATH")
	}()

	// Load configuration
	cfg := config.Load()
	drive.SetDrivePath(cfg.DrivePath)

	// Initialize components
	store, err := groups.NewStore(cfg.DataPath)
	if err != nil {
		t.Fatalf("Failed to initialize groups store: %v", err)
	}

	jwtManager, err := localjwt.NewJWTManager(cfg.DataPath)
	if err != nil {
		t.Fatalf("Failed to initialize JWT manager: %v", err)
	}

	notifier := notify.NewNotificationService(cfg)

	// Create router
	router := handlers.NewGroupRouter(store, jwtManager, notifier)
	server := httptest.NewServer(router)
	defer server.Close()

	// Test 1: Create a group (admin user)
	t.Run("CreateGroup", func(t *testing.T) {
		createGroupReq := map[string]interface{}{
			"name":               "Test Family",
			"owner_display_name": "John Doe",
			"email":              "john@example.com",
		}

		reqBody, _ := json.Marshal(createGroupReq)
		req, _ := http.NewRequest("POST", server.URL+"/groups", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Device-Name", "MacBook-Pro")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create group: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var createResp handlers.CreateGroupResponse
		if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if createResp.GroupID == "" {
			t.Error("Group ID should not be empty")
		}

		if createResp.Role != "admin" {
			t.Errorf("Expected role 'admin', got %s", createResp.Role)
		}

		if createResp.Token == "" {
			t.Error("Token should not be empty")
		}

		// Store for later tests
		adminToken := createResp.Token
		groupID := createResp.GroupID

		// Test 2: Get user info
		t.Run("WhoAmI", func(t *testing.T) {
			req, _ := http.NewRequest("GET", server.URL+"/me", nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to get user info: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var whoAmIResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&whoAmIResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			claims, ok := whoAmIResp["claims"].(map[string]interface{})
			if !ok {
				t.Error("Claims should be present")
			}

			if claims["role"] != "admin" {
				t.Errorf("Expected role 'admin', got %v", claims["role"])
			}
		})

		// Test 3: Open a session
		t.Run("OpenSession", func(t *testing.T) {
			req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/sessions/open", nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to open session: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var sessionResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if sessionResp["session_id"] == "" {
				t.Error("Session ID should not be empty")
			}

			if sessionResp["group_id"] != groupID {
				t.Errorf("Expected group ID %s, got %v", groupID, sessionResp["group_id"])
			}
		})

		// Test 4: Invite a member
		var pairingToken string
		t.Run("InviteMember", func(t *testing.T) {
			inviteReq := map[string]interface{}{
				"contact":     "jane@example.com",
				"ttl_minutes": 60,
			}

			reqBody, _ := json.Marshal(inviteReq)
			req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/members/invite", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to invite member: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var inviteResp handlers.InviteMemberResponse
			if err := json.NewDecoder(resp.Body).Decode(&inviteResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if inviteResp.PairingToken == "" {
				t.Error("Pairing token should not be empty")
			}

			if inviteResp.QR == "" {
				t.Error("QR code should not be empty")
			}

			pairingToken = inviteResp.PairingToken
		})

		// Test 5: Pair a device
		var deviceID string
		t.Run("PairDevice", func(t *testing.T) {
			pairReq := map[string]interface{}{
				"token":       pairingToken,
				"device_name": "iPhone-13",
			}

			reqBody, _ := json.Marshal(pairReq)
			req, _ := http.NewRequest("POST", server.URL+"/pair", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to pair device: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var pairResp handlers.PairResponse
			if err := json.NewDecoder(resp.Body).Decode(&pairResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if !pairResp.Pending {
				t.Error("Device should be pending approval")
			}

			if pairResp.GroupID != groupID {
				t.Errorf("Expected group ID %s, got %s", groupID, pairResp.GroupID)
			}

			if pairResp.DeviceID == "" {
				t.Error("Device ID should not be empty")
			}

			deviceID = pairResp.DeviceID
		})

		// Test 6: Approve device
		var memberToken string
		t.Run("ApproveDevice", func(t *testing.T) {
			req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/devices/"+deviceID+"/approve", nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to approve device: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var approveResp handlers.ApproveDeviceResponse
			if err := json.NewDecoder(resp.Body).Decode(&approveResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if approveResp.Token == "" {
				t.Error("Token should not be empty")
			}

			if approveResp.Role != "member" {
				t.Errorf("Expected role 'member', got %s", approveResp.Role)
			}

			memberToken = approveResp.Token
		})

		// Test 7: Member can access their info
		t.Run("MemberWhoAmI", func(t *testing.T) {
			req, _ := http.NewRequest("GET", server.URL+"/me", nil)
			req.Header.Set("Authorization", "Bearer "+memberToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to get member info: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var whoAmIResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&whoAmIResp); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			claims, ok := whoAmIResp["claims"].(map[string]interface{})
			if !ok {
				t.Error("Claims should be present")
			}

			if claims["role"] != "member" {
				t.Errorf("Expected role 'member', got %v", claims["role"])
			}
		})

		// Test 8: List group members
		t.Run("ListMembers", func(t *testing.T) {
			req, _ := http.NewRequest("GET", server.URL+"/groups/"+groupID+"/members", nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to list members: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}

			var members []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&members); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(members) != 2 {
				t.Errorf("Expected 2 members, got %d", len(members))
			}
		})

		// Test 9: Member cannot open session (admin only)
		t.Run("MemberCannotOpenSession", func(t *testing.T) {
			req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/sessions/open", nil)
			req.Header.Set("Authorization", "Bearer "+memberToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusForbidden {
				t.Errorf("Expected status 403, got %d", resp.StatusCode)
			}
		})

		// Test 10: Close session
		t.Run("CloseSession", func(t *testing.T) {
			req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/sessions/close", nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to close session: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("Expected status 200, got %d", resp.StatusCode)
			}
		})
	})
}

func TestRBACPermissions(t *testing.T) {
	// Setup test environment
	tempDir := t.TempDir()
	driveDir := t.TempDir()

	// Set environment variables
	os.Setenv("FAMILYVAULT_DATA_PATH", tempDir)
	os.Setenv("FAMILYVAULT_DRIVE_PATH", driveDir)
	defer func() {
		os.Unsetenv("FAMILYVAULT_DATA_PATH")
		os.Unsetenv("FAMILYVAULT_DRIVE_PATH")
	}()

	// Load configuration
	cfg := config.Load()
	drive.SetDrivePath(cfg.DrivePath)

	// Initialize components
	store, err := groups.NewStore(cfg.DataPath)
	if err != nil {
		t.Fatalf("Failed to initialize groups store: %v", err)
	}

	jwtManager, err := localjwt.NewJWTManager(cfg.DataPath)
	if err != nil {
		t.Fatalf("Failed to initialize JWT manager: %v", err)
	}

	notifier := notify.NewNotificationService(cfg)

	// Create router
	router := handlers.NewGroupRouter(store, jwtManager, notifier)
	server := httptest.NewServer(router)
	defer server.Close()

	// Create a group and get tokens for different roles
	adminToken, memberToken, viewerToken, groupID := setupTestGroup(t, server)

	// Test admin permissions
	t.Run("AdminPermissions", func(t *testing.T) {
		// Admin can open session
		req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/sessions/open", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Admin should be able to open session, got status %d", resp.StatusCode)
		}

		// Admin can invite members
		inviteReq := map[string]interface{}{
			"contact":     "test@example.com",
			"ttl_minutes": 60,
		}
		reqBody, _ := json.Marshal(inviteReq)
		req, _ = http.NewRequest("POST", server.URL+"/groups/"+groupID+"/members/invite", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Admin should be able to invite members, got status %d", resp.StatusCode)
		}
	})

	// Test member permissions
	t.Run("MemberPermissions", func(t *testing.T) {
		// Member cannot open session
		req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/sessions/open", nil)
		req.Header.Set("Authorization", "Bearer "+memberToken)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Member should not be able to open session, got status %d", resp.StatusCode)
		}

		// Member cannot invite members
		inviteReq := map[string]interface{}{
			"contact":     "test2@example.com",
			"ttl_minutes": 60,
		}
		reqBody, _ := json.Marshal(inviteReq)
		req, _ = http.NewRequest("POST", server.URL+"/groups/"+groupID+"/members/invite", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+memberToken)
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Member should not be able to invite members, got status %d", resp.StatusCode)
		}
	})

	// Test viewer permissions
	t.Run("ViewerPermissions", func(t *testing.T) {
		// Viewer cannot open session
		req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/sessions/open", nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Viewer should not be able to open session, got status %d", resp.StatusCode)
		}

		// Viewer cannot invite members
		inviteReq := map[string]interface{}{
			"contact":     "test3@example.com",
			"ttl_minutes": 60,
		}
		reqBody, _ := json.Marshal(inviteReq)
		req, _ = http.NewRequest("POST", server.URL+"/groups/"+groupID+"/members/invite", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Viewer should not be able to invite members, got status %d", resp.StatusCode)
		}
	})
}

// Helper function to setup a test group with different role users
func setupTestGroup(t *testing.T, server *httptest.Server) (adminToken, memberToken, viewerToken, groupID string) {
	// Create group (admin)
	createGroupReq := map[string]interface{}{
		"name":               "Test Group",
		"owner_display_name": "Admin User",
		"email":              "admin@example.com",
	}

	reqBody, _ := json.Marshal(createGroupReq)
	req, _ := http.NewRequest("POST", server.URL+"/groups", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Device-Name", "Admin-Device")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}
	defer resp.Body.Close()

	var createResp handlers.CreateGroupResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	adminToken = createResp.Token
	groupID = createResp.GroupID

	// Create member through invitation flow
	memberToken = createMemberToken(t, server, adminToken, groupID, "member@example.com")

	// Create viewer through invitation flow
	viewerToken = createMemberToken(t, server, adminToken, groupID, "viewer@example.com")

	return adminToken, memberToken, viewerToken, groupID
}

// Helper to create a member token through the full flow
func createMemberToken(t *testing.T, server *httptest.Server, adminToken, groupID, email string) string {
	// Invite member
	inviteReq := map[string]interface{}{
		"contact":     email,
		"ttl_minutes": 60,
	}

	reqBody, _ := json.Marshal(inviteReq)
	req, _ := http.NewRequest("POST", server.URL+"/groups/"+groupID+"/members/invite", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	var inviteResp handlers.InviteMemberResponse
	json.NewDecoder(resp.Body).Decode(&inviteResp)

	// Pair device
	pairReq := map[string]interface{}{
		"token":       inviteResp.PairingToken,
		"device_name": "Test-Device-" + email,
	}

	reqBody, _ = json.Marshal(pairReq)
	req, _ = http.NewRequest("POST", server.URL+"/pair", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	var pairResp handlers.PairResponse
	json.NewDecoder(resp.Body).Decode(&pairResp)

	// Approve device
	req, _ = http.NewRequest("POST", server.URL+"/groups/"+groupID+"/devices/"+pairResp.DeviceID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, _ = http.DefaultClient.Do(req)
	defer resp.Body.Close()

	var approveResp handlers.ApproveDeviceResponse
	json.NewDecoder(resp.Body).Decode(&approveResp)

	return approveResp.Token
}

func TestJWTTokenValidation(t *testing.T) {
	tempDir := t.TempDir()

	// Initialize JWT manager
	jwtManager, err := localjwt.NewJWTManager(tempDir)
	if err != nil {
		t.Fatalf("Failed to initialize JWT manager: %v", err)
	}

	// Test valid token
	token, err := jwtManager.IssueToken("group1", "user1", "device1", "admin", 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to issue token: %v", err)
	}

	claims, err := jwtManager.ParseToken(token)
	if err != nil {
		t.Fatalf("Failed to parse valid token: %v", err)
	}

	if claims.GroupID != "group1" {
		t.Errorf("Expected group ID 'group1', got %s", claims.GroupID)
	}

	// Test invalid token
	_, err = jwtManager.ParseToken("invalid-token")
	if err == nil {
		t.Error("Should fail to parse invalid token")
	}

	// Test expired token
	expiredToken, err := jwtManager.IssueToken("group1", "user1", "device1", "admin", 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to issue expired token: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = jwtManager.ParseToken(expiredToken)
	if err == nil {
		t.Error("Should fail to parse expired token")
	}
}

func TestNotificationService(t *testing.T) {
	// Test with no SMTP configuration (should not fail)
	cfg := &config.Config{
		SMTP: config.SMTPConfig{}, // Empty SMTP config
		SMS:  config.SMSConfig{Provider: "none"},
	}

	notifier := notify.NewNotificationService(cfg)

	// Email should fail gracefully
	err := notifier.SendEmail("test@example.com", "Test Subject", "Test Body")
	if err == nil {
		t.Error("Should fail when SMTP not configured")
	}

	// SMS should succeed (no-op)
	err = notifier.SendSMS("+1234567890", "Test SMS")
	if err != nil {
		t.Errorf("SMS should succeed with no-op provider: %v", err)
	}

	// Multi-channel should handle failures gracefully
	sent, failed := notifier.SendMultiChannel("test@example.com", "Subject", "Body", []string{"email", "sms"})
	if sent != 1 || failed != 1 {
		t.Errorf("Expected 1 sent, 1 failed, got %d sent, %d failed", sent, failed)
	}
}
