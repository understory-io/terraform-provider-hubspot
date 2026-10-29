# terraform-provider-hubspot

Manages HubSpot CRM property definitions in Terraform, so a property's
label, type, options and display format are reviewed in a pull request
instead of clicked into HubSpot, where they drift. It covers one resource,
`hubspot_property`, on any object type, and never deletes a property unless
asked to (see [`prune`](#lifecycle-and-the-prune-flag)).

Published on the public [Terraform Registry](https://registry.terraform.io/providers/understory-io/hubspot)
as `understory-io/hubspot`.

## Consuming the provider

```hcl
terraform {
  required_providers {
    hubspot = {
      source  = "understory-io/hubspot"
      version = "~> 2.1"
    }
  }
}

provider "hubspot" {
  access_token = var.hubspot_access_token
}
```

Moving a workspace off the old private-registry address
(`app.terraform.io/understory/hubspot`) needs a one-off state rewrite before
its first plan. Otherwise Terraform plans to drop every property from state
under the old provider and recreate it under the new one:

```sh
terraform state replace-provider \
  app.terraform.io/understory/hubspot registry.terraform.io/understory-io/hubspot
```

## Authentication

The private app the provider authenticates as is defined as code under
[`hubspot-app/`](https://github.com/understory-io/terraform-provider-hubspot/tree/main/hubspot-app)
— a HubSpot Developer Project (platform version `2026.03`) created with
`hs project create`. The required scopes for managing CRM property schemas
live in
[`hubspot-app/src/app/app-hsmeta.json`](https://github.com/understory-io/terraform-provider-hubspot/blob/main/hubspot-app/src/app/app-hsmeta.json)
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
| `number_display_hint` | no | How a `number` shows in HubSpot: `unformatted`, `formatted`, `currency`, `percentage`, `duration`, or `probability` (0–1 shown as a percent). Unset leaves HubSpot's value alone. |
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
#       "understory-io/hubspot" = "/abs/path/to/terraform-provider-hubspot"
#     }
#     direct {}
#   }
```

This shortcuts the registry; useful when iterating on provider code before
cutting a tag.

## Releasing

Merge conventional commits to `main`. release-please keeps a
`chore(main): release X.Y.Z` PR open; merging it tags `vX.Y.Z`, and the same
workflow run builds the platform zips with goreleaser, signs `SHA256SUMS`
with the GPG key in `GPG_PRIVATE_KEY`/`GPG_PASSPHRASE`, and attaches them to
the GitHub release. The Terraform Registry picks up the signed release from
GitHub by itself; consumers then `terraform init -upgrade`.

The registry verifies the signature against the public key registered for
the `understory-io` namespace (key `B5B77F14A8B186B7`, shared with
`terraform-provider-metabase` and `terraform-provider-shiitake`). A release
signed with any other key is rejected there.
