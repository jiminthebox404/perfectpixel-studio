#!/usr/bin/env bash
# perfectpixel 데스크탑 앱 개발 모드 실행 (Wails dev + Vite HMR)
set -euo pipefail

cd "$(dirname "$0")"

# wails CLI 탐색 (PATH → GOPATH/bin)
if command -v wails >/dev/null 2>&1; then
  WAILS=wails
elif [ -x "$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin/wails" ]; then
  WAILS="$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin/wails"
else
  echo "오류: wails CLI를 찾을 수 없습니다." >&2
  echo "설치: go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
  exit 1
fi

# 프론트엔드 의존성 설치 (최초 1회)
if [ ! -d frontend/node_modules ]; then
  echo "frontend 의존성 설치 중..."
  (cd frontend && npm install)
fi

exec "$WAILS" dev "$@"
