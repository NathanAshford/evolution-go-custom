#!/bin/bash
set -e

# ============================================================
# build-beta — Generates beta-release directory
# Targets: https://github.com/EvolutionAPI/evolution-go-beta
# Same structure as dist-release but with beta versioning.
#
# Usage: ./tools/build-dist/build-beta.sh
# Run from the evolution-go root directory.
# ============================================================

LOCAL_ONLY=false
[ "$1" = "--local" ] && LOCAL_ONLY=true

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
BETA_DIR="$ROOT_DIR/dist/beta"
BETA_REPO="https://github.com/EvolutionAPI/evolution-go-beta.git"
MANAGER_SRC="$ROOT_DIR/../evolution-go-manager"
OBFUSCATE_TOOL="$(dirname "$0")/obfuscate.go"
TEMPLATES_DIR="$(dirname "$0")/templates"
VERSION=$(cat "$ROOT_DIR/VERSION" 2>/dev/null || echo "0.0.0")
BETA_VERSION="${VERSION}-beta"

echo ""
echo "  ╔══════════════════════════════════════════╗"
echo "  ║     Evolution Go — Build BETA            ║"
echo "  ╚══════════════════════════════════════════╝"
echo ""
echo "  Root:    $ROOT_DIR"
echo "  Beta:    $BETA_DIR"
echo "  Version: $BETA_VERSION"
echo "  Remote:  $BETA_REPO"
echo ""

# ── 1. Init or clean ──
echo "[1/8] Preparing dist/beta..."
if [ -d "$BETA_DIR/.git" ] || [ -f "$BETA_DIR/.git" ]; then
    find "$BETA_DIR" -mindepth 1 -maxdepth 1 -not -name '.git' -exec rm -rf {} +
    echo "  Cleaned existing repo"
elif [ "$LOCAL_ONLY" = true ]; then
    rm -rf "$BETA_DIR"
    mkdir -p "$BETA_DIR"
    echo "  Created local directory (no git)"
else
    rm -rf "$BETA_DIR"
    echo "  Cloning $BETA_REPO ..."
    git clone "$BETA_REPO" "$BETA_DIR"
    find "$BETA_DIR" -mindepth 1 -maxdepth 1 -not -name '.git' -exec rm -rf {} +
    echo "  Cloned and cleaned"
fi

# ── 2. Build manager ──
echo "[2/7] Building manager..."
if [ -d "$MANAGER_SRC/src" ]; then
    cd "$MANAGER_SRC"
    pnpm install --frozen-lockfile 2>/dev/null || pnpm install
    pnpm build
    rm -rf "$ROOT_DIR/manager/dist"
    mkdir -p "$ROOT_DIR/manager/dist"
    cp -r "$MANAGER_SRC/dist/"* "$ROOT_DIR/manager/dist/"
    echo "  Built from source"
elif [ -f "$ROOT_DIR/manager/dist/index.html" ]; then
    echo "  Using existing manager/dist/"
else
    echo "  WARNING: No manager found"
fi
cd "$ROOT_DIR"

# ── 3. Copy files ──
echo "[3/7] Copying files..."

for f in README.md LICENSE NOTICE TRADEMARKS.md CHANGELOG.md COMMANDS.md \
         .env.example .dockerignore Dockerfile Makefile; do
    [ -f "$ROOT_DIR/$f" ] && cp "$ROOT_DIR/$f" "$BETA_DIR/"
done

echo "$BETA_VERSION" > "$BETA_DIR/VERSION"

cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$BETA_DIR/"

