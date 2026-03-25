#!/bin/bash
set -e

# ============================================================
# build-dist — Generates the dist-release directory
# This is the public Evolution Go repository (git submodule).
#
# Usage: ./tools/build-dist/build.sh
# Run from the evolution-go root directory.
# ============================================================

LOCAL_ONLY=false
[ "$1" = "--local" ] && LOCAL_ONLY=true

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
DIST_DIR="$ROOT_DIR/dist/release"
DIST_REPO="https://github.com/EvolutionAPI/evolution-go.git"
MANAGER_SRC="$ROOT_DIR/../evolution-go-manager"
OBFUSCATE_TOOL="$(dirname "$0")/obfuscate.go"

echo ""
echo "  ╔══════════════════════════════════════════╗"
echo "  ║       Evolution Go — Build dist          ║"
echo "  ╚══════════════════════════════════════════╝"
echo ""
echo "  Root:   $ROOT_DIR"
echo "  Dist:   $DIST_DIR"
echo "  Remote: $DIST_REPO"
echo ""

# ── 1. Init or clean ──
echo "[1/7] Preparing dist/release..."
if [ -d "$DIST_DIR/.git" ] || [ -f "$DIST_DIR/.git" ]; then
    find "$DIST_DIR" -mindepth 1 -maxdepth 1 -not -name '.git' -exec rm -rf {} +
    echo "  Cleaned existing repo"
elif [ "$LOCAL_ONLY" = true ]; then
    rm -rf "$DIST_DIR"
    mkdir -p "$DIST_DIR"
    echo "  Created local directory (no git)"
else
    rm -rf "$DIST_DIR"
    echo "  Cloning $DIST_REPO ..."
    git clone "$DIST_REPO" "$DIST_DIR"
    find "$DIST_DIR" -mindepth 1 -maxdepth 1 -not -name '.git' -exec rm -rf {} +
    echo "  Cloned and cleaned"
fi

# ── 2. Build manager ──
echo "[2/6] Building manager..."
if [ -d "$MANAGER_SRC/src" ]; then
    cd "$MANAGER_SRC"
    pnpm install --frozen-lockfile 2>/dev/null || pnpm install
    pnpm build
    # Copy to manager/dist inside evolution-go
    rm -rf "$ROOT_DIR/manager/dist"
    mkdir -p "$ROOT_DIR/manager/dist"
    cp -r "$MANAGER_SRC/dist/"* "$ROOT_DIR/manager/dist/"
    echo "  Built from source"
elif [ -d "$ROOT_DIR/manager/dist/index.html" ] || [ -f "$ROOT_DIR/manager/dist/index.html" ]; then
    echo "  Using existing manager/dist/"
else
    echo "  WARNING: No manager source or build found"
fi
cd "$ROOT_DIR"

# ── 3. Copy all repo files ──
echo "[3/6] Copying files..."

# Root files
for f in README.md LICENSE NOTICE TRADEMARKS.md CHANGELOG.md VERSION Makefile \
         .env.example .dockerignore Dockerfile COMMANDS.md; do
    [ -f "$ROOT_DIR/$f" ] && cp "$ROOT_DIR/$f" "$DIST_DIR/"
done

# go.mod / go.sum
cp "$ROOT_DIR/go.mod" "$ROOT_DIR/go.sum" "$DIST_DIR/"

# .gitignore
cat > "$DIST_DIR/.gitignore" << 'EOF'
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

# Directories to copy as-is
for dir in docker docs public; do
    [ -d "$ROOT_DIR/$dir" ] && cp -r "$ROOT_DIR/$dir" "$DIST_DIR/"
done

# GitHub Actions — from templates (not in repo root to avoid running on dev repo)
TEMPLATES_DIR="$(dirname "$0")/templates"
if [ -d "$TEMPLATES_DIR/.github" ]; then
    cp -r "$TEMPLATES_DIR/.github" "$DIST_DIR/"
    echo "  GitHub Actions copied from templates"
fi

# Manager dist
if [ -d "$ROOT_DIR/manager/dist" ]; then
    mkdir -p "$DIST_DIR/manager"
    cp -r "$ROOT_DIR/manager/dist" "$DIST_DIR/manager/"
fi

# cmd/
mkdir -p "$DIST_DIR/cmd/evolution-go"
cp "$ROOT_DIR/cmd/evolution-go/main.go" "$DIST_DIR/cmd/evolution-go/"

# pkg/ — everything except pkg/core/
cd "$ROOT_DIR"
find pkg -name "*.go" -not -path "pkg/core/*" | while read f; do
    mkdir -p "$DIST_DIR/$(dirname "$f")"
    cp "$f" "$DIST_DIR/$f"
