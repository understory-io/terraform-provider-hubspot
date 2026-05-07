# terraform-provider-hubspot

A small, private Terraform provider for managing HubSpot CRM properties via the
HubSpot CRM v3 API.

## Status

- One resource: `hubspot_property` (CRM properties on any object type).
- Authentication: HubSpot **private-app access token**.
- Used privately within Understory; not published to the public Terraform
  Registry.

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

## Releasing

Releases are managed by [release-please](https://github.com/googleapis/release-please).
Conventional Commits on `main` (`feat:`, `fix:`, `feat!:` …) accumulate into a
release PR; merging that PR cuts a tag.

The release workflow currently only manages versions and changelogs; building
distributable binaries (e.g. via GoReleaser) and signing them for a registry
isn't wired up yet — add that when/if we publish.
