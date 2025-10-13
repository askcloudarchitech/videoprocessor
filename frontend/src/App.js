import React, { useState, useEffect, useCallback } from "react";
import { BrowserRouter as Router, Route, Routes } from "react-router-dom";
import ConfigEditor from "./pages/ConfigEditor";
import HomePage from "./pages/HomePage";

function App() {
  const [videos, setVideos] = useState([]);
  const [destinations, setDestinations] = useState([]);
  const [selectedFiles, setSelectedFiles] = useState([]);
  const [destination, setDestination] = useState("");
  const [newFolder, setNewFolder] = useState("");

  const fetchVideos = useCallback(() => {
    fetch("/api/proxies")
      .then((res) => res.json())
      .then((data) => {
        setVideos(data);
      });
  }, []);

  const fetchDestinations = useCallback(() => {
    fetch("/api/destinations")
      .then((res) => res.json())
      .then((data) => setDestinations(data));
  }, []);

  useEffect(() => {
    fetchVideos();
    fetchDestinations();
  }, [fetchVideos, fetchDestinations]);

  const handleFileSelect = (file) => {
    setSelectedFiles((prev) =>
      prev.includes(file) ? prev.filter((f) => f !== file) : [...prev, file]
    );
  };

  const handleMoveFiles = async () => {
    const payload = {
      files: selectedFiles,
      destination,
      newFolder,
    };

    const response = await fetch("/api/move", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    if (response.ok) {
      alert("Files moved successfully!");
      fetchVideos(); // Refresh the video list after moving files

      // If a new folder was created, add it to the dropdown
      if (newFolder) {
        setDestinations((prev) => [...prev, `${destination}/${newFolder}`]);
      }

      // Reset the form
      setSelectedFiles([]);
      setDestination("");
      setNewFolder("");
    } else {
      alert("Error moving files.");
    }
  };

  const handleDeleteVideo = async (original, proxy) => {
    if (!window.confirm("Are you sure you want to delete this video and its proxy?")) return;

    const payload = { original, proxy };

    const response = await fetch("/api/delete", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    if (response.ok) {
      alert("Video and proxy deleted successfully!");
      fetchVideos(); // Refresh the video list after deletion
    } else {
      alert("Error deleting video.");
    }
  };

  const handleReprocessProxies = async () => {
    if (!window.confirm("Reprocess all high-resolution files?")) return;

    const response = await fetch("/api/reprocess", { method: "POST" });

    if (response.ok) {
      alert("Reprocessing started successfully!");
    } else {
      alert("Error starting reprocessing.");
    }
  };

  return (
    <Router>
      <Routes>
        <Route
          path="/"
          element={
            <HomePage
              videos={videos}
              destinations={destinations}
              selectedFiles={selectedFiles}
              destination={destination}
              newFolder={newFolder}
              handleFileSelect={handleFileSelect}
              handleMoveFiles={handleMoveFiles}
              handleDeleteVideo={handleDeleteVideo}
              handleReprocessProxies={handleReprocessProxies}
              setDestination={setDestination}
              setNewFolder={setNewFolder}
            />
          }
        />
        <Route path="/config" element={<ConfigEditor />} />
      </Routes>
    </Router>
  );
}

export default App;
