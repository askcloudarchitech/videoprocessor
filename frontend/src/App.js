import React, { useState, useEffect, useCallback } from "react";

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
    <Container>
      <Box>
        <Typography variant="h4">Unmoved Videos</Typography>
        <TableContainer component={Paper}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Select</TableCell>
                <TableCell>Preview</TableCell>
                <TableCell>Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {videos.map((video) => (
                <TableRow key={video.original}>
                  <TableCell>
                    <Checkbox
                      checked={selectedFiles.includes(video.original)}
                      onChange={() => handleFileSelect(video.original)}
                    />
                  </TableCell>
                  <TableCell>
                    {video.proxy ? (
                      <VideoPreview proxy={video.proxy} />
                    ) : (
                      <Typography>No proxy available</Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="contained"
                      color="error"
                      onClick={() => handleDeleteVideo(video.original, video.proxy)}
                    >
                      Delete
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>
      <Box>
        <Typography variant="h6">Destination Folder</Typography>
        <Select
          value={destination}
          onChange={(e) => setDestination(e.target.value)}
          fullWidth
        >
          {destinations.map((dest) => (
            <MenuItem key={dest} value={dest}>
              {dest}
            </MenuItem>
          ))}
        </Select>
        <TextField
          label="Or create a new folder"
          value={newFolder}
          onChange={(e) => setNewFolder(e.target.value)}
          fullWidth
        />
        <Button
          variant="contained"
          color="primary"
          onClick={handleMoveFiles}
          disabled={!selectedFiles.length || (!destination && !newFolder)}
        >
          Move Selected Videos
        </Button>
      </Box>
      <Box>
        <Typography variant="h6">Actions</Typography>
        <Button
          variant="contained"
          color="primary"
          onClick={handleReprocessProxies}
        >
          Reprocess Proxies
        </Button>
      </Box>
    </Container>
  );
}

export default App;
