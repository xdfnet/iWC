"use strict";

const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");
const { spawnSync } = require("child_process");

const PKG_VERSION = require("../package.json").version;
const owner = process.env.IWC_GH_OWNER || "xdfnet";
const repo = process.env.IWC_GH_REPO || "iWC";

const targetMap = {
  darwin: {
    arm64: { ext: "tar.gz", asset: "iwc-darwin-arm64.tar.gz" },
    x64: { ext: "tar.gz", asset: "iwc-darwin-amd64.tar.gz" }
  },
  linux: {
    arm64: { ext: "tar.gz", asset: "iwc-linux-arm64.tar.gz" },
    x64: { ext: "tar.gz", asset: "iwc-linux-amd64.tar.gz" }
  },
  win32: {
    x64: { ext: "zip", asset: "iwc-windows-amd64.zip" },
    arm64: { ext: "zip", asset: "iwc-windows-arm64.zip" }
  }
};

function fail(msg) {
  console.error(`[iwc postinstall] ${msg}`);
  process.exit(1);
}

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function getTarget() {
  const platform = process.platform;
  const arch = process.arch;
  const entry = targetMap[platform] && targetMap[platform][arch];
  if (!entry) {
    fail(`unsupported platform: ${platform}/${arch}`);
  }
  return { platform, arch, ...entry };
}

function releaseUrl(assetName) {
  const version = process.env.IWC_VERSION || PKG_VERSION;
  const tag = process.env.IWC_RELEASE_TAG || `v${version}`;
  return `https://github.com/${owner}/${repo}/releases/download/${tag}/${assetName}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const request = https.get(url, { headers: { "User-Agent": "iwc-cli-installer" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        file.close(() => fs.rmSync(dest, { force: true }));
        return resolve(download(res.headers.location, dest));
      }
      if (res.statusCode !== 200) {
        file.close(() => fs.rmSync(dest, { force: true }));
        return reject(new Error(`download failed (${res.statusCode}) from ${url}`));
      }
      res.pipe(file);
      file.on("finish", () => file.close(resolve));
    });
    request.on("error", (err) => {
      file.close(() => fs.rmSync(dest, { force: true }));
      reject(err);
    });
  });
}

function extract(archivePath, ext, vendorDir) {
  if (ext === "tar.gz") {
    const out = spawnSync("tar", ["-xzf", archivePath, "-C", vendorDir]);
    if (out.status !== 0) {
      fail(`extract tar failed: ${(out.stderr || "").toString()}`);
    }
    return;
  }

  if (ext === "zip") {
    if (process.platform === "win32") {
      const out = spawnSync("powershell", [
        "-NoProfile",
        "-Command",
        `Expand-Archive -LiteralPath '${archivePath}' -DestinationPath '${vendorDir}' -Force`
      ]);
      if (out.status !== 0) {
        fail(`extract zip failed: ${(out.stderr || "").toString()}`);
      }
      return;
    }

    const out = spawnSync("unzip", ["-o", archivePath, "-d", vendorDir]);
    if (out.status !== 0) {
      fail(`extract zip failed: ${(out.stderr || "").toString()}`);
    }
    return;
  }

  fail(`unsupported archive format: ${ext}`);
}

function resolveBinaryPath(vendorDir) {
  const exe = process.platform === "win32" ? "iwc.exe" : "iwc";
  const direct = path.join(vendorDir, exe);
  if (fs.existsSync(direct)) {
    return direct;
  }

  const nested = path.join(vendorDir, "build", exe);
  if (fs.existsSync(nested)) {
    fs.renameSync(nested, direct);
    return direct;
  }

  fail(`binary ${exe} not found in archive`);
}

async function main() {
  if (process.env.IWC_SKIP_DOWNLOAD === "1") {
    console.log("[iwc postinstall] skipped (IWC_SKIP_DOWNLOAD=1)");
    return;
  }

  const { asset, ext } = getTarget();
  const vendorDir = path.join(__dirname, "..", "vendor");
  ensureDir(vendorDir);

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "iwc-cli-"));
  const archivePath = path.join(tmpDir, `iwc.${ext === "tar.gz" ? "tar.gz" : "zip"}`);
  const url = releaseUrl(asset);

  console.log(`[iwc postinstall] downloading ${asset}`);
  await download(url, archivePath);
  extract(archivePath, ext, vendorDir);

  const binaryPath = resolveBinaryPath(vendorDir);
  if (process.platform !== "win32") {
    fs.chmodSync(binaryPath, 0o755);
  }

  console.log("[iwc postinstall] installed", path.basename(binaryPath));
}

main().catch((err) => fail(err.message));
