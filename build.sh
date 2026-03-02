#!/usr/bin/env bash
set -e

TARGET=$1

if [ -z "$TARGET" ]; then
    echo "Usage:"
    echo "  ./build.sh linux-amd64"
    echo "  ./build.sh linux-arm64"
    echo "  ./build.sh android-arm64"
    exit 1
fi


build_linux_amd64() {
    echo "Building Linux AMD64..."
    export CGO_LDFLAGS="-L$(pwd)/lib/linux_amd64 -lfrida-gum -ldl -lm -lrt -lpthread"
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
    -buildmode=c-shared \
    -ldflags="-s -w -buildid=" \
    -trimpath \
    -o libembedhttplua-linux-amd64.so .
    
    strip libembedhttplua-linux-amd64.so
}

build_linux_arm64() {
    echo "Building Linux ARM64..."
    export CGO_LDFLAGS="-L$(pwd)/lib/linux_arm64 -lfrida-gum -ldl -lm -lrt -lpthread"
    CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=arm64 \
    CC=aarch64-linux-gnu-gcc \
    go build \
    -buildmode=c-shared \
    -ldflags="-s -w -buildid=" \
    -trimpath \
    -o libembedhttplua-linux-arm64.so .
    
    aarch64-linux-gnu-strip libembedhttplua-linux-arm64.so
}

build_android_arm64() {
    echo "Building Android ARM64..."
    export CGO_LDFLAGS="-L$(pwd)/lib/android_arm64 -lfrida-gum -ldl -lm -llog"
    NDK_HOME=~/Android/Sdk/ndk/25.1.8937393
    TOOLCHAIN=$NDK_HOME/toolchains/llvm/prebuilt/linux-x86_64

    CGO_ENABLED=1 \
    GOOS=android \
    GOARCH=arm64 \
    CC=$TOOLCHAIN/bin/aarch64-linux-android24-clang \
    CXX=$TOOLCHAIN/bin/aarch64-linux-android24-clang++ \
    go build \
    -buildmode=c-shared \
    -ldflags="-s -w -buildid=" \
    -trimpath \
    -o libembedhttplua-android-arm64.so .

    $TOOLCHAIN/bin/llvm-strip libembedhttplua-android-arm64.so
}

case "$TARGET" in
    linux-amd64)
        build_linux_amd64
        ;;
    linux-arm64)
        build_linux_arm64
        ;;
    android-arm64)
        build_android_arm64
        ;;
    *)
        echo "Unknown target: $TARGET"
        exit 1
        ;;
esac

echo "Build finished."