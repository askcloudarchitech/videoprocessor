package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProxyData represents the data passed to the template.
type ProxyData struct {
	Proxies      []ProxyFile
	Destinations []string
}

// ProxyFile represents a proxy file and its original counterpart.
type ProxyFile struct {
	Original        string `json:"original"`
	Proxy           string `json:"proxy"`
	DisplayOriginal string // Field for the original path with the prefix removed
}

var configLock sync.Mutex // To ensure thread-safe updates to the config file

// loadConfig loads the configuration from a JSON file.
func loadConfig(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config file: %v", err)
	}

	sdCardMappings = config.SDCardMappings
	ignoredExtensions = config.IgnoredExtensions
	destinationConfig = config.DestinationConfig // Load destinationConfig

	// Load the time zone from the configuration
	timezone, err = time.LoadLocation(config.Timezone)
	if err != nil {
		return fmt.Errorf("failed to load timezone: %v", err)
	}

	return nil
}

// ListProxyFiles lists all files in the destination directories, including those without proxies.
func ListProxyFiles(w http.ResponseWriter, r *http.Request) {
	var proxies []ProxyFile

	for _, sdCard := range sdCardMappings {
		destinationFolder := sdCard.Destination
		proxyFolder := filepath.Join(destinationFolder, "Proxy")

		files, err := os.ReadDir(destinationFolder)
		if err != nil {
			logReceiver.Log("Error reading destination folder %s: %v", destinationFolder, err)
			continue
		}

		for _, file := range files {
			if file.IsDir() || shouldIgnoreFile(file.Name()) {
				continue
			}

			originalFilePath := filepath.Join(destinationFolder, file.Name())
			proxyFilePath := filepath.Join(proxyFolder, file.Name())

			// Check if the proxy exists
			proxyPath := ""
			if _, err := os.Stat(proxyFilePath); err == nil {
				proxyPath = "/media" + strings.TrimPrefix(proxyFilePath, "/media/nfs/video_archive")
			}

			proxies = append(proxies, ProxyFile{
				Original:        originalFilePath,
				Proxy:           proxyPath, // Empty if no proxy exists
				DisplayOriginal: strings.TrimPrefix(originalFilePath, "/media/nfs/video_archive/RecentImports/"),
			})
		}
	}

	logReceiver.Log("Listed %d files (including those without proxies)", len(proxies))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(proxies)
}

