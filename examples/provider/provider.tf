terraform {
  required_providers {
    hubspot = {
      source = "understory-io/hubspot"
    }
  }
}

# Reads HUBSPOT_ACCESS_TOKEN from the environment.
provider "hubspot" {}