done

# whatsmeow-lib — init as submodule inside dist-release
if [ -d "$DIST_DIR/.git" ] || [ -f "$DIST_DIR/.git" ]; then
    cd "$DIST_DIR"
    if [ ! -d "whatsmeow-lib/.git" ] && [ ! -f "whatsmeow-lib/.git" ]; then
        git submodule add https://github.com/EvolutionAPI/whatsmeow.git whatsmeow-lib 2>/dev/null || true
        echo "  whatsmeow-lib submodule added"
    else
        echo "  whatsmeow-lib submodule exists"
    fi
    cd "$ROOT_DIR"
else
    # Fallback: copy whatsmeow-lib files
    if [ -d "$ROOT_DIR/whatsmeow-lib" ]; then
        cp -r "$ROOT_DIR/whatsmeow-lib" "$DIST_DIR/"
        echo "  whatsmeow-lib copied (no .git in dist)"
    fi
fi

# ── 4. Obfuscate pkg/core → c0.go ──
echo "[4/6] Obfuscating pkg/core..."
mkdir -p "$DIST_DIR/pkg/core"

if [ -f "$OBFUSCATE_TOOL" ]; then
    go run "$OBFUSCATE_TOOL" "$ROOT_DIR/pkg/core" "$DIST_DIR/pkg/core/c0.go"
else
    echo "  ERROR: $OBFUSCATE_TOOL not found"
    exit 1
fi

# ── 5. Clean artifacts ──
echo "[5/6] Cleaning artifacts..."

# Remove duplicate manager/dist/dist/ if exists
rm -rf "$DIST_DIR/manager/dist/dist" 2>/dev/null

# Remove scripts/tools (private)
rm -rf "$DIST_DIR/tools" "$DIST_DIR/scripts" 2>/dev/null

# Remove CLAUDE.md (private)
rm -f "$DIST_DIR/CLAUDE.md" 2>/dev/null

# ── 6. Verify ──
echo "[6/6] Verifying..."

OK=true
for f in cmd/evolution-go/main.go go.mod go.sum pkg/core/c0.go \
         Dockerfile README.md CHANGELOG.md VERSION .gitignore; do
    if [ ! -f "$DIST_DIR/$f" ]; then
        echo "  MISSING: $f"
        OK=false
    fi
done

cd "$DIST_DIR"
GO_FILES=$(find . -name "*.go" -type f | wc -l | tr -d ' ')
TOTAL=$(find . -type f -not -path './.git/*' | wc -l | tr -d ' ')

echo ""
echo "  ┌─────────────────────────────────┐"
echo "  │  Go files:    $GO_FILES"
echo "  │  Total files: $TOTAL"
echo "  │  Manager:     $([ -f manager/dist/index.html ] && echo 'YES' || echo 'NO')"
echo "  │  Docker:      $([ -d docker ] && echo 'YES' || echo 'NO')"
echo "  │  CI/CD:       $([ -f .github/workflows/publish_docker_image.yml ] && echo 'YES' || echo 'NO')"
echo "  │  Docs:        $([ -d docs/wiki ] && echo 'YES' || echo 'NO')"
echo "  │  Public:      $([ -d public ] && echo 'YES' || echo 'NO')"
echo "  └─────────────────────────────────┘"
echo ""

if [ "$OK" != true ]; then
    echo "  ⚠ Built with warnings — check missing files"
    exit 1
fi

# ── 7. Git sync ──
if [ "$LOCAL_ONLY" = true ]; then
    echo "[7/7] Local build complete (--local, skipping git sync)"
    echo ""
    echo "  ✓ dist/release built locally"
    echo "  Test: cd dist/release && go build ./cmd/evolution-go/"
    echo ""
else
    echo "[7/7] Syncing to $DIST_REPO ..."
    cd "$DIST_DIR"
    RELEASE_VERSION=$(cat VERSION 2>/dev/null || echo "0.0.0")

    git add -A
    if git diff --staged --quiet; then
        echo "  No changes to sync"
    else
        git commit -m "release: ${RELEASE_VERSION}"
        git tag -a "${RELEASE_VERSION}" -m "Release ${RELEASE_VERSION}" 2>/dev/null || echo "  Tag ${RELEASE_VERSION} already exists"
        git push origin main --tags
        echo "  ✓ Pushed ${RELEASE_VERSION} to $DIST_REPO"
    fi

    cd "$ROOT_DIR"
    echo ""
    echo "  ✓ dist/release built and synced"
    echo ""
fi
