package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/understory-io/terraform-provider-hubspot/internal/hubspot"
)

// fakeHubSpot is an in-memory HubSpot CRM properties API used by unit tests.
// It supports a single object type at a time (the path prefix is parameterised).
type fakeHubSpot struct {
	mu         sync.Mutex
	properties map[string]hubspot.Property // key = "<objectType>/<name>"
	deleteCalls int
}

func newFakeHubSpot() *fakeHubSpot {
	return &fakeHubSpot{properties: map[string]hubspot.Property{}}
}

func (f *fakeHubSpot) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/crm/v3/properties/", func(w http.ResponseWriter, r *http.Request) {
		// Path is /crm/v3/properties/<objectType> or /crm/v3/properties/<objectType>/<name>
		rest := r.URL.Path[len("/crm/v3/properties/"):]
		// rest is either "<objectType>" or "<objectType>/<name>"
		var objectType, name string
		for i := 0; i < len(rest); i++ {
			if rest[i] == '/' {
				objectType = rest[:i]
				name = rest[i+1:]
				break
			}
		}
		if objectType == "" {
			objectType = rest
		}

		f.mu.Lock()
		defer f.mu.Unlock()

		switch r.Method {
		case http.MethodPost:
			var p hubspot.Property
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &p); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			key := objectType + "/" + p.Name
			if _, exists := f.properties[key]; exists {
				http.Error(w, "exists", http.StatusConflict)
				return
			}
			f.properties[key] = p
			writeJSON(w, http.StatusCreated, p)

		case http.MethodGet:
			key := objectType + "/" + name
			p, ok := f.properties[key]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			writeJSON(w, http.StatusOK, p)

		case http.MethodPatch:
			key := objectType + "/" + name
			existing, ok := f.properties[key]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			var patch hubspot.Property
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &patch); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if patch.Label != "" {
				existing.Label = patch.Label
			}
			if patch.FieldType != "" {
				existing.FieldType = patch.FieldType
			}
			if patch.GroupName != "" {
				existing.GroupName = patch.GroupName
			}
			if patch.Description != "" {
				existing.Description = patch.Description
			}
			if patch.Options != nil {
				existing.Options = patch.Options
			}
			if patch.NumberDisplayHint != "" {
				existing.NumberDisplayHint = patch.NumberDisplayHint
			}
			f.properties[key] = existing
			writeJSON(w, http.StatusOK, existing)

		case http.MethodDelete:
			key := objectType + "/" + name
			f.deleteCalls++
			delete(f.properties, key)
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func protoFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"hubspot": providerserver.NewProtocol6WithError(New("test")()),
	}
}

func providerConfig(srv *httptest.Server) string {
	return fmt.Sprintf(`
provider "hubspot" {
  access_token = "test-token"
  base_url     = %q
}
`, srv.URL)
}

func TestAccPropertyResource_CreateReadUpdateDelete_NoPrune(t *testing.T) {
	t.Parallel()

	fake := newFakeHubSpot()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv) + `
resource "hubspot_property" "fav_color" {
  name        = "fav_color"
  object_type = "contacts"
  type        = "enumeration"
  label       = "Favourite colour"
  group_name  = "contactinformation"
  options = [
    { label = "Blue", value = "blue", display_order = 1 },
    { label = "Red",  value = "red",  display_order = 2 },
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "id", "contacts/fav_color"),
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "field_type", "select"),
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "prune", "false"),
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "options.#", "2"),
				),
			},
			{
				Config: providerConfig(srv) + `
resource "hubspot_property" "fav_color" {
  name        = "fav_color"
  object_type = "contacts"
  type        = "enumeration"
  label       = "Favourite color"
  group_name  = "contactinformation"
  description = "Updated"
  options = [
    { label = "Blue", value = "blue", display_order = 1 },
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "label", "Favourite color"),
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "description", "Updated"),
					resource.TestCheckResourceAttr("hubspot_property.fav_color", "options.#", "1"),
				),
			},
		},
	})

	if fake.deleteCalls != 0 {
		t.Errorf("expected zero DELETE calls (prune=false), got %d", fake.deleteCalls)
	}
	if _, exists := fake.properties["contacts/fav_color"]; !exists {
		t.Errorf("property should remain in HubSpot after destroy when prune=false")
	}
}

