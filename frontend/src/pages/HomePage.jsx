import React from "react";
import {
  Box,
  Typography,
  Button,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Checkbox,
  Select,
  MenuItem,
  TextField,
} from "@mui/material";
import VideoPreview from "../components/VideoPreview";

function HomePage({
  videos,
  destinations,
  selectedFiles,
  destination,
  newFolder,
  handleFileSelect,
  handleMoveFiles,
  handleDeleteVideo,
  handleReprocessProxies,
  setDestination,
  setNewFolder,
}) {
  return (
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
    </Box>
  );
}

export default HomePage;