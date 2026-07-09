import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  transpilePackages: ["@openintern/db"],
  experimental: {
    serverActions: {
      bodySizeLimit: "1mb",
    },
  },
};

export default nextConfig;
