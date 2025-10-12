const esbuild = require("esbuild");
const fs = require("fs");
const path = require("path");

// Ensure the dist directory exists
const distDir = path.resolve(__dirname, "./frontend/dist");
if (!fs.existsSync(distDir)) {
  fs.mkdirSync(distDir, { recursive: true });
}

// Copy index.html to the dist directory
const indexHtmlPath = path.resolve(__dirname, "./frontend/public/index.html");
const distHtmlPath = path.resolve(distDir, "index.html");
fs.copyFileSync(indexHtmlPath, distHtmlPath);

// Build the React app
esbuild
  .build({
    entryPoints: ["./frontend/src/index.jsx"], // Ensure the path is relative to the current working directory
    bundle: true,
    outfile: path.resolve(distDir, "bundle.js"), // Ensure the output path is correct
    loader: { ".js": "jsx", ".jsx": "jsx" }, // Enable JSX for .js and .jsx files
    define: { "process.env.NODE_ENV": '"production"' },
    minify: true,
  })
  .catch(() => process.exit(1));
