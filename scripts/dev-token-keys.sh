#!/usr/bin/env bash
set -euo pipefail
umask 077

repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
key_dir="$repo_root/apps/fleet-api/.dev-keys"
private_key="$key_dir/private.pem"
jwks_file="$key_dir/jwks.json"

mkdir -p "$key_dir"

if [[ ! -f "$private_key" ]]; then
  openssl genrsa -out "$private_key" 2048
fi

node -e '
  const crypto = require("node:crypto");
  const fs = require("node:fs");
  const privateKey = fs.readFileSync(process.argv[1], "utf8");
  const { kty, n, e } = crypto.createPublicKey(privateKey).export({ format: "jwk" });
  const jwks = { keys: [{ kty, kid: "dev-1", use: "sig", alg: "RS256", n, e }] };
  fs.writeFileSync(process.argv[2], `${JSON.stringify(jwks, null, 2)}\n`);
' "$private_key" "$jwks_file"

printf 'Dev JWKS written to %s\n' "$jwks_file"