cat > "$BETA_DIR/.gitignore" << 'EOF'
.env
logs/*
build/
*.prof
coverage.*
.air.toml
.idea/
.vscode/
.DS_Store
.cursorrules
EOF

for dir in docker docs public; do
    [ -d "$ROOT_DIR/$dir" ] && cp -r "$ROOT_DIR/$dir" "$BETA_DIR/"
done

# GitHub Actions from templates
if [ -d "$TEMPLATES_DIR/.github" ]; then
    cp -r "$TEMPLATES_DIR/.github" "$BETA_DIR/"
fi

# Manager dist
if [ -d "$ROOT_DIR/manager/dist" ]; then
    mkdir -p "$BETA_DIR/manager"
    cp -r "$ROOT_DIR/manager/dist" "$BETA_DIR/manager/"
fi

# cmd/
mkdir -p "$BETA_DIR/cmd/evolution-go"
cp "$ROOT_DIR/cmd/evolution-go/main.go" "$BETA_DIR/cmd/evolution-go/"

# pkg/ (except core)
cd "$ROOT_DIR"
find pkg -name "*.go" -not -path "pkg/core/*" | while read f; do
    mkdir -p "$BETA_DIR/$(dirname "$f")"
    cp "$f" "$BETA_DIR/$f"
done

# whatsmeow-lib submodule
if [ -d "$BETA_DIR/.git" ] || [ -f "$BETA_DIR/.git" ]; then
    cd "$BETA_DIR"
    if [ ! -d "whatsmeow-lib/.git" ] && [ ! -f "whatsmeow-lib/.git" ]; then
        git submodule add https://github.com/EvolutionAPI/whatsmeow.git whatsmeow-lib 2>/dev/null || true
    fi
    cd "$ROOT_DIR"
fi

# ── 4. Obfuscate pkg/core ──
echo "[4/7] Obfuscating pkg/core..."
mkdir -p "$BETA_DIR/pkg/core"
if [ -f "$OBFUSCATE_TOOL" ]; then
    go run "$OBFUSCATE_TOOL" "$ROOT_DIR/pkg/core" "$BETA_DIR/pkg/core/c0.go"
else
    echo "  ERROR: $OBFUSCATE_TOOL not found"
    exit 1
fi

# ── 5. Patch Docker workflow for beta ──
echo "[5/7] Patching workflow for beta..."
WORKFLOW="$BETA_DIR/.github/workflows/publish_docker_image.yml"
if [ -f "$WORKFLOW" ]; then
    cat > "$WORKFLOW" << 'WORKFLOW_EOF'
name: Build and Publish Docker Image (Beta)

on:
  push:
    branches:
      - main
    tags:
      - "*.*.*-beta"

jobs:
  build_deploy:
    name: Build and Deploy Beta
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          submodules: recursive

      - name: Docker meta
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: evoapicloud/evolution-go
          tags: |
            type=raw,value=beta
            type=semver,pattern={{version}}

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKER_USERNAME }}
          password: ${{ secrets.DOCKER_PASSWORD }}

      - name: Build and push
        id: docker_build
        uses: docker/build-push-action@v5
        with:
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

      - name: Image digest
        run: echo ${{ steps.docker_build.outputs.digest }}
WORKFLOW_EOF
    echo "  Workflow patched for beta"
fi

# ── 6. Clean ──
echo "[6/7] Cleaning..."
rm -rf "$BETA_DIR/manager/dist/dist" 2>/dev/null
rm -rf "$BETA_DIR/tools" "$BETA_DIR/scripts" 2>/dev/null
rm -f "$BETA_DIR/CLAUDE.md" 2>/dev/null

# ── 7. Verify ──
echo "[7/7] Verifying..."

OK=true
for f in cmd/evolution-go/main.go go.mod go.sum pkg/core/c0.go \
         Dockerfile README.md VERSION .gitignore; do
    if [ ! -f "$BETA_DIR/$f" ]; then
        echo "  MISSING: $f"
        OK=false
    fi
done

cd "$BETA_DIR"
GO_FILES=$(find . -name "*.go" -type f | wc -l | tr -d ' ')
TOTAL=$(find . -type f -not -path './.git/*' | wc -l | tr -d ' ')

echo ""
echo "  ┌──────────────────────────────────┐"
echo "  │  Version:    $BETA_VERSION"
echo "  │  Docker tag: beta"
echo "  │  Go files:   $GO_FILES"
echo "  │  Total:      $TOTAL"
echo "  │  Manager:    $([ -f manager/dist/index.html ] && echo 'YES' || echo 'NO')"
echo "  └──────────────────────────────────┘"
echo ""

if [ "$OK" != true ]; then
    echo "  ⚠ Built with warnings"
    exit 1
fi

# ── 8. Git sync ──
if [ "$LOCAL_ONLY" = true ]; then
    echo "[8/8] Local build complete (--local, skipping git sync)"
    echo ""
    echo "  ✓ dist/beta built locally"
    echo "  Test: cd dist/beta && go build ./cmd/evolution-go/"
    echo ""
else
    echo "[8/8] Syncing to $BETA_REPO ..."
    cd "$BETA_DIR"

    git add -A
    if git diff --staged --quiet; then
        echo "  No changes to sync"
    else
        git commit -m "beta: ${BETA_VERSION}"
        git tag -a "${BETA_VERSION}" -m "Beta ${BETA_VERSION}" 2>/dev/null || echo "  Tag ${BETA_VERSION} already exists"
        git push origin main --tags
        echo "  ✓ Pushed ${BETA_VERSION} to $BETA_REPO"
    fi

    cd "$ROOT_DIR"
    echo ""
    echo "  ✓ dist/beta built and synced"
    echo "  Docker: evoapicloud/evolution-go:beta"
    echo ""
fi
