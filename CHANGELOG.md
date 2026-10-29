# Changelog

## [2.1.0](https://github.com/understory-io/terraform-provider-hubspot/compare/v2.0.0...v2.1.0) (2026-10-29)


### Features

* number_display_hint on hubspot_property ([#7](https://github.com/understory-io/terraform-provider-hubspot/issues/7)) ([90348b9](https://github.com/understory-io/terraform-provider-hubspot/commit/90348b971ba6a1c3dd87eace1f99cce829971da4))


### Bug Fixes

* **ci:** build and upload the release that release-please creates. Its ([90348b9](https://github.com/understory-io/terraform-provider-hubspot/commit/90348b971ba6a1c3dd87eace1f99cce829971da4))

## [2.0.0](https://github.com/understory-io/terraform-provider-hubspot/compare/v1.0.0...v2.0.0) (2026-08-12)


### ⚠ BREAKING CHANGES

* any consumer using `source = "understory-io/hubspot"` must update to `source = "understory/hubspot"`.

### Features

* upload signed releases to TFC private registry ([541b0bb](https://github.com/understory-io/terraform-provider-hubspot/commit/541b0bb180fbd2f5680ba7028c4336bd3f164947))


### Bug Fixes

* change provider source from understory-io/hubspot to understory/hubspot ([988426d](https://github.com/understory-io/terraform-provider-hubspot/commit/988426dc9ef96fcf0b754d860344718934a3b049))
* **deps:** clear govulncheck findings in transitive modules ([8004b85](https://github.com/understory-io/terraform-provider-hubspot/commit/8004b853186c7b6605526166e5b94f0f467922d2))

## 1.0.0 (2026-05-07)


### Features

* initial provider for managing HubSpot CRM properties ([e103afd](https://github.com/understory-io/terraform-provider-hubspot/commit/e103afd6ba23315902f4a1e97310cdd246043e65))


### Bug Fixes

* drop hs:validate from CI chain ([29b15bb](https://github.com/understory-io/terraform-provider-hubspot/commit/29b15bb0a27eb04121c9960b8e80a4d3ff4ad97e))
