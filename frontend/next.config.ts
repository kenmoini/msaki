import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Enable static export for embedding in Go binary
  output: "export",
  distDir: "out",
  trailingSlash: true,

  // Serve UI from /ui path
  basePath: "/ui",

  // Required for static export with images
  images: {
    unoptimized: true,
  },
};

export default nextConfig;
