#!/bin/bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <app-path> <codesign-identity>"
  exit 1
fi

APP_PATH="$1"
IDENTITY="$2"
ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
APP_ENTITLEMENTS="$ROOT_DIR/electron/App.entitlements"
PROVIDER_ENTITLEMENTS="$ROOT_DIR/fileprovider/EleFileProvider/Provider.entitlements"

codesign --force --sign "$IDENTITY" "$APP_PATH/Contents/Resources/backend/zcopy-client-backend"
codesign --force --sign "$IDENTITY" "$APP_PATH/Contents/Resources/app.asar.unpacked/node_modules/electron-macos-file-provider/build/Release/efphelper.node"
codesign --force --sign "$IDENTITY" --entitlements "$PROVIDER_ENTITLEMENTS" "$APP_PATH/Contents/PlugIns/EleFileProvider.appex"
codesign --force --sign "$IDENTITY" --entitlements "$APP_ENTITLEMENTS" "$APP_PATH"