func TestAccPropertyResource_DestroyPrunes(t *testing.T) {
	t.Parallel()

	fake := newFakeHubSpot()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv) + `
resource "hubspot_property" "p" {
  name        = "delete_me"
  object_type = "contacts"
  type        = "string"
  label       = "Delete me"
  group_name  = "contactinformation"
  prune       = true
}`,
			},
		},
	})

	if fake.deleteCalls != 1 {
		t.Errorf("expected 1 DELETE call (prune=true), got %d", fake.deleteCalls)
	}
	if _, exists := fake.properties["contacts/delete_me"]; exists {
		t.Errorf("property should be gone from HubSpot when prune=true")
	}
}

func TestAccPropertyResource_ImportState(t *testing.T) {
	t.Parallel()

	fake := newFakeHubSpot()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	cfg := providerConfig(srv) + `
resource "hubspot_property" "imported" {
  name        = "imported"
  object_type = "contacts"
  type        = "string"
  label       = "Imported"
  group_name  = "contactinformation"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:            cfg,
				ResourceName:      "hubspot_property.imported",
				ImportState:       true,
				ImportStateId:     "contacts/imported",
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					// prune is provider-side lifecycle metadata; HubSpot doesn't echo it.
					"prune",
				},
			},
		},
	})
}

func TestAccPropertyResource_DriftDetectionDeletedExternally(t *testing.T) {
	t.Parallel()

	fake := newFakeHubSpot()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	cfg := providerConfig(srv) + `
resource "hubspot_property" "p" {
  name        = "drift_me"
  object_type = "contacts"
  type        = "string"
  label       = "Drift"
  group_name  = "contactinformation"
}`

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.TestCheckResourceAttr("hubspot_property.p", "id", "contacts/drift_me"),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.properties, "contacts/drift_me")
					fake.mu.Unlock()
				},
				Config:             cfg,
				ExpectNonEmptyPlan: true, // a plan to recreate is expected after external delete
				PlanOnly:           true,
			},
		},
	})
}

func TestAccPropertyResource_NumberDisplayHint(t *testing.T) {
	t.Parallel()

	fake := newFakeHubSpot()
	srv := httptest.NewServer(fake.handler())
	t.Cleanup(srv.Close)

	cfg := func(hint string) string {
		line := ""
		if hint != "" {
			line = fmt.Sprintf("  number_display_hint = %q\n", hint)
		}
		return providerConfig(srv) + `
resource "hubspot_property" "share" {
  name        = "share"
  object_type = "deals"
  type        = "number"
  label       = "Share"
  group_name  = "dealinformation"
` + line + `}`
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{
				Config: cfg(""),
				Check:  resource.TestCheckNoResourceAttr("hubspot_property.share", "number_display_hint"),
			},
			{
				Config: cfg("probability"),
				Check:  resource.TestCheckResourceAttr("hubspot_property.share", "number_display_hint", "probability"),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					p := fake.properties["deals/share"]
					p.NumberDisplayHint = "unformatted"
					fake.properties["deals/share"] = p
					fake.mu.Unlock()
				},
				Config:             cfg("probability"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: cfg("probability"),
				Check:  resource.TestCheckResourceAttr("hubspot_property.share", "number_display_hint", "probability"),
			},
		},
	})

	if got := fake.properties["deals/share"].NumberDisplayHint; got != "probability" {
		t.Errorf("HubSpot numberDisplayHint = %q, want probability", got)
	}
}

func TestAccPropertyResource_NumberDisplayHintRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(newFakeHubSpot().handler())
	t.Cleanup(srv.Close)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoFactories(),
		Steps: []resource.TestStep{
			{
				Config: providerConfig(srv) + `
resource "hubspot_property" "share" {
  name                = "share"
  object_type         = "deals"
  type                = "number"
  label               = "Share"
  group_name          = "dealinformation"
  number_display_hint = "percent"
}`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
		},
	})
}
