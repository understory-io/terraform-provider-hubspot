package hubspot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-token", WithBaseURL(srv.URL), WithUserAgent("ua/1")), srv
}

func TestCreateProperty_SendsAuthAndBody(t *testing.T) {
	t.Parallel()

	var gotAuth, gotCT, gotUA, gotMethod, gotPath string
	var gotBody Property

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotCT = r.Header.Get("Content-Type")
		gotUA = r.Header.Get("User-Agent")
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body) // echo
	})

	in := Property{
		Name:      "fav_color",
		Label:     "Favourite colour",
		Type:      "enumeration",
		FieldType: "select",
		GroupName: "contactinformation",
		Options: []PropertyOption{
			{Label: "Blue", Value: "blue", DisplayOrder: 1},
		},
	}
	got, err := c.CreateProperty(context.Background(), "contacts", in)
	if err != nil {
		t.Fatalf("CreateProperty: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/crm/v3/properties/contacts" {
		t.Errorf("path = %s", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotCT != "application/json" {
		t.Errorf("content-type = %q", gotCT)
	}
	if gotUA != "ua/1" {
		t.Errorf("ua = %q", gotUA)
	}
	if gotBody.Name != in.Name || gotBody.Type != in.Type {
		t.Errorf("body roundtrip mismatch: %+v", gotBody)
	}
	if got.Name != in.Name {
		t.Errorf("response name = %q", got.Name)
	}
}

func TestGetProperty_NotFound(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":"error","message":"missing"}`, http.StatusNotFound)
	})

	_, err := c.GetProperty(context.Background(), "contacts", "nope")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Errorf("IsNotFound(err) = false, want true; err = %v", err)
	}
}

func TestUpdateProperty_PatchPath(t *testing.T) {
	t.Parallel()

	var gotMethod, gotPath string
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"x"}`))
	})

	if _, err := c.UpdateProperty(context.Background(), "contacts", "x", Property{Label: "X"}); err != nil {
		t.Fatalf("UpdateProperty: %v", err)
	}
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %s", gotMethod)
	}
	if gotPath != "/crm/v3/properties/contacts/x" {
		t.Errorf("path = %s", gotPath)
	}
}

func TestDeleteProperty_AcceptsNoBody(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	if err := c.DeleteProperty(context.Background(), "contacts", "x"); err != nil {
		t.Fatalf("DeleteProperty: %v", err)
	}
}

func TestAPIError_Message(t *testing.T) {
	t.Parallel()

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})

	_, err := c.GetProperty(context.Background(), "contacts", "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error missing status: %v", err)
	}
}

func TestPropertyPath_EscapesObjectType(t *testing.T) {
	t.Parallel()
	got := propertyPath("0-1", "weird name")
	want := "/crm/v3/properties/0-1/weird%20name"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
