# iwc-cli

Scoped package name: `@xdfnet/iwc-cli`

npm distribution package for **iWC (WeChat to Claude Code)**.

## Install

```bash
npm i -g @xdfnet/iwc-cli
```

## Usage

```bash
iwc version
iwc wechat setup
iwc start
```

## How it works

- `postinstall` downloads platform binary from GitHub Releases.
- Binary is stored under this package's `vendor/` directory.
- `iwc` command proxies all args to that binary.

## Required release assets

For tag `vX.Y.Z`, publish these files:

- `iwc-darwin-arm64.tar.gz`
- `iwc-darwin-amd64.tar.gz`
- `iwc-linux-arm64.tar.gz`
- `iwc-linux-amd64.tar.gz`
- `iwc-windows-amd64.zip`
- `iwc-windows-arm64.zip`

Each archive must contain `iwc` (or `iwc.exe` on Windows) at archive root.

## Installer env vars

- `IWC_SKIP_DOWNLOAD=1`: skip postinstall download.
- `IWC_RELEASE_TAG=vX.Y.Z`: override release tag.
- `IWC_VERSION=X.Y.Z`: override version used for default tag.
- `IWC_GH_OWNER` / `IWC_GH_REPO`: override GitHub source.
