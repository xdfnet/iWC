#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

const exeName = process.platform === "win32" ? "iwc.exe" : "iwc";
const binaryPath = path.join(__dirname, "..", "vendor", exeName);

if (!fs.existsSync(binaryPath)) {
  console.error("iwc binary not found.");
  console.error("Run: npm rebuild iwc-cli or reinstall the package.");
  process.exit(1);
}

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: "inherit"
});

if (result.error) {
  console.error(`failed to launch iwc: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
