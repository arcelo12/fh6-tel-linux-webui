#!/bin/bash
# clean-before-push.sh
# Hapus semua build artifacts sebelum git push
# Jalankan dari root repo: bash clean-before-push.sh

set -e
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

echo "🧹 Cleaning build artifacts..."

# 1. Go CLI/TUI compiled binaries
if [ -d "fh6-tel-mirror/dist" ]; then
  rm -rf fh6-tel-mirror/dist
  echo "  ✅ Removed: fh6-tel-mirror/dist/"
fi

# 2. Local Go binary
for f in fh6-tel-mirror/fh6-tel-mirror fh6-tel-mirror/fh6-tel-mirror.exe; do
  if [ -f "$f" ]; then
    rm -f "$f"
    echo "  ✅ Removed: $f"
  fi
done

# 3. Tauri Rust build artifacts (can be several GB)
if [ -d "fh6-tel-mirror-tauri/src-tauri/target" ]; then
  rm -rf fh6-tel-mirror-tauri/src-tauri/target
  echo "  ✅ Removed: fh6-tel-mirror-tauri/src-tauri/target/"
fi

# 4. Tauri frontend Vite output
if [ -d "fh6-tel-mirror-tauri/dist" ]; then
  rm -rf fh6-tel-mirror-tauri/dist
  echo "  ✅ Removed: fh6-tel-mirror-tauri/dist/"
fi

# 5. Tauri gen/ directory
if [ -d "fh6-tel-mirror-tauri/src-tauri/gen" ]; then
  rm -rf fh6-tel-mirror-tauri/src-tauri/gen
  echo "  ✅ Removed: fh6-tel-mirror-tauri/src-tauri/gen/"
fi

# 6. Main app build
if [ -d "dist" ]; then
  rm -rf dist
  echo "  ✅ Removed: dist/"
fi

# 7. Runtime logs
find . -name "mirror.log" -not -path "*/node_modules/*" -delete 2>/dev/null && echo "  ✅ Removed: mirror.log files"
find . -name "*.log" -not -path "*/node_modules/*" -not -path "*/.git/*" -delete 2>/dev/null && echo "  ✅ Removed: *.log files"

echo ""
echo "🎉 Clean complete! Safe to git push."
echo ""
echo "📊 Git status:"
git status --short
