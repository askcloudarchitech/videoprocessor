#!/bin/bash

set -e

# Load configuration from external file
if [ ! -f "config.env" ]; then
  echo "Error: config.env file not found. Please create one based on config.env.example."
  exit 1
fi
source config.env

# Fetch the latest version of the repository
if [ ! -d "/root/videoprocessor" ]; then
  echo "Cloning the repository..."
  git clone --branch "$BRANCH" "$REPO_URL" /root/videoprocessor
else
  echo "Updating the repository..."
  cd /root/videoprocessor
  git pull origin "$BRANCH"
fi

# Change to the project directory
cd /root/videoprocessor

# Create LXC container
pct create 100 local:vztmpl/$TEMPLATE.tar.gz \
  -hostname "$CONTAINER_NAME" \
  -storage "$STORAGE" \
  -net0 name=eth0,bridge=vmbr0,ip=dhcp \
  -rootfs "$STORAGE:8" \
  -memory 2048 \
  -cores 2

# Start the container
pct start 100

# Install dependencies inside the container
pct exec 100 -- bash -c "\
  apt-get update && \
  apt-get install -y curl git build-essential golang nodejs npm nfs-common && \
  npm install -g esbuild && \
  mkdir -p $NFS_MOUNT && \
  echo '$NFS_SERVER:$NFS_PATH $NFS_MOUNT nfs defaults 0 0' >> /etc/fstab && \
  mount -a"

# Ensure the config.json file is stored on a permanent mounted storage location
CONFIG_MOUNT="/mnt/config"

# Check if the config mount exists
if [ ! -d "$CONFIG_MOUNT" ]; then
  echo "Error: Config mount $CONFIG_MOUNT does not exist. Please ensure it is mounted."
  exit 1
fi

# Copy external config.json to the container
pct push 100 "$CONFIG_MOUNT/config.json" /root/deploy/config.json

# Set up the application as a systemd service
pct exec 100 -- bash -c "\
  mv /root/deploy/videoprocessor.service /etc/systemd/system/videoprocessor.service && \
  systemctl enable videoprocessor && \
  systemctl start videoprocessor"

# Output success message
echo "LXC container '$CONTAINER_NAME' has been set up successfully."