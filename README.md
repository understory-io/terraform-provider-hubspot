# terraform-provider-hubspot

A small, private Terraform provider for managing HubSpot CRM properties via
the HubSpot CRM v3 API. Used privately within Understory; published to the
**TFC private registry** as `understory/hubspot`, not to the public Terraform
Registry.

## Status

- One resource: `hubspot_property` (CRM properties on any object type).
- Authentication: HubSpot **private-app access token**, scoped to the
  developer-project-defined private app under [`hubspot-app/`](./hubspot-app).
- Distribution: signed releases pushed to TFC private registry on every
  `v*` tag. See [Releasing](#releasing).

## Consuming the provider

```hcl
terraform {
  required_providers {
    hubspot = {
      source  = "app.terraform.io/understory/hubspot"
      version = "~> 1.0"
    }
  }
}

provider "hubspot" {
  access_token = var.hubspot_access_token
}
```

The host (`app.terraform.io`) routes Terraform to TFC's private registry
rather than the public one. Workspaces in the `understory` TFC org pick up
the provider automatically. For local-only consumers, run `terraform login`
first so the CLI has a registry token.

## Authentication

The private app the provider authenticates as is defined as code under
[`hubspot-app/`](./hubspot-app) — a HubSpot Developer Project (platform
version `2026.03`) created with `hs project create`. The required scopes for
managing CRM property schemas live in
[`hubspot-app/src/app/app-hsmeta.json`](./hubspot-app/src/app/app-hsmeta.json)
so they're reviewable in code rather than clicked into the UI.

Bootstrap once per account:

```sh
mise run hs:install   # installs the HubSpot CLI locally
mise run hs:auth      # one-time CLI auth into the target HubSpot account
mise run hs:upload    # uploads & installs the private app
```

> **Heads up:** there is an unrelated package called `hs` on npm. `npx hs` from
> outside `hubspot-app/` will resolve to that imposter, not HubSpot's CLI.
> Always go through `mise run hs:*` (or `cd hubspot-app && npm run …`), which
> use the locally-installed `@hubspot/cli`.

After upload, copy the access token from the app's settings in HubSpot. Then:

```hcl
provider "hubspot" {
  access_token = var.hubspot_access_token
}
```

Or set `HUBSPOT_ACCESS_TOKEN` in the environment and leave the block empty.

If the provider needs a new scope, edit
`hubspot-app/src/app/app-hsmeta.json`, run `mise run hs:upload`, and rotate
the token.

## `hubspot_property`

```hcl
resource "hubspot_property" "loyalty_id" {
  name        = "loyalty_id"
  object_type = "contacts"
  type        = "string"
  label       = "Loyalty ID"
  group_name  = "contactinformation"
}
```

| Argument      | Required | Notes                                                                                  |
| ------------- | -------- | -------------------------------------------------------------------------------------- |
| `name`        | yes      | Internal name. Immutable — changing it triggers replacement.                           |
| `object_type` | yes      | `contacts`, `companies`, `deals`, `tickets`, or a custom object id (`2-12345`).        |
| `type`        | yes      | `string`, `number`, `bool`, `enumeration`, `date`, `datetime`.                         |
| `label`       | yes      | Shown in the HubSpot UI.                                                               |
| `group_name`  | yes      | Property group on the object.                                                          |
| `field_type`  | no       | UI widget. Defaults derived from `type` (e.g. `enumeration` → `select`).               |
| `description` | no       | Free text.                                                                             |
| `options`     | no       | Required when `type = "enumeration"`. List of `{ label, value, display_order, … }`.    |
| `prune`       | no       | Default `false`. See lifecycle below.                                                  |

### Lifecycle and the `prune` flag

By default this provider is **non-destructive**. When you remove a
`hubspot_property` resource (or `terraform destroy` the workspace), the
property is removed from Terraform state but **left in place in HubSpot**.

To opt into actual deletion, set `prune = true` on the resource. With
`prune = true`, destroying the resource issues a `DELETE` against
`/crm/v3/properties/{objectType}/{name}` and the property is archived in
HubSpot.

This is intentional: HubSpot properties often carry data, and the data is
lost when the property is archived. We default to safe.

### Importing

```sh
terraform import hubspot_property.loyalty_id contacts/loyalty_id
```

The import id is `<object_type>/<name>`.

## Development

This project uses [mise](https://mise.jdx.dev/) for tool management and task
running. With mise installed:

```sh
mise install      # installs Go, golangci-lint, terraform
mise run ci       # the full CI suite (tests, vet, lint, vuln, coverage, build)
mise run test     # unit tests with the in-memory HubSpot fake
mise run test:acc # acceptance tests against the real HubSpot API
```

### Testing strategy

There are three layers, all in `go test`:

1. **Client unit tests** (`internal/hubspot/client_test.go`) — exercise the
   HTTP client against `httptest.Server`, covering auth headers, paths, and
   error mapping.
2. **Resource unit tests** (`internal/provider/property_resource_test.go`) —
   use the framework's `resource.UnitTest` with an in-memory HubSpot fake
   served by `httptest`. The provider is pointed at the fake via the
   `base_url` provider attribute. These cover full plan/apply/destroy cycles,
   the `prune = false` default, `prune = true`, drift detection, and import.
   They do not require any HubSpot credentials.
3. **Acceptance tests** (gated by `TF_ACC=1`) — same test files, but flip the
   gate and provide `HUBSPOT_ACCESS_TOKEN`/`HUBSPOT_BASE_URL` env vars to hit
   real HubSpot. Useful before cutting a release.

`mise run ci` runs everything except the acceptance tests.

### Local manual testing with a built binary

```sh
mise run build
# Add to ~/.terraformrc to point Terraform at the local binary:
#   provider_installation {
#     dev_overrides {
#       "understory/hubspot" = "/abs/path/to/terraform-provider-hubspot"
#     }
#     direct {}
#   }
```

This shortcuts the registry; useful when iterating on provider code before
cutting a tag.

## Releasing

The release pipeline runs entirely off `main` and tag pushes:

```
conventional commit on main
    │
    ▼
release-please opens "chore(main): release X.Y.Z" PR (or updates it)
    │  merge
    ▼
release-please commits the version bump + changelog, creates tag vX.Y.Z
    │  (tag push triggers next stage)
    ▼
goreleaser builds 13 platform zips, signs SHA256SUMS with the org GPG key,
uploads everything to the GitHub Release
    │
    ▼
scripts/tfc-upload-version.sh registers the version in the TFC private
registry, uploads SHASUMS + .sig + every platform zip
    │
    ▼
infrastructure-hubspot (and any other consumer) can `terraform init -upgrade`
to pick up the new version
```

Tag pushes from release-please's `GITHUB_TOKEN` *don't* trigger the goreleaser
job (GitHub's recursion guard). For automatic triggering, swap the
`googleapis/release-please-action` token for a PAT, or just push the tag
manually:

```sh
git push origin :refs/tags/vX.Y.Z   # in case the tag exists already
git push origin refs/tags/vX.Y.Z
```

### Workflows

| File                                  | Trigger                       | Purpose                                                        |
| ------------------------------------- | ----------------------------- | -------------------------------------------------------------- |
| `.github/workflows/ci.yml`            | push to main, PRs to main     | `mise run ci` — tests, vet, lint, vuln, coverage, build        |
| `.github/workflows/release.yml`       | push to main; push of `v*` tag| release-please job (PR/tag); goreleaser job (build + TFC push) |

The release workflow needs three repo secrets to function:

| Secret             | Source                                                    |
| ------------------ | --------------------------------------------------------- |
| `GPG_PRIVATE_KEY`  | ASCII-armored private key matching the TFC org's GPG key  |
| `GPG_PASSPHRASE`   | Empty when the key has no passphrase (CI is non-interactive) |
| `TFC_TOKEN`        | TFC user/team token with permission to publish providers  |

All three are set by `scripts/tfc-registry-bootstrap.sh` — see [Bootstrapping the TFC registry](#bootstrapping-the-tfc-registry).

### Bootstrapping the TFC registry

`scripts/tfc-registry-bootstrap.sh` is a one-shot setup. It generates a
4096-bit RSA GPG key (idempotent — reuses an existing one for the configured
email), exports the key material to `.tfc-setup/` (git-ignored), sets the
three GitHub secrets above, and prints two `curl` commands to:

1. upload the public key to the `understory` TFC org's GPG keys, and
2. register the `understory/hubspot` provider entry in the private registry.

Run the script, then paste the printed `curl` commands. After that the
release workflow will sign and upload on every tag with no further setup.

To rotate the GPG key, delete `.tfc-setup/` plus the key from the local
gpg keyring (`gpg --delete-secret-keys <fingerprint>`), then re-run the
script.

### scripts/tfc-upload-version.sh

The artifact-upload step in the release workflow. Reads goreleaser's `dist/`
directory after a build, then walks the TFC private registry API:

1. POST a `registry-provider-versions` resource (returns presigned upload
   URLs for SHASUMS and SHASUMS.sig).
2. PUT the SHASUMS + signature.
3. For each platform zip: POST `registry-provider-version-platforms`
   (returns a presigned URL), then PUT the zip.

Re-runs are safe — it returns 422 from TFC when a version or platform already
exists and skips that step.
