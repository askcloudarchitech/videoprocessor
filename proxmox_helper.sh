#!/bin/bash

set -e

# Default branch name
DEFAULT_BRANCH="main"
REPO_URL="https://github.com/askcloudarchitech/videoprocessor/archive/refs/heads/$DEFAULT_BRANCH.tar.gz"

# Function to find the next available VM ID
find_next_vm_id() {
  local id=100
  while pct status $id &>/dev/null; do
    id=$((id + 1))
  done
  echo $id
}

# Backup existing config files if they exist
if [ -f "/root/videoprocessor/config.env" ]; then
  echo "Backing up existing config.env..."
  cp /root/videoprocessor/config.env /tmp/config.env.backup
fi

if [ -f "/root/videoprocessor/config.json" ]; then
  echo "Backing up existing config.json..."
  cp /root/videoprocessor/config.json /tmp/config.json.backup
fi

# Always re-download the repository to ensure updates
if [ -d "/root/videoprocessor" ]; then
  echo "Removing old repository..."
  rm -rf /root/videoprocessor
fi

echo "Downloading the repository..."
curl -L "$REPO_URL" -o /tmp/videoprocessor.tar.gz
tar -xzf /tmp/videoprocessor.tar.gz -C /tmp/
mv /tmp/videoprocessor-$DEFAULT_BRANCH /root/videoprocessor
rm /tmp/videoprocessor.tar.gz

# Restore backed-up config files
if [ -f "/tmp/config.env.backup" ]; then
  echo "Restoring config.env..."
  mv /tmp/config.env.backup /root/videoprocessor/config.env
fi

if [ -f "/tmp/config.json.backup" ]; then
  echo "Restoring config.json..."
  mv /tmp/config.json.backup /root/videoprocessor/config.json
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

# Find the next available VM ID
VM_ID=$(find_next_vm_id)

# Create LXC container
pct create $VM_ID local:vztmpl/$TEMPLATE.tar.gz \
  -hostname "$CONTAINER_NAME" \
  -storage "$STORAGE" \
  -net0 name=eth0,bridge=vmbr0,ip=dhcp \
  -rootfs "$STORAGE:8" \
  -memory 2048 \
  -cores 2

# Start the container
pct start $VM_ID

# Install dependencies inside the container
pct exec $VM_ID -- bash -c "\
  apt-get update && \
  apt-get install -y curl build-essential golang nodejs npm nfs-common && \
  npm install -g esbuild && \
  mkdir -p $NFS_MOUNT && \
  echo '$NFS_SERVER:$NFS_PATH $NFS_MOUNT nfs defaults 0 0' >> /etc/fstab && \
  mount -a"

# Copy application files to the container
pct push $VM_ID ./deploy /root/deploy --recursive

# Copy external config.json to the container
pct push $VM_ID ./config.json /root/deploy/config.json

# Pass the NFS_MOUNT environment variable to the application
pct exec $VM_ID -- bash -c "echo 'Environment=NFS_MOUNT=$NFS_MOUNT' >> /etc/systemd/system/videoprocessor.service"

# Set up the application as a systemd service
pct exec $VM_ID -- bash -c "\
  mv /root/deploy/videoprocessor.service /etc/systemd/system/videoprocessor.service && \
  systemctl enable videoprocessor && \
  systemctl start videoprocessor"

# Output success message
echo "LXC container '$CONTAINER_NAME' has been set up successfully with VM ID $VM_ID."