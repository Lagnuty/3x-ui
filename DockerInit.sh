#!/bin/sh
set -eu

TARGET_ARCH="${1:-amd64}"
TARGET_VARIANT="${2:-}"
XRAY_VERSION="${3:-v26.9.30}"

case "${TARGET_ARCH}/${TARGET_VARIANT}" in
    amd64/)
        ARCH="64"
        FNAME="amd64"
        ;;
    386/ | i386/)
        ARCH="32"
        FNAME="i386"
        ;;
    arm64/* | armv8/* | aarch64/*)
        ARCH="arm64-v8a"
        FNAME="arm64"
        ;;
    arm/v6 | armv6/*)
        ARCH="arm32-v6"
        FNAME="armv6"
        ;;
    arm/v7 | arm/ | armv7/* | arm32/*)
        ARCH="arm32-v7a"
        FNAME="arm32"
        ;;
    *)
        echo "Unsupported Docker target architecture: ${TARGET_ARCH}/${TARGET_VARIANT}" >&2
        exit 1
        ;;
esac
mkdir -p build/bin
cd build/bin
curl -sfLRO "https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-${ARCH}.zip"
unzip "Xray-linux-${ARCH}.zip"
rm -f "Xray-linux-${ARCH}.zip" geoip.dat geosite.dat
mv xray "xray-linux-${FNAME}"
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRO https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat
curl -sfLRo geoip_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geoip.dat
curl -sfLRo geosite_IR.dat https://github.com/chocolate4u/Iran-v2ray-rules/releases/latest/download/geosite.dat
curl -sfLRo geoip_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geoip.dat
curl -sfLRo geosite_RU.dat https://github.com/runetfreedom/russia-v2ray-rules-dat/releases/latest/download/geosite.dat
cd ../../
