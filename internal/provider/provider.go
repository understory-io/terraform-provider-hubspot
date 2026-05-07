// Package provider implements the terraform-provider-hubspot Terraform
// provider. The provider currently exposes a single resource,
// hubspot_property, for managing HubSpot CRM properties.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/understory-io/terraform-provider-hubspot/internal/hubspot"
)

// New returns a provider factory bound to the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &hubspotProvider{version: version} }
}

type hubspotProvider struct {
	version string
}

type providerModel struct {
	AccessToken types.String `tfsdk:"access_token"`
	BaseURL     types.String `tfsdk:"base_url"`
}

func (p *hubspotProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "hubspot"
	resp.Version = p.version
}

func (p *hubspotProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages resources in HubSpot via the CRM v3 API.",
		Attributes: map[string]schema.Attribute{
			"access_token": schema.StringAttribute{
				MarkdownDescription: "HubSpot private-app access token. Falls back to the `HUBSPOT_ACCESS_TOKEN` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Override the HubSpot API base URL. Defaults to `https://api.hubapi.com`. Falls back to the `HUBSPOT_BASE_URL` environment variable. Mainly useful in tests.",
				Optional:            true,
			},
		},
	}
}

func (p *hubspotProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := data.AccessToken.ValueString()
	if token == "" {
		token = os.Getenv("HUBSPOT_ACCESS_TOKEN")
	}
	if token == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_token"),
			"Missing HubSpot access token",
			"Provide `access_token` in the provider block or set the HUBSPOT_ACCESS_TOKEN environment variable.",
		)
		return
	}

	opts := []hubspot.Option{hubspot.WithUserAgent("terraform-provider-hubspot/" + p.version)}
	baseURL := data.BaseURL.ValueString()
	if baseURL == "" {
		baseURL = os.Getenv("HUBSPOT_BASE_URL")
	}
	if baseURL != "" {
		opts = append(opts, hubspot.WithBaseURL(baseURL))
	}

	client := hubspot.New(token, opts...)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *hubspotProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewPropertyResource,
	}
}

func (p *hubspotProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
