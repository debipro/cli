#!/usr/bin/env bash
# Splits a combined checksums.txt into per-OS checksum files.
set -euo pipefail

checksums_file="${1:?usage: split-checksums.sh <checksums.txt> <output-dir>}"
output_dir="${2:?usage: split-checksums.sh <checksums.txt> <output-dir>}"

mkdir -p "$output_dir"

grep '_linux_' "$checksums_file" >"$output_dir/debi-linux-checksums.txt" || : >"$output_dir/debi-linux-checksums.txt"
grep '_mac-os_' "$checksums_file" >"$output_dir/debi-mac-checksums.txt" || : >"$output_dir/debi-mac-checksums.txt"
grep '_windows_' "$checksums_file" >"$output_dir/debi-windows-checksums.txt" || : >"$output_dir/debi-windows-checksums.txt"
