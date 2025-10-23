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

  # Prefer checking the local template cache which is the definitive location for downloaded templates
  TEMPLATE_PATH="/var/lib/vz/template/cache/$TEMPLATE"

  if [ -f "$TEMPLATE_PATH" ]; then
    echo "Template $TEMPLATE is already available at $TEMPLATE_PATH."
    return 0
  fi

  # If not present in cache, check pveam listing (use fixed-string match, not whole-line)
  if pveam list local | grep -Fq "$TEMPLATE"; then
    echo "Template $TEMPLATE is listed by pveam but not present in cache; attempting to ensure it's downloaded to cache..."
    pveam update
    pveam download local "$TEMPLATE" || echo "Warning: pveam download returned non-zero status; continuing."
  else
    echo "Template $TEMPLATE not found in cache or pveam list. Downloading..."
    pveam update
    pveam download local "$TEMPLATE"
  fi
}

# Call ensure_template before creating the container
ensure_template "$TEMPLATE"

# Ensure persistent config directory on the host exists
if [ ! -d "/root/videoprocessorconfig" ]; then
  echo "Creating /root/videoprocessorconfig directory..."
  mkdir -p /root/videoprocessorconfig
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

cd /root/videoprocessor

# Load configuration from external file early to ensure variables like $CONTAINER_NAME are available
if [ ! -f "/root/videoprocessorconfig/config.env" ]; then
  echo "config.env not found. Generating default config.env from config.env.example..."
  cp config.env.example /root/videoprocessorconfig/config.env
  echo "Please edit the generated config.env file to match your environment."
fi
source /root/videoprocessorconfig/config.env

# Ensure the config.json file exists in the persistent directory
if [ ! -f "/root/videoprocessorconfig/config.json" ]; then
  echo "Creating default config.json in persistent storage..."
  cp config.json.example "/root/videoprocessorconfig/config.json"
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
pct set $VM_ID -mp0 /mnt/$CONTAINER_NAME-nfs,mp=/media/nfs

# Add a bind mount for config.json to the LXC container
pct set $VM_ID -mp1 /root/videoprocessorconfig,mp=/root/config

# Start the container
pct start $VM_ID

# Install dependencies inside the container
pct exec $VM_ID -- bash -c "\
  apt-get update && \
  apt-get install -y curl build-essential nodejs npm nfs-common && \
  curl -fsSL https://deb.nodesource.com/setup_18.x | bash - && \
  apt-get install -y nodejs"

# Install Go 1.24.1 manually inside the container
pct exec $VM_ID -- bash -c "\
  curl -LO https://go.dev/dl/go1.24.1.linux-amd64.tar.gz && \
  rm -rf /usr/local/go && \
  tar -C /usr/local -xzf go1.24.1.linux-amd64.tar.gz && \
  rm go1.24.1.linux-amd64.tar.gz && \
  echo 'export PATH=\"$PATH:/usr/local/go/bin\"' >> /etc/profile && \
  source /etc/profile && \
  go version"

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

# Output success message
echo "LXC container '$CONTAINER_NAME' has been set up successfully with VM ID $VM_ID."