#!/usr/bin/env bash
# pikapika 自定义核心 Windows 构建脚本 (WSL2)
# 用法:
#   ./build-win.sh          快模式: 只改 Go 核心时, 复用已编译的 Dart 产物 (~10s)
#   ./build-win.sh --full   全量: hover 重建 Dart + Go (~60s)
#   ./build-win.sh --deploy 部署到 Windows 桌面
set -e

ROOT=/home/quiz/dev/pikapika
CORE=/home/quiz/dev/pikapika-core
OUT=$ROOT/go/build/outputs/windows-release
ENGINE_CACHE=$HOME/.cache/hover/engine/windows-release
DEPLOY_DIR=/mnt/c/Users/33257/Desktop/pikapika-win

export PATH=/home/quiz/dev/flutter-sdk/flutter-2.10.3/bin:$PATH
export FLUTTER_STORAGE_BASE_URL=https://storage.flutter-io.cn
export PUB_HOSTED_URL=https://pub.flutter-io.cn
export GOPROXY=https://goproxy.cn,direct
export DISPLAY=:90 WINEDEBUG=-all

MODE=quick
[ "$1" = "--full" ] && MODE=full
[ "$1" = "--deploy" ] && MODE=deploy

# 同步核心源码
sync_core() {
  rm -rf $ROOT/go/cmd $ROOT/go/pikapika $ROOT/go/hover.yaml $ROOT/go/go.mod $ROOT/go/go.sum \
         $ROOT/go/packaging $ROOT/go/assets $ROOT/go/mobile.go $ROOT/go/.gitignore
  cp -r $CORE/cmd $CORE/pikapika $CORE/hover.yaml $CORE/go.mod $CORE/go.sum \
        $CORE/packaging $CORE/assets $CORE/mobile/mobile.go $ROOT/go/
  echo "# hover generated" > $ROOT/go/.gitignore
}

# 仅重编 Go 二进制 (快模式)
build_go() {
  cd $ROOT/go
  CGO_LDFLAGS=" -L$ENGINE_CACHE -L$OUT -lflutter_engine" \
  CGO_CFLAGS="" \
  GOOS=windows GOARCH=amd64 CGO_ENABLED=1 \
  CC="ccache x86_64-w64-mingw32-gcc" \
  go build -o $OUT/pikapika.exe ./cmd/
  echo "OK: $OUT/pikapika.exe"
}

# 全量 hover 构建
build_full() {
  cd $ROOT
  hover build windows
}

package_deploy() {
  # 清理调试符号
  rm -f $OUT/flutter_engine.pdb $OUT/flutter_engine.exp $OUT/flutter_engine.lib
  # 关掉旧进程后部署
  /mnt/c/Windows/System32/taskkill.exe -F -IM pikapika.exe 2>/dev/null || true
  cd $OUT && python3 -c "
import zipfile, os
zf = zipfile.ZipFile('/tmp/opencode/win-build.zip','w',zipfile.ZIP_DEFLATED)
for root,_,files in os.walk('.'):
    for f in files:
        p=os.path.join(root,f)
        zf.write(p, p.replace('\\\\','/').lstrip('./'))
zf.close()
"
  python3 -c "
import zipfile
zipfile.ZipFile('/tmp/opencode/win-build.zip').extractall('$DEPLOY_DIR')
print('已部署到: $DEPLOY_DIR')
"
}

case $MODE in
  full)   sync_core; build_full; package_deploy ;;
  deploy) package_deploy ;;
  *)      sync_core; build_go; package_deploy ;;
esac
