package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SDCard represents the configuration for an SD card.
type SDCard struct {
	Name        string   `json:"name"`
	SourceDirs  []string `json:"sourceDirs"`
	Destination string   `json:"destination"`
}

var processedDevices = make(map[string]bool)

// shouldIgnoreFile checks if a file should be ignored based on its extension.
func shouldIgnoreFile(fileName string) bool {
	lowerFileName := strings.ToLower(fileName) // Convert the file name to lowercase
	for _, ext := range ignoredExtensions {
		if strings.HasSuffix(lowerFileName, strings.ToLower(ext)) { // Convert the extension to lowercase
			return true
		}
	}
	return false
}

// getConnectedDevices retrieves a list of connected devices by scanning /dev/disk/by-label.
func getConnectedDevices() []string {
	devices := []string{}
	files, err := os.ReadDir("/dev/disk/by-label")
	if err != nil {
		logReceiver.Log("Error reading /dev/disk/by-label: %v", err)
		return devices
	}

	for _, file := range files {
		name := file.Name()
		if _, exists := sdCardMappings[name]; exists {
			devices = append(devices, name) // Append the label to the devices list
			if !processedDevices[name] {    // Only log if the device is not already processed
				logReceiver.Log("Detected new device: %s", name)
			}
		}
	}
	return devices
}

// copyFiles copies files from the SD card's source directories to the specified destination.
// It returns a boolean indicating whether any files were copied.
func copyFiles(sdCard SDCard) (bool, error) {
	filesCopied := false
	for _, sourceDir := range sdCard.SourceDirs {
		sdCardPath := filepath.Join("/media/videoserver", sdCard.Name, sourceDir)

		// Check if the source directory exists
		if _, err := os.Stat(sdCardPath); os.IsNotExist(err) {
			logReceiver.Log("Source directory does not exist: %s", sdCardPath)
			continue // Skip this source directory
		}

		// List files in the source directory for debugging
		files, err := os.ReadDir(sdCardPath)
		if err != nil {
			return false, fmt.Errorf("failed to read directory %s: %v", sdCardPath, err)
		}
		logReceiver.Log("Files found in %s: %v", sdCardPath, files)

		// Ensure destination directory exists
		err = os.MkdirAll(sdCard.Destination, 0777) // Explicitly set permissions to 0777
		if err != nil {
			return false, fmt.Errorf("failed to create destination directory: %v", err)
		}

		// Copy each file individually
		for _, file := range files {
			if file.IsDir() { // Skip directories
				continue
			}

			if shouldIgnoreFile(file.Name()) {
				logReceiver.Log("Ignoring file: %s", file.Name())
				continue // Skip copying ignored files
			}

			sourceFilePath := filepath.Join(sdCardPath, file.Name())
			destinationFileName := file.Name()

			// Change .insv extension to .mp4
			if strings.HasSuffix(strings.ToLower(file.Name()), ".insv") {
				destinationFileName = strings.TrimSuffix(file.Name(), ".insv") + ".mp4"
			}

			destinationFilePath := filepath.Join(sdCard.Destination, destinationFileName)

			cmd := exec.Command("cp", sourceFilePath, destinationFilePath)
			output, err := cmd.CombinedOutput() // Capture both stdout and stderr
			if err != nil {
				return false, fmt.Errorf("failed to copy file from %s to %s: %v\nOutput: %s", sourceFilePath, destinationFilePath, err, output)
			}
			logReceiver.Log("Copied file: %s to %s", sourceFilePath, destinationFilePath)
			filesCopied = true
		}
	}
	return filesCopied, nil
}

// createProxies generates proxy files from the original media files in the destination directory.
// Files that cannot be downscaled are skipped without errors.
func createProxies(sdCard SDCard) error {
	return createProxiesForDirectory(sdCard.Destination)
}

// createProxiesForDirectory generates proxy files for a given directory.
func createProxiesForDirectory(directory string) error {
	if _, err := os.Stat(directory); err == nil {
		files, err := os.ReadDir(directory)
		if err != nil {
			return fmt.Errorf("failed to read directory %s: %v", directory, err)
		}

		// Create the Proxy subfolder if it doesn't exist
		proxyFolder := filepath.Join(directory, "Proxy")
		if err := os.MkdirAll(proxyFolder, 0777); err != nil { // Explicitly set permissions to 0777
			return fmt.Errorf("failed to create Proxy folder: %v", err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue // Skip subdirectories
			}

			originalFilePath := filepath.Join(directory, file.Name())
			proxyFilePath := filepath.Join(proxyFolder, file.Name()) // Proxy file has the same name as the original

			// Skip reprocessing if the proxy file already exists
			if _, err := os.Stat(proxyFilePath); err == nil {
				continue
			}

			// Attempt to create a proxy only for supported file types
			if strings.HasSuffix(strings.ToLower(file.Name()), ".mp4") {
				cmd := exec.Command(
					"ffmpeg", "-i", originalFilePath,
					"-vf", "scale=-1:720", // Scale height to 720 and maintain aspect ratio
					"-c:v", "libx264", "-preset", "fast", "-crf", "23",
					"-c:a", "aac", "-b:a", "128k",
					proxyFilePath,
				)
				output, err := cmd.CombinedOutput() // Capture both stdout and stderr
				if err != nil {
					logReceiver.Log("Failed to create proxy for %s: %v\nOutput: %s", originalFilePath, err, output)
					continue // Move on to the next file
				}
				logReceiver.Log("Created proxy for %s", originalFilePath)
			} else {
				logReceiver.Log("Skipping proxy creation for unsupported file: %s", originalFilePath)
			}
		}
	}
	return nil
}

