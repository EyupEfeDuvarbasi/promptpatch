#!/bin/sh
set -eu

repo="EyupEfeDuvarbasi/promptpatch"
version="${PROMPTER_VERSION:-latest}"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in x86_64) arch=amd64;; aarch64|arm64) arch=arm64;; *) echo "Desteklenmeyen mimari: $arch" >&2; exit 1;; esac
case "$os" in linux|darwin) :;; *) echo "Windows için install-promptpatch.ps1 kullanın." >&2; exit 1;; esac
if [ "$version" = latest ]; then version=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1); fi
name="promptcheck_${version}_${os}_${arch}"
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
curl -fsSL "https://github.com/$repo/releases/download/$version/$name.tar.gz" -o "$tmp/archive.tar.gz"
curl -fsSL "https://github.com/$repo/releases/download/$version/$name.tar.gz.sha256" -o "$tmp/checksum"
(cd "$tmp" && sed "s|dist/||;s|$name.tar.gz|archive.tar.gz|" checksum | sha256sum -c -)
tar -xzf "$tmp/archive.tar.gz" -C "$tmp"
mkdir -p "$HOME/.local/bin"
install -m 0755 "$tmp/$name/prompter" "$HOME/.local/bin/prompter"
case ":$PATH:" in *":$HOME/.local/bin:"*) :;; *)
  rc="$HOME/.profile"; [ "$(basename "${SHELL:-sh}")" = zsh ] && rc="$HOME/.zshrc"
  grep -q 'Prompter PATH' "$rc" 2>/dev/null || printf '\n# Prompter PATH\nexport PATH="$HOME/.local/bin:$PATH"\n' >> "$rc"
esac
echo "Kuruldu: $HOME/.local/bin/prompter"
echo "Sonraki adım: prompter setup-codex && prompter serve"
