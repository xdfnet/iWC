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

function fetchToBuffer(url, depth = 0) {
  return new Promise((resolve, reject) => {
    if (depth > 5) {
      reject(new Error(`too many redirects: ${url}`));
      return;
    }

    const request = https.get(url, { headers: { "User-Agent": "iwc-cli-installer" } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        resolve(fetchToBuffer(res.headers.location, depth + 1));
        return;
      }
      if (res.statusCode !== 200) {
        reject(new Error(`download failed (${res.statusCode}) from ${url}`));
        return;
      }

      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    });
    request.on("error", reject);
  });
}

function extractTarFromBuffer(data, vendorDir) {
  const out = spawnSync("tar", ["-xzf", "-", "-C", vendorDir], { input: data });
  if (out.status !== 0) {
    fail(`extract tar failed: ${(out.stderr || "").toString()}`);
  }
}

function extract(archivePath, ext, vendorDir, data) {
  if (ext === "tar.gz") {
    extractTarFromBuffer(data, vendorDir);
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
  const data = await fetchToBuffer(url);
  if (ext === "zip") {
    fs.writeFileSync(archivePath, data);
  }
  extract(archivePath, ext, vendorDir, data);

  const binaryPath = resolveBinaryPath(vendorDir);
  if (process.platform !== "win32") {
    fs.chmodSync(binaryPath, 0o755);
  }

  console.log("[iwc postinstall] installed", path.basename(binaryPath));

  // 检查是否已配置微信（支持新旧两个路径）
  const configPathNew = path.join(os.homedir(), ".config", "iwc", "config.toml");
  const configPathOld = path.join(os.homedir(), ".iwc", "config.toml");
  let needSetup = true;
  let configPath = configPathNew;
  if (!fs.existsSync(configPathNew) && fs.existsSync(configPathOld)) {
    configPath = configPathOld;
  }
  if (fs.existsSync(configPath)) {
    const content = fs.readFileSync(configPath, "utf8");
    if (content.includes('token = "') && !content.includes('token = ""')) {
      needSetup = false;
    }
  }

  console.log();
  if (needSetup) {
    console.log("[iwc postinstall] 检测到未配置微信，开始扫码登录...");
    console.log();
    const iwcPath = binaryPath;
    const setupResult = spawnSync(iwcPath, ["setup"], {
      stdio: "inherit",
      shell: process.platform === "win32"
    });
    if (setupResult.status !== 0) {
      console.log("[iwc postinstall] 扫码登录取消，请稍后运行 iwc setup");
    } else {
      console.log("[iwc postinstall] 微信配置完成!");
    }
  } else {
    console.log("[iwc postinstall] 检测到已配置微信，跳过扫码");
  }

  // 设置 launchd plist（macOS）
  if (process.platform === "darwin") {
    console.log();
    console.log("[iwc postinstall] 设置开机自启...");
    const home = os.homedir();
    const plistDir = path.join(home, "Library", "LaunchAgents");
    const plistPath = path.join(plistDir, "com.user.iwc.plist");
    const logDir = path.join(home, ".config", "iwc");
    ensureDir(plistDir);
    ensureDir(logDir);

    const plistContent = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.user.iwc</string>
    <key>ProgramArguments</key>
    <array>
        <string>${binaryPath}</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${logDir}/iwc.log</string>
    <key>StandardErrorPath</key>
    <string>${logDir}/iwc_error.log</string>
</dict>
</plist>
`;
    fs.writeFileSync(plistPath, plistContent);
    spawnSync("launchctl", ["load", "-w", plistPath], { stdio: "ignore" });
    console.log("[iwc postinstall] 开机自启已设置");
  }

  console.log();
  console.log("[iwc postinstall] 安装完成! 向微信发消息试试吧");
}

main().catch((err) => fail(err.message));
