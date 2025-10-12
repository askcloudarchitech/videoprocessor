import React, { useEffect, useState, useRef } from "react";

function LogViewer({ onNewProxy }) {
  const [logs, setLogs] = useState([]);
  const socketRef = useRef(null); // Store the WebSocket instance
  const reconnectAttempts = useRef(0); // Track reconnection attempts

  useEffect(() => {
    const connectWebSocket = () => {
      // Ensure no duplicate WebSocket connections
      if (socketRef.current) {
        socketRef.current.close();
      }

      const socket = new WebSocket(`ws://${window.location.host}/ws/logs`);
      socketRef.current = socket;

      socket.onopen = () => {
        console.log("WebSocket connected");
        reconnectAttempts.current = 0; // Reset reconnection attempts on successful connection
      };

      socket.onmessage = (event) => {
        const logMessage = event.data;

        // Avoid adding duplicate log messages
        setLogs((prevLogs) => {
          if (prevLogs.includes(logMessage)) {
            return prevLogs; // Skip duplicate messages
          }
          return [logMessage, ...prevLogs];
        });

        // Check if the log indicates a new proxy was created
        if (logMessage.includes("Created proxy for")) {
          onNewProxy(); // Trigger the callback to refresh the video list
        }
      };

      socket.onerror = (error) => {
        console.error("WebSocket error:", error);
      };

      socket.onclose = (event) => {
        if (event.wasClean) {
          console.log("WebSocket closed cleanly");
        } else {
          console.warn(
            `WebSocket disconnected unexpectedly (code: ${event.code}, reason: ${event.reason}). Attempting to reconnect...`
          );
          const timeout = Math.min(
            1000 * 2 ** reconnectAttempts.current,
            30000
          ); // Exponential backoff, max 30 seconds
          reconnectAttempts.current += 1;
          setTimeout(connectWebSocket, timeout);
        }
      };
    };

    connectWebSocket();

    return () => {
      if (socketRef.current) {
        socketRef.current.close(); // Clean up the WebSocket connection on unmount
      }
    };
  }, [onNewProxy]);

  return (
    <div
      style={{
        height: "100%",
        overflowY: "auto",
        backgroundColor: "#f4f4f4",
        padding: "10px",
        border: "1px solid #ddd",
        width: "100%", // Ensure it takes the full width of the column
        boxSizing: "border-box",
        wordWrap: "break-word", // Ensure long messages wrap
        whiteSpace: "pre-wrap", // Preserve whitespace and wrap long lines
      }}
    >
      <h3>Server Logs</h3>
      <div>
        {logs.map((log, index) => {
          const [timestamp, ...messageParts] = log.split("] ");
          const message = messageParts.join("] "); // Rejoin in case the message contains "]"
          return (
            <div key={index} style={{ marginBottom: "10px" }}>
              <div>{message}</div>
              <div style={{ fontSize: "0.8em", color: "#888" }}>
                {timestamp.replace("[", "")}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default LogViewer;
