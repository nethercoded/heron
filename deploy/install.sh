#!/bin/sh
set -eu

REPO="nethercoded/heron"
BIN_DIR="/usr/local/bin"
CONF_DIR="/etc/heron"
UNIT="/etc/systemd/system/heron.service"
VERSION="${HERON_VERSION:-latest}"

say() { printf '\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "run as root: curl -fsSL https://raw.githubusercontent.com/$REPO/main/deploy/install.sh | sudo sh"
[ "$(uname -s)" = "Linux" ] || die "only Linux is supported by this installer"
command -v systemctl >/dev/null || die "systemd is required"
command -v curl >/dev/null || die "curl is required"

case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 ;;
	aarch64 | arm64) ARCH=arm64 ;;
	*) die "unsupported architecture $(uname -m)" ;;
esac

if [ "$VERSION" = "latest" ]; then
	VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
	[ -n "$VERSION" ] || die "could not determine the latest release"
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

ARCHIVE="heron_${VERSION#v}_linux_${ARCH}.tar.gz"
say "downloading heron $VERSION for linux/$ARCH"
curl -fsSL -o "$TMP/$ARCHIVE" "https://github.com/$REPO/releases/download/$VERSION/$ARCHIVE"
curl -fsSL -o "$TMP/checksums.txt" "https://github.com/$REPO/releases/download/$VERSION/checksums.txt"

(cd "$TMP" && grep " $ARCHIVE\$" checksums.txt | sha256sum -c - >/dev/null) || die "checksum mismatch"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

if ! id heron >/dev/null 2>&1; then
	say "creating system user heron"
	useradd --system --home-dir /var/lib/heron --shell /usr/sbin/nologin heron
fi

install -m 0755 "$TMP/heron" "$BIN_DIR/heron"
install -d -m 0750 -o root -g heron "$CONF_DIR"

if [ ! -f "$CONF_DIR/heron.toml" ]; then
	install -m 0640 -o root -g heron "$TMP/heron.example.toml" "$CONF_DIR/heron.toml"
	say "wrote $CONF_DIR/heron.toml, set public_url before going live"
fi

install -m 0644 "$TMP/deploy/heron.service" "$UNIT"
systemctl daemon-reload
systemctl enable --now heron >/dev/null 2>&1
systemctl restart heron

say "heron $("$BIN_DIR/heron" version | cut -d' ' -f2) is running"
sleep 1
code=$(journalctl -u heron -n 50 --no-pager 2>/dev/null | sed -n 's/.*setup_code[=":]*\([A-Za-z0-9_-]*\).*/\1/p' | tail -n1)
say "open http://$(hostname -I 2>/dev/null | cut -d' ' -f1):8080/admin to create the administrator"
if [ -n "$code" ]; then
	say "setup code: $code"
fi
