#!/usr/bin/env bash
#
# One-time bootstrap for publishing terraform-provider-hubspot to Understory's
# Terraform Cloud private provider registry.
#
# What it does, in order:
#   1. Verifies prerequisites (gpg, gh, jq, curl, a TFC token).
#   2. Generates a GPG keypair (4096 RSA, no passphrase) if one isn't already
#      present for the configured email. Re-running is idempotent.
#   3. Exports the public + private keys and the fingerprint to ./.tfc-setup/
#      (git-ignored).
#   4. Sets the GPG_PRIVATE_KEY secret on the GitHub repo via `gh secret set`,
#      so the goreleaser workflow can sign artifacts.
#   5. Prints (does NOT execute) the curl commands needed to:
#        a. Upload the GPG public key to the TFC org's GPG keys.
#        b. Register the provider in the TFC private registry.
#      Review them, then copy/paste to run. Everything after this is
#      destructive on the TFC org so the script stops short of executing.
#
# Re-run safe. Inspect everything in ./.tfc-setup/ when done.

set -euo pipefail

# ----- Configuration -----
ORG="understory"
# NOTE: TFC private registry requires the namespace to equal the TFC
# organization name (not the GitHub org). The provider source in tf configs
# must match: understory/hubspot (NOT understory-io/hubspot).
REGISTRY_NAMESPACE="understory"
REGISTRY_NAME="hubspot"
GH_REPO="understory-io/terraform-provider-hubspot"
KEY_NAME="Understory Terraform Providers"
KEY_EMAIL="kasper@understory.io"
TFC_HOST="app.terraform.io"
OUT_DIR=".tfc-setup"

cd "$(dirname "$0")/.."

# ----- Prereqs -----
need() {
  command -v "$1" >/dev/null || { echo "missing: $1"; exit 1; }
}
need gpg
need gh
need jq
need curl

TFC_TOKEN="$(jq -r '.credentials["app.terraform.io"].token // ""' "$HOME/.terraform.d/credentials.tfrc.json" 2>/dev/null || true)"
if [ -z "$TFC_TOKEN" ]; then
  echo "no TFC token found. run 'terraform login' first."
  exit 1
fi

mkdir -p "$OUT_DIR"
chmod 700 "$OUT_DIR"

# ----- 1. GPG keypair -----
EXISTING_FP="$(gpg --list-secret-keys --with-colons "$KEY_EMAIL" 2>/dev/null | awk -F: '/^fpr:/{print $10; exit}')"

if [ -n "$EXISTING_FP" ]; then
  echo "reusing existing GPG key for $KEY_EMAIL: $EXISTING_FP"
  FINGERPRINT="$EXISTING_FP"
else
  echo "generating new GPG keypair for $KEY_EMAIL ..."
  gpg --batch --gen-key <<EOF
Key-Type: RSA
Key-Length: 4096
Subkey-Type: RSA
Subkey-Length: 4096
Name-Real: $KEY_NAME
Name-Email: $KEY_EMAIL
Expire-Date: 0
%no-protection
%commit
EOF
  FINGERPRINT="$(gpg --list-secret-keys --with-colons "$KEY_EMAIL" | awk -F: '/^fpr:/{print $10; exit}')"
  echo "generated fingerprint: $FINGERPRINT"
fi

# ----- 2. Export key material -----
gpg --armor --export "$FINGERPRINT"             > "$OUT_DIR/public.asc"
gpg --armor --export-secret-keys "$FINGERPRINT" > "$OUT_DIR/private.asc"
echo "$FINGERPRINT"                              > "$OUT_DIR/fingerprint.txt"
chmod 600 "$OUT_DIR/private.asc"

echo "exported to $OUT_DIR/"

# ----- 3. GitHub secret -----
echo
echo "==> setting GitHub secrets on $GH_REPO ..."
gh secret set GPG_PRIVATE_KEY -R "$GH_REPO" < "$OUT_DIR/private.asc"
printf '' | gh secret set GPG_PASSPHRASE -R "$GH_REPO"
printf '%s' "$TFC_TOKEN" | gh secret set TFC_TOKEN -R "$GH_REPO"
echo "GitHub secrets set: GPG_PRIVATE_KEY, GPG_PASSPHRASE, TFC_TOKEN"

# ----- 4. Print TFC API commands (do NOT auto-execute) -----
PUBLIC_KEY_JSON_PAYLOAD="$OUT_DIR/tfc-gpg-key.json"
jq -n --arg ns "$REGISTRY_NAMESPACE" --arg armor "$(cat "$OUT_DIR/public.asc")" '{
  data: {
    type: "gpg-keys",
    attributes: {
      namespace: $ns,
      "ascii-armor": $armor
    }
  }
}' > "$PUBLIC_KEY_JSON_PAYLOAD"

REGISTER_PROVIDER_PAYLOAD="$OUT_DIR/tfc-provider.json"
jq -n --arg name "$REGISTRY_NAME" --arg ns "$REGISTRY_NAMESPACE" '{
  data: {
    type: "registry-providers",
    attributes: {
      name: $name,
      namespace: $ns,
      "registry-name": "private"
    }
  }
}' > "$REGISTER_PROVIDER_PAYLOAD"

cat <<EOF

==> Next, review and run the TFC API calls below.

# 1) Upload the GPG public key to the org's private registry.
curl -sS \\
  --header "Authorization: Bearer \$TFC_TOKEN" \\
  --header "Content-Type: application/vnd.api+json" \\
  --request POST \\
  --data @$PUBLIC_KEY_JSON_PAYLOAD \\
  "https://$TFC_HOST/api/registry/private/v2/gpg-keys" | jq .

# 2) Register the private provider entry (creates the registry slot, no
#    versions yet). Idempotent — returns 422 if it already exists.
curl -sS \\
  --header "Authorization: Bearer \$TFC_TOKEN" \\
  --header "Content-Type: application/vnd.api+json" \\
  --request POST \\
  --data @$REGISTER_PROVIDER_PAYLOAD \\
  "https://$TFC_HOST/api/v2/organizations/$ORG/registry-providers" | jq .

# Convenience: export the token from your terraform creds file.
export TFC_TOKEN="\$(jq -r '.credentials["app.terraform.io"].token' ~/.terraform.d/credentials.tfrc.json)"

EOF

echo "fingerprint: $FINGERPRINT"
echo "public key:  $OUT_DIR/public.asc"
echo "done. run the curl commands above to finish bootstrapping the TFC registry."
