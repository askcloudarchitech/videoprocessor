#!/bin/bash

set -e

# Define paths
GO_APP="backend/videoprocessor"
FRONTEND_DIR="frontend"
DIST_DIR="$FRONTEND_DIR/dist"
DEPLOY_DIR="deploy"

# Step 1: Install Node.js dependencies
echo "Installing Node.js dependencies..."
cd "$FRONTEND_DIR"
npm install
cd ..

# Step 2: Build the React frontend
echo "Building React frontend..."
if [ -d "$DIST_DIR" ]; then
  rm -rf "$DIST_DIR"
fi
node esbuild.js # Ensure the script is run from the correct directory

# Step 3: Build the Go backend
echo "Building Go backend..."
cd backend
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "../videoprocessor" main.go web.go sdcard.go
cd ..

# Step 4: Prepare deployment directory
echo "Preparing deployment directory..."
if [ -d "$DEPLOY_DIR" ]; then
  rm -rf "$DEPLOY_DIR"
fi
mkdir -p "$DEPLOY_DIR"

# Copy frontend build to deployment directory
cp -r "$DIST_DIR" "$DEPLOY_DIR/frontend"

cp videoprocessor "$DEPLOY_DIR/"
cp videoprocessor.service "$DEPLOY_DIR/"

# Deployment package is ready
echo "Deployment package is ready in the '$DEPLOY_DIR' directory."