// clearSDCard removes all files from the source directories on the SD card.
func clearSDCard(sdCard SDCard) error {
	for _, sourceDir := range sdCard.SourceDirs {
		sdCardPath := filepath.Join("/media/videoserver", sdCard.Name, sourceDir)
		cmd := exec.Command("find", sdCardPath, "-mindepth", "1", "-delete")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clear directory %s on SD card: %s", sourceDir, sdCard.Name)
		}
		logReceiver.Log("Cleared directory %s on SD card: %s", sourceDir, sdCard.Name)
	}
	time.Sleep(2 * time.Second) // Add a delay after clearing files
	return nil
}

// isMounted checks if the device is mounted at the desired directory.
func isMounted(label string) (bool, error) {
	mountPoint := filepath.Join("/media/videoserver", label)

	// Check if the mount point exists
	if _, err := os.Stat(mountPoint); os.IsNotExist(err) {
		logReceiver.Log("Mount point does not exist: %s", mountPoint)
		return false, nil
	}

	// Check if the device is actually mounted
	cmd := exec.Command("mountpoint", "-q", mountPoint)
	if err := cmd.Run(); err != nil {
		logReceiver.Log("Device %s is not mounted at %s", label, mountPoint)
		return false, nil
	}

	logReceiver.Log("Device %s is mounted at %s", label, mountPoint)
	return true, nil
}

// mountDevice mounts the SD card to the desired directory.
func mountDevice(label string) error {
	devicePath := "/dev/disk/by-label/" + label
	mountPoint := filepath.Join("/media/videoserver", label)

	// Ensure the desired mount point exists
	if err := os.MkdirAll(mountPoint, 0777); err != nil { // Explicitly set permissions to 0777
		return fmt.Errorf("failed to create mount point %s: %v", mountPoint, err)
	}

	// Mount the device manually to the desired directory
	cmd := exec.Command("mount", devicePath, mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mount %s to %s: %v\nOutput: %s", devicePath, mountPoint, err, output)
	}

	logReceiver.Log("Successfully mounted device %s to %s", devicePath, mountPoint)
	return nil
}

// ejectSDCard unmounts the SD card using the umount command.
func ejectSDCard(sdCard SDCard) error {
	mountPoint := filepath.Join("/media/videoserver", sdCard.Name)

	// Check if the device is actually mounted before attempting to unmount
	mounted, err := isMounted(sdCard.Name)
	if err != nil {
		return fmt.Errorf("error checking mount status for %s: %v", sdCard.Name, err)
	}
	if !mounted {
		logReceiver.Log("Device %s is not mounted. Skipping unmount.", sdCard.Name)
		return nil
	}

	cmd := exec.Command("umount", mountPoint)
	output, err := cmd.CombinedOutput() // Capture both stdout and stderr
	if err != nil {
		return fmt.Errorf("failed to unmount %s: %v\nOutput: %s", mountPoint, err, output)
	}

	logReceiver.Log("Successfully unmounted device %s", mountPoint)
	return nil
}

// processSDCard handles the entire workflow for a given SD card device.
func processSDCard(label string) {
	// Ensure the device is not processed multiple times
	if processedDevices[label] {
		logReceiver.Log("Skipping already processed device: %s", label)
		return
	}

	// Mark the device as being processed to prevent duplicate processing
	processedDevices[label] = true
	logReceiver.Log("Starting processing for device: %s", label)

	sdCard, exists := sdCardMappings[label]
	if !exists {
		logReceiver.Log("No configuration found for label: %s", label)
		return
	}

	// Check if the SD card is already mounted
	mounted, err := isMounted(label)
	if err != nil {
		logReceiver.Log("%v", err)
		return
	}

	if !mounted {
		// Attempt to mount the SD card
		if err := mountDevice(label); err != nil {
			logReceiver.Log("Error mounting device %s: %v", label, err)
			return
		}

		// Re-check if the device is mounted after attempting to mount
		mounted, err = isMounted(label)
		if err != nil || !mounted {
			logReceiver.Log("Failed to verify mount status for device %s after mounting attempt", label)
			return
		}
	}

	// Copy files from the SD card to the destination
	filesCopied, err := copyFiles(sdCard)
	if err != nil {
		logReceiver.Log("%v", err)
		return
	}

	if filesCopied {
		logReceiver.Log("Files were copied from %s, creating proxies...", label)
		if err := createProxies(sdCard); err != nil {
			logReceiver.Log("%v", err)
			return
		}

		// Verify files exist before clearing the SD card
		for _, sourceDir := range sdCard.SourceDirs {
			sdCardPath := filepath.Join("/media/videoserver", sdCard.Name, sourceDir)
			files, err := os.ReadDir(sdCardPath)
			if err != nil {
				logReceiver.Log("Error reading directory %s: %v", sdCardPath, err)
				continue
			}

			for _, file := range files {
				if !file.IsDir() {
					destinationFilePath := filepath.Join(sdCard.Destination, file.Name())
					if !verifyFileExists(destinationFilePath) {
						logReceiver.Log("File %s not found at destination %s", file.Name(), destinationFilePath)
						return
					}
				}
			}
		}

		if err := clearSDCard(sdCard); err != nil {
			logReceiver.Log("%v", err)
			return
		}
	} else {
		logReceiver.Log("No files copied from %s. Skipping proxy creation and clearing.", label)
	}

	// Eject the SD card after processing
	if err := ejectSDCard(sdCard); err != nil {
		logReceiver.Log("%v", err)
		return
	}

	logReceiver.Log("Finished processing SD card: %s", label)
}

// Helper function to check if a slice contains a specific device label
func contains(devices []string, label string) bool {
	for _, device := range devices {
		if device == label {
			return true
		}
	}
	return false
}
