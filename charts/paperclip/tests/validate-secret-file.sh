#!/bin/sh
# Validate a live Paperclip secret-values file before any render or cluster
# contact: the file must be mode 0600 and must NOT contain a `testMode` key at
# all (any presence, including false/"false"/null, is rejected). Local-only;
# uses system Ruby YAML (already a repo dependency for the contract tests).
set -eu

file="${1:-}"
if [ -z "$file" ]; then
  echo "usage: validate-secret-file.sh <path>" >&2
  exit 1
fi
if [ ! -f "$file" ]; then
  echo "ERROR: $file does not exist" >&2
  exit 1
fi

if [ "$(uname -s)" = "Darwin" ]; then
  mode="$(stat -f '%Lp' "$file")"
else
  mode="$(stat -c '%a' "$file")"
fi
if [ "$mode" != "600" ]; then
  echo "ERROR: $file must be mode 0600 (got $mode)" >&2
  exit 1
fi

ruby - "$file" <<'RUBY'
require 'yaml'
path = ARGV.fetch(0)
data = YAML.load_file(path)
data = {} if data.nil?
if data.key?('testMode')
  warn "ERROR: #{path} sets testMode=#{data['testMode'].inspect}; testMode must be absent from a live secret file"
  exit 1
end
RUBY

echo "ok   - $file: mode 0600 and no testMode key"
