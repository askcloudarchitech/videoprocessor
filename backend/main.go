package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Config represents the structure of the configuration file.
type Config struct {
	SDCardMappings    map[string]SDCard `json:"sdCardMappings"`
	IgnoredExtensions []string          `json:"ignoredExtensions"`
	Timezone          string            `json:"timezone"`
	DestinationConfig DestinationConfig `json:"destinationConfig"`
}

type DestinationConfig struct {
	Type string `json:"type"` // "local" or "nfs"
	Path string `json:"path"`
}

var sdCardMappings map[string]SDCard
var ignoredExtensions []string
var timezone *time.Location
var destinationConfig DestinationConfig

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// LogReceiver handles centralized logging and WebSocket streaming.
type LogReceiver struct {
	clients   map[*websocket.Conn]bool
	broadcast chan string
	mu        sync.Mutex
	logs      []string // Retain the last 20 log entries
}

// NewLogReceiver creates a new LogReceiver.
func NewLogReceiver() *LogReceiver {
	return &LogReceiver{
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan string),
		logs:      make([]string, 0, 20), // Preallocate space for 20 logs
	}
}

// Start starts the LogReceiver to handle broadcasting logs.
func (lr *LogReceiver) Start() {
	go func() {
		for message := range lr.broadcast {
			lr.mu.Lock()
			// Add the log message to the in-memory log history
			if len(lr.logs) == 20 {
				lr.logs = lr.logs[1:] // Remove the oldest log if at capacity
			}
			lr.logs = append(lr.logs, message)

			// Broadcast the log message to all connected clients
			for client := range lr.clients {
				err := client.WriteMessage(websocket.TextMessage, []byte(message))
				if err != nil {
					log.Printf("Error writing to WebSocket client: %v", err)
					client.Close()
					delete(lr.clients, client)
				}
			}
			lr.mu.Unlock()
		}
	}()
}

// Log logs a message with a timestamp in the configured time zone and broadcasts it to WebSocket clients.
func (lr *LogReceiver) Log(format string, v ...interface{}) {
	timestamp := time.Now().In(timezone).Format("2006-01-02 15:04:05") // Use configured time zone
	message := fmt.Sprintf("[%s] %s", timestamp, fmt.Sprintf(format, v...))
	log.Println(message) // Log to the server console
	lr.broadcast <- message
}

// HandleWebSocket handles WebSocket connections for log streaming.
func (lr *LogReceiver) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading WebSocket connection: %v", err)
		http.Error(w, "Failed to upgrade WebSocket connection", http.StatusInternalServerError)
		return
	}

	lr.mu.Lock()
	lr.clients[conn] = true

	// Send the last 20 log entries to the newly connected client
	for _, logEntry := range lr.logs {
		err := conn.WriteMessage(websocket.TextMessage, []byte(logEntry))
		if err != nil {
			log.Printf("Error sending log history to WebSocket client: %v", err)
			conn.Close()
			delete(lr.clients, conn)
			lr.mu.Unlock()
			return
		}
	}
	lr.mu.Unlock()

	log.Printf("New WebSocket client connected")

	// Keep the connection open and listen for client disconnects
	go func() {
		defer func() {
			lr.mu.Lock()
			delete(lr.clients, conn)
			lr.mu.Unlock()
			conn.Close()
			log.Printf("WebSocket client disconnected")
		}()

		for {
			// Read messages from the client to detect disconnects
			if _, _, err := conn.ReadMessage(); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("Unexpected WebSocket close error: %v", err)
				} else {
					log.Printf("WebSocket closed: %v", err)
				}
				break
			}
		}
	}()
}

var logReceiver = NewLogReceiver()

// checkAndMountNFS checks if the NFS mount is established and mounts it if necessary.
func checkAndMountNFS() error {
	// Load NFS configuration from environment variables
	nfsServerIP := os.Getenv("NFS_SERVER")
	nfsServerPath := os.Getenv("NFS_PATH")
	nfsMountPoint := os.Getenv("NFS_MOUNT")

	if nfsServerIP == "" || nfsServerPath == "" || nfsMountPoint == "" {
		return fmt.Errorf("NFS configuration is missing. Ensure NFS_SERVER, NFS_PATH, and NFS_MOUNT are set in the environment")
	}

	// Check if the mount point is already mounted
	cmd := exec.Command("mountpoint", "-q", nfsMountPoint)
	if err := cmd.Run(); err == nil {
		logReceiver.Log("NFS mount already established at %s", nfsMountPoint)
		return nil
	}

	// Attempt to establish the NFS mount
	logReceiver.Log("NFS mount not found. Attempting to mount %s:%s to %s",
		nfsServerIP, nfsServerPath, nfsMountPoint)

	mountCmd := exec.Command("sudo", "mount", "-t", "nfs",
		nfsServerIP+":"+nfsServerPath, nfsMountPoint)
	output, err := mountCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to mount NFS: %v\nOutput: %s", err, string(output))
	}

	logReceiver.Log("Successfully mounted NFS at %s", nfsMountPoint)
	return nil
}

func main() {
	// Load configuration at startup
	configPath := "config.json"
	if err := loadConfig(configPath); err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Start the log receiver
	logReceiver.Start()

	// Check and establish NFS mount
	if err := checkAndMountNFS(); err != nil {
		log.Fatalf("Error establishing NFS mount: %v", err)
	}

	// Start the combined server (declared in web.go)
	go StartServer()

	// Continuous SD card processing
	var wg sync.WaitGroup

	for {
		time.Sleep(5 * time.Second)

		// Detect connected devices
		devices := getConnectedDevices()

		// Remove devices that are no longer connected from the processedDevices map
		for label := range processedDevices {
			if !contains(devices, label) {
				delete(processedDevices, label) // Remove from processedDevices if physically removed
				logReceiver.Log("Removed %s from processed devices", label)
			}
		}

		// Process each newly detected device
		for _, device := range devices {
			if !processedDevices[device] {
				wg.Add(1) // Increment the WaitGroup counter
				go func(device string) {
					defer func() {
						// Ensure Done() is called even if a panic occurs
						if r := recover(); r != nil {
							logReceiver.Log("Recovered from panic while processing device %s: %v", device, r)
						}
						wg.Done() // Decrement the WaitGroup counter
					}()
					logReceiver.Log("Processing SD card: %s", device)
					processSDCard(device)
					logReceiver.Log("Finished processing SD card: %s", device)
				}(device)
			}
		}
	}
}
