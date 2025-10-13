import React, { useState, useEffect } from "react";
import {
  Box,
  Button,
  TextField,
  Typography,
  List,
  ListItem,
  ListItemText,
  IconButton,
} from "@mui/material";
import { Delete } from "@mui/icons-material";

function ConfigEditor() {
  const [config, setConfig] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    fetch("/api/config")
      .then((res) => res.json())
      .then(setConfig)
      .catch(() => setError("Failed to load configuration"));
  }, []);

  const handleSave = () => {
    fetch("/api/config/update", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    })
      .then((res) => {
        if (!res.ok) {
          throw new Error("Failed to save configuration");
        }
        alert("Configuration saved successfully!");
      })
      .catch((err) => setError(err.message));
  };

  if (!config) {
    return <Typography>Loading...</Typography>;
  }

  return (
    <Box>
      <Typography variant="h4">Edit Configuration</Typography>

      {error && <Typography color="error">{error}</Typography>}

      <Box>
        <Typography variant="h6">Destination Config</Typography>
        <TextField
          label="Type"
          value={config.destinationConfig.type}
          onChange={(e) =>
            setConfig({
              ...config,
              destinationConfig: {
                ...config.destinationConfig,
                type: e.target.value,
              },
            })
          }
        />
        <TextField
          label="Path"
          value={config.destinationConfig.path}
          onChange={(e) =>
            setConfig({
              ...config,
              destinationConfig: {
                ...config.destinationConfig,
                path: e.target.value,
              },
            })
          }
        />
      </Box>

      <Box>
        <Typography variant="h6">SD Card Mappings</Typography>
        <List>
          {Object.entries(config.sdCardMappings).map(([key, mapping]) => (
            <ListItem key={key}>
              <ListItemText
                primary={mapping.name}
                secondary={`Source: ${mapping.sourceDirs.join(", ")}, Destination: ${mapping.destination}`}
              />
              <IconButton
                onClick={() => {
                  const newMappings = { ...config.sdCardMappings };
                  delete newMappings[key];
                  setConfig({ ...config, sdCardMappings: newMappings });
                }}
              >
                <Delete />
              </IconButton>
            </ListItem>
          ))}
        </List>
      </Box>

      <Box>
        <Typography variant="h6">Ignored Extensions</Typography>
        <List>
          {config.ignoredExtensions.map((ext, index) => (
            <ListItem key={index}>
              <ListItemText primary={ext} />
              <IconButton
                onClick={() => {
                  const newExtensions = config.ignoredExtensions.filter(
                    (e) => e !== ext
                  );
                  setConfig({ ...config, ignoredExtensions: newExtensions });
                }}
              >
                <Delete />
              </IconButton>
            </ListItem>
          ))}
        </List>
      </Box>

      <Box>
        <Typography variant="h6">Timezone</Typography>
        <TextField
          label="Timezone"
          value={config.timezone}
          onChange={(e) => setConfig({ ...config, timezone: e.target.value })}
        />
      </Box>

      <Button variant="contained" color="primary" onClick={handleSave}>
        Save Configuration
      </Button>
    </Box>
  );
}

export default ConfigEditor;