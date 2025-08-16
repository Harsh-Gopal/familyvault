package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration
type Config struct {
	DataPath          string
	DrivePath         string
	SMTP              SMTPConfig
	SMS               SMSConfig
	PairingTTLMinutes int
	JWTTTLMinutes     int
	DefaultGroupName  string
	Upload            UploadConfig
}

// SMTPConfig holds SMTP configuration for email notifications
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	TLS      bool
}

// SMSConfig holds SMS configuration
type SMSConfig struct {
	Provider string // "twilio" or "none"
	// Twilio specific
	AccountSID string
	AuthToken  string
	FromNumber string
}

// UploadConfig holds configuration for file uploads
type UploadConfig struct {
	MaxFileSize       int64    // Maximum file size in bytes
	AllowedExtensions []string // Allowed file extensions
	RequireExtension  bool     // Whether to require file extension validation
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	config := &Config{
		DataPath:          getEnvOrDefault("FAMILYVAULT_DATA_PATH", filepath.Join(os.Getenv("HOME"), ".familyvault")),
		DrivePath:         getEnvOrDefault("FAMILYVAULT_DRIVE_PATH", ""),
		PairingTTLMinutes: getEnvIntOrDefault("FAMILYVAULT_PAIRING_TTL_MINUTES", 60),
		JWTTTLMinutes:     getEnvIntOrDefault("FAMILYVAULT_JWT_TTL_MINUTES", 1440),
		DefaultGroupName:  getEnvOrDefault("FAMILYVAULT_DEFAULT_GROUP_NAME", "My Family"),
		SMTP:              loadSMTPConfig(),
		SMS:               loadSMSConfig(),
		Upload:            loadUploadConfig(),
	}

	// Ensure data directory exists
	os.MkdirAll(config.DataPath, 0755)

	return config
}

// GetPairingTTL returns the pairing token TTL as duration
func (c *Config) GetPairingTTL() time.Duration {
	return time.Duration(c.PairingTTLMinutes) * time.Minute
}

// GetJWTTTL returns the JWT TTL as duration
func (c *Config) GetJWTTTL() time.Duration {
	return time.Duration(c.JWTTTLMinutes) * time.Minute
}

// IsSMTPConfigured returns true if SMTP is configured
func (c *Config) IsSMTPConfigured() bool {
	return c.SMTP.Host != "" && c.SMTP.Port > 0
}

// IsSMSConfigured returns true if SMS is configured
func (c *Config) IsSMSConfigured() bool {
	return c.SMS.Provider == "twilio" && c.SMS.AccountSID != "" && c.SMS.AuthToken != ""
}

func loadSMTPConfig() SMTPConfig {
	return SMTPConfig{
		Host:     os.Getenv("FAMILYVAULT_SMTP_HOST"),
		Port:     getEnvIntOrDefault("FAMILYVAULT_SMTP_PORT", 587),
		Username: os.Getenv("FAMILYVAULT_SMTP_USER"),
		Password: os.Getenv("FAMILYVAULT_SMTP_PASS"),
		From:     os.Getenv("FAMILYVAULT_SMTP_FROM"),
		TLS:      getEnvBoolOrDefault("FAMILYVAULT_SMTP_TLS", true),
	}
}

func loadSMSConfig() SMSConfig {
	return SMSConfig{
		Provider:   getEnvOrDefault("FAMILYVAULT_SMS_PROVIDER", "none"),
		AccountSID: os.Getenv("FAMILYVAULT_SMS_ACCOUNT_SID"),
		AuthToken:  os.Getenv("FAMILYVAULT_SMS_AUTH_TOKEN"),
		FromNumber: os.Getenv("FAMILYVAULT_SMS_FROM_NUMBER"),
	}
}

// GetUploadConfig returns the upload configuration with defaults and environment overrides
func loadUploadConfig() UploadConfig {
	config := UploadConfig{
		MaxFileSize:      50 * 1024 * 1024, // 50MB default
		RequireExtension: true,
		AllowedExtensions: []string{
			// Images
			"jpg", "jpeg", "png", "gif", "bmp", "webp", "svg",
			// Documents
			"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "rtf", "odt",
			// Archives
			"zip", "rar", "7z", "tar", "gz", "bz2",
			// Audio/Video
			"mp3", "wav", "flac", "aac", "mp4", "avi", "mkv", "mov", "wmv", "webm",
			// Code/Data
			"json", "xml", "csv", "sql", "log",
		},
	}

	// Override max file size from environment
	if maxSizeStr := os.Getenv("FAMILYVAULT_MAX_FILE_SIZE_MB"); maxSizeStr != "" {
		if maxSizeMB, err := strconv.Atoi(maxSizeStr); err == nil && maxSizeMB > 0 {
			config.MaxFileSize = int64(maxSizeMB) * 1024 * 1024
		}
	}

	// Override allowed extensions from environment
	if extensionsStr := os.Getenv("FAMILYVAULT_ALLOWED_EXTENSIONS"); extensionsStr != "" {
		extensions := strings.Split(extensionsStr, ",")
		var cleanExtensions []string
		for _, ext := range extensions {
			ext = strings.TrimSpace(strings.ToLower(ext))
			if ext != "" {
				// Remove leading dot if present
				ext = strings.TrimPrefix(ext, ".")
				cleanExtensions = append(cleanExtensions, ext)
			}
		}
		if len(cleanExtensions) > 0 {
			config.AllowedExtensions = cleanExtensions
		}
	}

	// Override extension requirement from environment
	if requireExtStr := os.Getenv("FAMILYVAULT_REQUIRE_EXTENSION"); requireExtStr != "" {
		config.RequireExtension = strings.ToLower(requireExtStr) == "true"
	}

	return config
}

// IsExtensionAllowed checks if a file extension is allowed
func (c *UploadConfig) IsExtensionAllowed(extension string) bool {
	if !c.RequireExtension {
		return true
	}

	extension = strings.ToLower(strings.TrimPrefix(extension, "."))
	for _, allowed := range c.AllowedExtensions {
		if extension == allowed {
			return true
		}
	}
	return false
}

// Helper functions
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if valueStr := os.Getenv(key); valueStr != "" {
		return strings.ToLower(valueStr) == "true"
	}
	return defaultValue
}

// GetUploadConfig returns the upload configuration (legacy compatibility)
// Deprecated: Use Load().Upload instead
func GetUploadConfig() *UploadConfig {
	cfg := Load()
	return &cfg.Upload
}
