package hubspot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// Property models a HubSpot CRM property.
//
// See https://developers.hubspot.com/docs/api/crm/properties for the full
// schema. Only the fields the provider exposes are modelled here; additional
// fields returned by the API are ignored.
type Property struct {
	Name        string           `json:"name"`
	Label       string           `json:"label,omitempty"`
	Type        string           `json:"type,omitempty"`
	FieldType   string           `json:"fieldType,omitempty"`
	GroupName   string           `json:"groupName,omitempty"`
	Description string           `json:"description,omitempty"`
	Options     []PropertyOption `json:"options,omitempty"`
}

// PropertyOption is one option for an enumeration property.
type PropertyOption struct {
	Label        string `json:"label"`
	Value        string `json:"value"`
	Description  string `json:"description,omitempty"`
	DisplayOrder int64  `json:"displayOrder,omitempty"`
	Hidden       bool   `json:"hidden,omitempty"`
}

func propertyPath(objectType, name string) string {
	return fmt.Sprintf("/crm/v3/properties/%s/%s", url.PathEscape(objectType), url.PathEscape(name))
}

// CreateProperty creates a property on the given object type.
func (c *Client) CreateProperty(ctx context.Context, objectType string, p Property) (*Property, error) {
	out := &Property{}
	path := fmt.Sprintf("/crm/v3/properties/%s", url.PathEscape(objectType))
	if err := c.do(ctx, http.MethodPost, path, p, out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetProperty fetches a single property by name.
func (c *Client) GetProperty(ctx context.Context, objectType, name string) (*Property, error) {
	out := &Property{}
	if err := c.do(ctx, http.MethodGet, propertyPath(objectType, name), nil, out); err != nil {
		return nil, err
	}
	return out, nil
}

// UpdateProperty applies a partial update.
func (c *Client) UpdateProperty(ctx context.Context, objectType, name string, p Property) (*Property, error) {
	out := &Property{}
	if err := c.do(ctx, http.MethodPatch, propertyPath(objectType, name), p, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteProperty archives the property in HubSpot. The provider only calls
// this when the user has opted in via prune = true; otherwise the property is
// left in place.
func (c *Client) DeleteProperty(ctx context.Context, objectType, name string) error {
	return c.do(ctx, http.MethodDelete, propertyPath(objectType, name), nil, nil)
}
