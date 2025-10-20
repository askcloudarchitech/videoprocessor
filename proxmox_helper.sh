#!/bin/bash

set -e

# Default branch name
DEFAULT_BRANCH="main"
REPO_URL="https://github.com/askcloudarchitech/videoprocessor/archive/refs/heads/$DEFAULT_BRANCH.tar.gz"

# Default template name
TEMPLATE="debian-11-standard_11.7-1_amd64.tar.zst"

# Function to find the next available VM ID
find_next_vm_id() {
  local id=100
  while [ -e "/etc/pve/lxc/$id.conf" ] || [ -e "/etc/pve/qemu-server/$id.conf" ]; do
    id=$((id + 1))
  done
  echo $id
}

# Ensure the required template is available
ensure_template() {
  echo "ensure_template called with TEMPLATE: '$TEMPLATE'"
  echo "Checking for template: '$TEMPLATE'"
  if ! pveam list local | grep -Fxq "$TEMPLATE"; then
    echo "Template $TEMPLATE not found. Downloading..."
    pveam update
    pveam download local "$TEMPLATE"
  else
    echo "Template $TEMPLATE is already available."
  fi
}

# Call ensure_template before creating the container
ensure_template "$TEMPLATE"

# Backup existing config files if they exist
if [ -f "/root/videoprocessor/config.env" ]; then
  echo "Backing up existing config.env..."
  cp /root/videoprocessor/config.env /tmp/config.env.backup
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

cd /root/videoprocessor

# Load configuration from external file early to ensure variables like $CONTAINER_NAME are available
if [ ! -f "config.env" ]; then
  echo "config.env not found. Generating default config.env from config.env.example..."
  cp config.env.example config.env
  echo "Please edit the generated config.env file to match your environment before proceeding."
  exit 1
fi
source config.env

# Ensure the directory for persistent config storage exists on the host
persistent_config_dir="/mnt/$CONTAINER_NAME-config"
if [ ! -d "$persistent_config_dir" ]; then
  echo "Creating persistent config directory: $persistent_config_dir"
  mkdir -p "$persistent_config_dir"
fi

# Ensure the config.json file exists in the persistent directory
if [ ! -f "$persistent_config_dir/config.json" ]; then
  echo "Creating default config.json in persistent storage..."
  cp config.json.example "$persistent_config_dir/config.json"
fi

# Check if a container with the name already exists
existing_vm_id=$(pct list | awk -v name="$CONTAINER_NAME" '$3 == name {print $1}')

if [ -n "$existing_vm_id" ]; then
  echo "Container with name $CONTAINER_NAME already exists (VM ID: $existing_vm_id). Replacing it..."
  pct stop $existing_vm_id
  pct destroy $existing_vm_id
fi

# Find the next available VM ID
VM_ID=$(find_next_vm_id)

# Ensure the mount point directory exists before adding the NFS share to /etc/fstab
if [ ! -d "/mnt/$CONTAINER_NAME-nfs" ]; then
  echo "Creating mount point directory: /mnt/$CONTAINER_NAME-nfs"
  mkdir -p "/mnt/$CONTAINER_NAME-nfs"
fi

# Add the NFS share to /etc/fstab on the Proxmox host
fstab_entry="$NFS_SERVER:$NFS_PATH /mnt/$CONTAINER_NAME-nfs nfs defaults 0 0"
if ! grep -Fxq "$fstab_entry" /etc/fstab; then
  echo "Adding NFS share to /etc/fstab..."
  echo "$fstab_entry" >> /etc/fstab
  systemctl daemon-reload
  mount -a
else
  echo "NFS share is already in /etc/fstab."
fi

# Create LXC container
pct create $VM_ID local:vztmpl/$TEMPLATE \
  -hostname "$CONTAINER_NAME" \
  -storage "$STORAGE" \
  -net0 name=eth0,bridge=vmbr0,ip=dhcp \
  -rootfs "$STORAGE:8" \
  -memory 2048 \
  -cores 2

# Add a bind mount to the LXC container using the standard `pct set` command
pct set $VM_ID -mp0 /mnt/$CONTAINER_NAME-nfs,mp=$NFS_MOUNT

# Start the container
pct start $VM_ID

# Install dependencies inside the container
pct exec $VM_ID -- bash -c "\
  apt-get update && \
  apt-get install -y curl build-essential golang nodejs npm nfs-common"

# Pass the NFS_MOUNT environment variable to the application
pct exec $VM_ID -- bash -c "echo 'Environment=NFS_MOUNT=$NFS_MOUNT' >> /etc/systemd/system/videoprocessor.service"

# Create a tarball of the source code
source_tarball="/tmp/source.tar.gz"
echo "Creating tarball of the source code at $source_tarball..."
tar -czf "$source_tarball" .

# Push the tarball into the container
pct push $VM_ID "$source_tarball" /root/source.tar.gz

# Extract the tarball inside the container
pct exec $VM_ID -- bash -c "\
  cd /root && \
  tar -xzf source.tar.gz && \
  rm source.tar.gz"

# Run the build script inside the container
pct exec $VM_ID -- bash -c "\
  cd /root && \
  chmod +x build.sh && \
  ./build.sh"

# Set up the application as a systemd service
pct exec $VM_ID -- bash -c "\
  mv /root/deploy/videoprocessor.service /etc/systemd/system/videoprocessor.service && \
  systemctl enable videoprocessor && \
  systemctl start videoprocessor"

# Add a bind mount for config.json to the LXC container
pct set $VM_ID -mp1 $persistent_config_dir/config.json,mp=/root/deploy/config.json

# Output success message
echo "LXC container '$CONTAINER_NAME' has been set up successfully with VM ID $VM_ID."