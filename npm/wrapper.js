#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");
const path = require("node:path");

const platformPackages = {
  "win32 x64": "@jacobjoergensen/preflight-win32-x64",
  "darwin arm64": "@jacobjoergensen/preflight-darwin-arm64",
  "darwin x64": "@jacobjoergensen/preflight-darwin-x64",
  "linux x64": "@jacobjoergensen/preflight-linux-x64",
  "linux arm64": "@jacobjoergensen/preflight-linux-arm64",
};

const key = `${process.platform} ${process.arch}`;
const pkg = platformPackages[key];

if (!pkg) {
  console.error(
    `preflight: unsupported platform ${key}. Supported: ${Object.keys(platformPackages).join(", ")}. ` +
      `Download a binary from https://github.com/JacobJoergensen/preflight/releases instead.`
  );
  process.exit(1);
}

let binary;

try {
  const manifest = require.resolve(`${pkg}/package.json`);
  const binaryName = process.platform === "win32" ? "preflight.exe" : "preflight";
  binary = path.join(path.dirname(manifest), "bin", binaryName);
} catch {
  console.error(
    `preflight: platform package ${pkg} is not installed (optional dependencies may have been skipped).\n` +
      `  Common causes:\n` +
      `    - pnpm install --shamefully-hoist (breaks optionalDependencies resolution)\n` +
      `    - npm install --no-optional or --omit=optional\n` +
      `    - CI environment skipping optional deps\n` +
      `  Reinstall without those flags, or download a binary from\n` +
      `  https://github.com/JacobJoergensen/preflight/releases.`
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`preflight: failed to run ${binary}: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status ?? 1);