// ListDestinations lists all folders in /media/nfs/video_archive.
func ListDestinations(w http.ResponseWriter, r *http.Request) {
	basePath := "/media/nfs/video_archive"
	var destinations []string

	entries, err := os.ReadDir(basePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading destinations: %v", err), http.StatusInternalServerError)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			destinations = append(destinations, filepath.Join(basePath, entry.Name()))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(destinations)
}

// CreateDestination creates a new folder in /media/nfs/video_archive.
func CreateDestination(w http.ResponseWriter, r *http.Request) {
	var request struct {
		FolderName string `json:"folderName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	newFolderPath := filepath.Join("/media/nfs/video_archive", request.FolderName)
	if err := os.MkdirAll(newFolderPath, 0777); err != nil { // Explicitly set permissions to 0777
		http.Error(w, fmt.Sprintf("Error creating folder: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "Folder created: %s", newFolderPath)
}

// verifyFileExists checks if a file exists at the given path.
func verifyFileExists(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		return true
	}
	return false
}

// MoveFiles moves the original and proxy files to the selected destination folder.
func MoveFiles(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Files       []string `json:"files"`
		Destination string   `json:"destination"`
		NewFolder   string   `json:"newFolder"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// If a new folder is specified, create it
	if request.NewFolder != "" {
		newFolderPath, err := getDestinationPath(request.NewFolder)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error determining destination path: %v", err), http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(newFolderPath, 0777); err != nil { // Explicitly set permissions to 0777
			http.Error(w, fmt.Sprintf("Error creating new folder: %v", err), http.StatusInternalServerError)
			return
		}
		request.Destination = newFolderPath
	}

	// Validate destination folder
	destinationPath, err := getDestinationPath(request.Destination)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error determining destination path: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := os.Stat(destinationPath); os.IsNotExist(err) {
		http.Error(w, "Destination folder does not exist", http.StatusBadRequest)
		return
	}

	// Move files to the destination
	for _, file := range request.Files {
		sourcePath := filepath.Join("/media/nfs", file) // Adjust source path as needed
		destinationFilePath := filepath.Join(destinationPath, filepath.Base(file))
		if err := os.Rename(sourcePath, destinationFilePath); err != nil {
			http.Error(w, fmt.Sprintf("Error moving file %s: %v", file, err), http.StatusInternalServerError)
			return
		}

		// Verify the file exists at the destination before deleting the source
		if !verifyFileExists(destinationFilePath) {
			http.Error(w, fmt.Sprintf("File %s not found at destination after move", file), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// HandleDestinations handles both GET and POST methods for the /api/destinations endpoint.
func HandleDestinations(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ListDestinations(w, r)
	case http.MethodPost:
		CreateDestination(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ReprocessProxies reprocesses all high-resolution files to create proxies.
func ReprocessProxies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		for _, sdCard := range sdCardMappings {
			if err := createProxies(sdCard); err != nil {
				logReceiver.Log("Error reprocessing proxies for SD card %s: %v", sdCard.Name, err)
			}
		}
	}()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Reprocessing started")
}

// DeleteVideo deletes the high-resolution video and its proxy.
func DeleteVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Original string `json:"original"`
		Proxy    string `json:"proxy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the paths being used for deletion
	log.Printf("Attempting to delete original file: %s", request.Original)
	log.Printf("Attempting to delete proxy file: %s", request.Proxy)

	// Correct the proxy file path
	if !strings.HasPrefix(request.Proxy, "/media/nfs/video_archive") {
		request.Proxy = filepath.Join("/media/nfs/video_archive", strings.TrimPrefix(request.Proxy, "/media"))
		log.Printf("Corrected proxy file path: %s", request.Proxy)
	}

	// Check if both files exist
	if _, err := os.Stat(request.Original); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Original file does not exist: %s", request.Original), http.StatusNotFound)
		return
	}
	if _, err := os.Stat(request.Proxy); os.IsNotExist(err) {
		http.Error(w, fmt.Sprintf("Proxy file does not exist: %s", request.Proxy), http.StatusNotFound)
		return
	}

	// Delete the high-resolution video
	if err := os.Remove(request.Original); err != nil {
		http.Error(w, fmt.Sprintf("Error deleting original file: %v", err), http.StatusInternalServerError)
		return
	}

	// Delete the proxy file
	if err := os.Remove(request.Proxy); err != nil {
		// If deleting the proxy fails, restore the original file
		log.Printf("Error deleting proxy file: %v. Restoring original file: %s", err, request.Original)
		if restoreErr := os.Rename(request.Original+".bak", request.Original); restoreErr != nil {
			log.Printf("Failed to restore original file: %v", restoreErr)
		}
		http.Error(w, fmt.Sprintf("Error deleting proxy file: %v", err), http.StatusInternalServerError)
		return
	}

	logReceiver.Log("Deleted video: %s and proxy: %s", request.Original, request.Proxy)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Video and proxy deleted successfully")
}

// FetchConfig handles fetching the current configuration.
func FetchConfig(w http.ResponseWriter, r *http.Request) {
	configLock.Lock()
	defer configLock.Unlock()

	configData, err := ioutil.ReadFile("config.json")
	if err != nil {
		http.Error(w, "Failed to read configuration", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(configData)
}

// UpdateConfig handles updating the configuration.
func UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var newConfig Config
	if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
		http.Error(w, "Invalid configuration format", http.StatusBadRequest)
		return
	}

	// Validate the new configuration
	if newConfig.DestinationConfig.Type != "nfs" && newConfig.DestinationConfig.Type != "local" {
		http.Error(w, "Invalid destination type", http.StatusBadRequest)
		return
	}
	if newConfig.Timezone == "" {
		http.Error(w, "Timezone cannot be empty", http.StatusBadRequest)
		return
	}

	// Save the new configuration
	configLock.Lock()
	defer configLock.Unlock()

	configData, err := json.MarshalIndent(newConfig, "", "  ")
	if err != nil {
		http.Error(w, "Failed to serialize configuration", http.StatusInternalServerError)
		return
	}

	if err := ioutil.WriteFile("config.json", configData, 0644); err != nil {
		http.Error(w, "Failed to save configuration", http.StatusInternalServerError)
		return
	}

	// Reload the configuration in memory
	if err := loadConfig("config.json"); err != nil {
		http.Error(w, "Failed to reload configuration", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// StartServer starts the combined HTTP server for the REST API and web interface.
func StartServer() {
	// Serve static files from the frontend/dist directory
	fs := http.FileServer(http.Dir("./frontend/dist"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/bundle.js", fs)   // Serve the bundle.js file directly
	http.Handle("/favicon.ico", fs) // Serve favicon if needed

	// Serve video proxies and other static files
	http.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir("/media/nfs/video_archive"))))

	// Serve WebSocket logs
	http.HandleFunc("/ws/logs", logReceiver.HandleWebSocket)

	// REST API routes
	http.HandleFunc("/api/proxies", ListProxyFiles)
	http.HandleFunc("/api/destinations", HandleDestinations)
	http.HandleFunc("/api/move", MoveFiles)
	http.HandleFunc("/api/reprocess", ReprocessProxies)
	http.HandleFunc("/api/delete", DeleteVideo)
	http.HandleFunc("/api/config", FetchConfig)
	http.HandleFunc("/api/config/update", UpdateConfig)

	// Fallback to index.html for React app routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html only if the requested file does not exist
		if _, err := os.Stat("./frontend/dist" + r.URL.Path); os.IsNotExist(err) {
			http.ServeFile(w, r, "./frontend/dist/index.html")
		} else {
			fs.ServeHTTP(w, r)
		}
	})

	logReceiver.Log("Starting server on :80")
	log.Fatal(http.ListenAndServe(":80", nil))
}
