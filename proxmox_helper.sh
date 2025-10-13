#!/bin/bash

set -e

# Default branch name
DEFAULT_BRANCH="main"

# Ensure the repository is cloned before generating config files
if [ ! -d "/root/videoprocessor" ]; then
  echo "Cloning the repository..."
  git clone --branch "$DEFAULT_BRANCH" "https://github.com/askcloudarchitech/videoprocessor.git" /root/videoprocessor
else
  echo "Repository already cloned. Pulling latest changes..."
  cd /root/videoprocessor
  git pull origin "$DEFAULT_BRANCH"
fi

cd /root/videoprocessor

# Load configuration from external file
if [ ! -f "config.env" ]; then
  echo "config.env not found. Generating default config.env from config.env.example..."
  cp config.env.example config.env
  echo "Please edit the generated config.env file to match your environment before proceeding."
  exit 1
fi
source config.env

# Ensure config.json exists
if [ ! -f "config.json" ]; then
  echo "config.json not found. Generating default config.json from config.json.example..."
  cp config.json.example config.json
  echo "Please edit the generated config.json file to match your environment before proceeding."
  exit 1
fi

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

# Copy application files to the container
pct push 100 ./deploy /root/deploy --recursive

# Copy external config.json to the container
pct push 100 ./config.json /root/deploy/config.json

# Pass the NFS_MOUNT environment variable to the application
Environment="NFS_MOUNT=$NFS_MOUNT"

# Set up the application as a systemd service
pct exec 100 -- bash -c "\
  mv /root/deploy/videoprocessor.service /etc/systemd/system/videoprocessor.service && \
  systemctl enable videoprocessor && \
  systemctl start videoprocessor"

# Output success message
echo "LXC container '$CONTAINER_NAME' has been set up successfully."