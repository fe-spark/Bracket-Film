import type { NextConfig } from "next";
import os from "os";

const cpuCount = Math.max(1, os.cpus().length - 1);

const nextConfig: NextConfig = {
  output: "standalone",
  typescript: {
    ignoreBuildErrors: true,
  },
  async rewrites() {
    if (!process.env.API_URL) {
      return [];
    }
    return [
      {
        source: "/api/:path*",
        destination: `${process.env.API_URL}/:path*`,
      },
    ];
  },

  turbopack: {
    rules: {
      "*.module.less": {
        loaders: ["less-loader"],
        as: "*.module.css",
      },
    },
  },
  experimental: {
    // 自动获取 CPU 核心数量进行构建并行化
    cpus: cpuCount,
  },
};

export default nextConfig;
