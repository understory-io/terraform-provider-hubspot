# A simple text property on contacts. Removing this resource will NOT delete
# the property in HubSpot — it just leaves state.
resource "hubspot_property" "loyalty_id" {
  name        = "loyalty_id"
  object_type = "contacts"
  type        = "string"
  label       = "Loyalty ID"
  group_name  = "contactinformation"
  description = "Customer loyalty programme identifier"
}

# An enumeration property. Set prune = true to opt into actual deletion in
# HubSpot when the resource is destroyed.
resource "hubspot_property" "subscription_tier" {
  name        = "subscription_tier"
  object_type = "contacts"
  type        = "enumeration"
  label       = "Subscription tier"
  group_name  = "contactinformation"
  prune       = true

  options = [
    { label = "Free",       value = "free",       display_order = 1 },
    { label = "Pro",        value = "pro",        display_order = 2 },
    { label = "Enterprise", value = "enterprise", display_order = 3 },
  ]
}
