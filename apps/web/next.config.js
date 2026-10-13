/** @type {import('next').NextConfig} */
const nextConfig = {
  // Standalone output produces a minimal self-contained server tree — what the
  // multi-stage Dockerfile (HEL-0.3.2) copies into the runtime image instead of
  // the full node_modules tree.
  output: 'standalone',
  reactStrictMode: true,
};

module.exports = nextConfig;
