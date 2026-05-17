#!/usr/bin/env sh

set -eu

REPO_OWNER="${FIBE_REPO_OWNER:-fibegg}"
REPO_NAME="${FIBE_REPO_NAME:-sdk}"
VERSION="${FIBE_VERSION:-${FIBE_CLI_VERSION:-}}"
INSTALL_DIR="${FIBE_INSTALL_DIR:-/usr/local/bin}"
TOKEN=""

if [ -n "${GH_TOKEN:-}" ]; then
  TOKEN="${GH_TOKEN}"
elif [ -f /run/secrets/gh_token ] && [ -s /run/secrets/gh_token ]; then
  TOKEN="$(tr -d '\r\n' < /run/secrets/gh_token)"
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)
    FIBE_ARCH="amd64"
    ;;
  aarch64|arm64)
    FIBE_ARCH="arm64"
    ;;
  *)
    echo "ERROR: unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

if [ -n "$VERSION" ]; then
  TAG="$VERSION"
  case "$TAG" in
    v*) ;;
    *) TAG="v$TAG" ;;
  esac
  RELEASE_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/tags/${TAG}"
  echo "[fibe-installer] Resolving pinned Fibe CLI ${TAG} from ${REPO_OWNER}/${REPO_NAME}"
else
  RELEASE_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"
  echo "[fibe-installer] Resolving latest Fibe CLI from ${REPO_OWNER}/${REPO_NAME}"
fi

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

github_json() {
  url="$1"
  output="$2"
  if [ -n "$TOKEN" ]; then
    curl -fsSL --retry 2 --connect-timeout 10 --max-time 60 \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Accept: application/vnd.github+json" \
      "$url" \
      -o "$output"
  else
    curl -fsSL --retry 2 --connect-timeout 10 --max-time 60 \
      -H "Accept: application/vnd.github+json" \
      "$url" \
      -o "$output"
  fi
}

download_asset() {
  api_url="$1"
  browser_url="$2"
  output="$3"
  if [ -n "$TOKEN" ] && [ -n "$api_url" ] && [ "$api_url" != "null" ]; then
    curl -fsSL --retry 2 --connect-timeout 10 --max-time 120 \
      -H "Authorization: Bearer ${TOKEN}" \
      -H "Accept: application/octet-stream" \
      "$api_url" \
      -o "$output"
  else
    curl -fsSL --retry 2 --connect-timeout 10 --max-time 120 "$browser_url" -o "$output"
  fi
}

RELEASE_JSON="${TMP_DIR}/release.json"
github_json "$RELEASE_URL" "$RELEASE_JSON"

TAG="$(jq -r '.tag_name // empty' "$RELEASE_JSON")"
if [ -z "$TAG" ]; then
  echo "ERROR: could not resolve Fibe CLI release tag from ${RELEASE_URL}" >&2
  exit 1
fi

FILE_VERSION="${TAG#v}"
ASSET_NAME="fibe_${FILE_VERSION}_linux_${FIBE_ARCH}.tar.gz"
ASSET_API_URL="$(jq -r --arg name "$ASSET_NAME" '.assets[] | select(.name == $name) | .url' "$RELEASE_JSON" | head -n 1)"
ASSET_BROWSER_URL="$(jq -r --arg name "$ASSET_NAME" '.assets[] | select(.name == $name) | .browser_download_url' "$RELEASE_JSON" | head -n 1)"

if [ -z "$ASSET_BROWSER_URL" ] || [ "$ASSET_BROWSER_URL" = "null" ]; then
  echo "ERROR: could not find release asset ${ASSET_NAME} for ${TAG}" >&2
  echo "Available assets:" >&2
  jq -r '.assets[].name' "$RELEASE_JSON" >&2
  exit 1
fi

echo "[fibe-installer] Downloading ${ASSET_NAME}"
ARCHIVE_PATH="${TMP_DIR}/fibe.tar.gz"
download_asset "$ASSET_API_URL" "$ASSET_BROWSER_URL" "$ARCHIVE_PATH"

CHECKSUMS_API_URL="$(jq -r '.assets[] | select(.name == "checksums.txt") | .url' "$RELEASE_JSON" | head -n 1)"
CHECKSUMS_BROWSER_URL="$(jq -r '.assets[] | select(.name == "checksums.txt") | .browser_download_url' "$RELEASE_JSON" | head -n 1)"
if [ "${FIBE_NO_VERIFY:-false}" != "true" ] && [ -n "$CHECKSUMS_BROWSER_URL" ] && [ "$CHECKSUMS_BROWSER_URL" != "null" ]; then
  CHECKSUMS_PATH="${TMP_DIR}/checksums.txt"
  if download_asset "$CHECKSUMS_API_URL" "$CHECKSUMS_BROWSER_URL" "$CHECKSUMS_PATH"; then
    EXPECTED="$(grep " ${ASSET_NAME}\$" "$CHECKSUMS_PATH" | awk '{print $1}' | head -n 1)"
    if [ -n "$EXPECTED" ]; then
      if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL="$(sha256sum "$ARCHIVE_PATH" | awk '{print $1}')"
      elif command -v shasum >/dev/null 2>&1; then
        ACTUAL="$(shasum -a 256 "$ARCHIVE_PATH" | awk '{print $1}')"
      else
        echo "[fibe-installer] No sha256sum or shasum found; skipping checksum verification"
        ACTUAL="$EXPECTED"
      fi
      if [ "$EXPECTED" != "$ACTUAL" ]; then
        echo "ERROR: checksum mismatch for ${ASSET_NAME}" >&2
        echo "Expected: ${EXPECTED}" >&2
        echo "Actual:   ${ACTUAL}" >&2
        exit 1
      fi
      echo "[fibe-installer] Checksum verified"
    else
      echo "[fibe-installer] ${ASSET_NAME} is not listed in checksums.txt; skipping checksum verification"
    fi
  else
    echo "[fibe-installer] Could not download checksums.txt; skipping checksum verification"
  fi
fi

EXTRACT_DIR="${TMP_DIR}/extract"
mkdir -p "$EXTRACT_DIR" "$INSTALL_DIR"
tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR" fibe

if [ ! -f "${EXTRACT_DIR}/fibe" ]; then
  echo "ERROR: fibe binary not found in ${ASSET_NAME}" >&2
  exit 1
fi

install -m 0755 "${EXTRACT_DIR}/fibe" "${INSTALL_DIR}/fibe.new"
mv "${INSTALL_DIR}/fibe.new" "${INSTALL_DIR}/fibe"
echo "[fibe-installer] Installed fibe ${FILE_VERSION} to ${INSTALL_DIR}/fibe"
