package drive

import (
	"log"
	"os"
)

var drivePath string

// SetDrivePath configures the simulated drive mount/path to check for presence.
func SetDrivePath(path string) {
	drivePath = path
}

// GetDrivePath returns the configured drive path.
func GetDrivePath() string {
	return drivePath
}

// IsDrivePlugged simulates drive detection by checking if the configured path exists.
// On Linux/macOS this could be replaced later with udev/diskutil integration.
func IsDrivePlugged() bool {
	if drivePath == "" {
		return false
	}
	if _, err := os.Stat(drivePath); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		log.Printf("drive check error: %v", err)
		return false
	}
}

