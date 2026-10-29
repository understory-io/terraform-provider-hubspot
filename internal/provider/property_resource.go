package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"github.com/understory-io/terraform-provider-hubspot/internal/hubspot"
)

// NewPropertyResource is the resource factory used by the provider.
func NewPropertyResource() resource.Resource { return &propertyResource{} }

type propertyResource struct {
	client *hubspot.Client
}

type propertyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	ObjectType  types.String `tfsdk:"object_type"`
	Label       types.String `tfsdk:"label"`
	Type        types.String `tfsdk:"type"`
	FieldType   types.String `tfsdk:"field_type"`
	GroupName   types.String `tfsdk:"group_name"`
	Description types.String `tfsdk:"description"`
	Options     types.List   `tfsdk:"options"`
	Prune       types.Bool   `tfsdk:"prune"`

	NumberDisplayHint types.String `tfsdk:"number_display_hint"`
}

func optionObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"label":         types.StringType,
		"value":         types.StringType,
		"description":   types.StringType,
		"display_order": types.Int64Type,
		"hidden":        types.BoolType,
	}}
}

func (r *propertyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_property"
}

func (r *propertyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a HubSpot CRM property on a given object type. " +
			"By default, removing this resource only removes it from Terraform state — the property stays in HubSpot. " +
			"Set `prune = true` to actually archive the property in HubSpot when the resource is destroyed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite identifier `<object_type>/<name>`.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Internal property name. Immutable.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"object_type": schema.StringAttribute{
				MarkdownDescription: "HubSpot CRM object type, e.g. `contacts`, `companies`, `deals`, or a numeric custom object id like `2-12345`. Immutable.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Data type. One of: `string`, `number`, `bool`, `enumeration`, `date`, `datetime`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("string", "number", "bool", "enumeration", "date", "datetime"),
				},
			},
			"label": schema.StringAttribute{
				MarkdownDescription: "Human-readable label shown in the HubSpot UI.",
				Required:            true,
			},
			"group_name": schema.StringAttribute{
				MarkdownDescription: "Property group this property belongs to.",
				Required:            true,
			},
			"field_type": schema.StringAttribute{
				MarkdownDescription: "HubSpot field type (controls the UI widget). Common values: `text`, `textarea`, `select`, `radio`, `checkbox`, `booleancheckbox`, `number`, `date`. If omitted, a sensible default is derived from `type`.",
				Optional:            true,
				Computed:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Free-form description.",
				Optional:            true,
			},
			"options": schema.ListNestedAttribute{
				MarkdownDescription: "Options for `enumeration` properties. Ignored for other types.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label":         schema.StringAttribute{Required: true},
						"value":         schema.StringAttribute{Required: true},
						"description":   schema.StringAttribute{Optional: true, Computed: true},
						"display_order": schema.Int64Attribute{Optional: true, Computed: true},
						"hidden":        schema.BoolAttribute{Optional: true, Computed: true},
					},
				},
			},
			"number_display_hint": schema.StringAttribute{
				MarkdownDescription: "How HubSpot formats a `number` property in its UI. One of: `unformatted`, `formatted`, `currency`, `percentage`, `duration`, `probability` (a value from 0 to 1 shown as a percentage, so 1 is 100%). If omitted, the property keeps whatever HubSpot has and nothing is sent.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Validators: []validator.String{
					stringvalidator.OneOf("unformatted", "formatted", "currency", "percentage", "duration", "probability"),
				},
			},
			"prune": schema.BoolAttribute{
				MarkdownDescription: "If true, destroying this resource archives the property in HubSpot. If false (default), the property is left in HubSpot when the resource is removed.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *propertyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*hubspot.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("Expected *hubspot.Client, got %T. This is a bug in the provider.", req.ProviderData),
		)
		return
	}
	r.client = c
}

func (r *propertyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan propertyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.FieldType.IsNull() || plan.FieldType.IsUnknown() {
		plan.FieldType = types.StringValue(defaultFieldType(plan.Type.ValueString()))
	}

	apiProp, diags := planToProperty(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.CreateProperty(ctx, plan.ObjectType.ValueString(), apiProp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create HubSpot property", err.Error())
		return
	}

	state := plan
	applyAPIToModel(ctx, created, &state, &resp.Diagnostics)
	state.ID = types.StringValue(plan.ObjectType.ValueString() + "/" + created.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *propertyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state propertyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	got, err := r.client.GetProperty(ctx, state.ObjectType.ValueString(), state.Name.ValueString())
	if err != nil {
		if hubspot.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read HubSpot property", err.Error())
		return
	}

	applyAPIToModel(ctx, got, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *propertyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan propertyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.FieldType.IsNull() || plan.FieldType.IsUnknown() {
		plan.FieldType = types.StringValue(defaultFieldType(plan.Type.ValueString()))
	}

	apiProp, diags := planToProperty(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// HubSpot rejects `name` in PATCH bodies — clear it.
	apiProp.Name = ""

	updated, err := r.client.UpdateProperty(ctx, plan.ObjectType.ValueString(), plan.Name.ValueString(), apiProp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update HubSpot property", err.Error())
		return
	}

	state := plan
	applyAPIToModel(ctx, updated, &state, &resp.Diagnostics)
	state.ID = types.StringValue(plan.ObjectType.ValueString() + "/" + plan.Name.ValueString())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *propertyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state propertyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !state.Prune.ValueBool() {
		// Default: leave the HubSpot property in place. Removing from state
		// is implicit on a successful Delete response.
		return
	}

	if err := r.client.DeleteProperty(ctx, state.ObjectType.ValueString(), state.Name.ValueString()); err != nil {
		if hubspot.IsNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete HubSpot property", err.Error())
	}
}

func (r *propertyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected `<object_type>/<name>`, e.g. `contacts/fav_color`.",
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("object_type"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// --- helpers ---

func defaultFieldType(t string) string {
	switch t {
	case "string":
		return "text"
	case "number":
		return "number"
	case "bool":
		return "booleancheckbox"
	case "enumeration":
		return "select"
	case "date":
		return "date"
	case "datetime":
		return "date"
	default:
		return "text"
	}
}

type optionModel struct {
	Label        types.String `tfsdk:"label"`
	Value        types.String `tfsdk:"value"`
	Description  types.String `tfsdk:"description"`
	DisplayOrder types.Int64  `tfsdk:"display_order"`
	Hidden       types.Bool   `tfsdk:"hidden"`
}

func planToProperty(ctx context.Context, m propertyResourceModel) (hubspot.Property, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := hubspot.Property{
		Name:        m.Name.ValueString(),
		Label:       m.Label.ValueString(),
		Type:        m.Type.ValueString(),
		FieldType:   m.FieldType.ValueString(),
		GroupName:   m.GroupName.ValueString(),
		Description: m.Description.ValueString(),
	}
	if !m.NumberDisplayHint.IsNull() && !m.NumberDisplayHint.IsUnknown() {
		p.NumberDisplayHint = m.NumberDisplayHint.ValueString()
	}

	if !m.Options.IsNull() && !m.Options.IsUnknown() {
		var opts []optionModel
		diags.Append(m.Options.ElementsAs(ctx, &opts, false)...)
		if diags.HasError() {
			return p, diags
		}
		for _, o := range opts {
			p.Options = append(p.Options, hubspot.PropertyOption{
				Label:        o.Label.ValueString(),
				Value:        o.Value.ValueString(),
				Description:  o.Description.ValueString(),
				DisplayOrder: o.DisplayOrder.ValueInt64(),
				Hidden:       o.Hidden.ValueBool(),
			})
		}
	}
	return p, diags
}

func applyAPIToModel(_ context.Context, p *hubspot.Property, m *propertyResourceModel, diags *diag.Diagnostics) {
	m.Name = types.StringValue(p.Name)
	m.Label = types.StringValue(p.Label)
	m.Type = types.StringValue(p.Type)
	m.FieldType = types.StringValue(p.FieldType)
	m.GroupName = types.StringValue(p.GroupName)
	m.NumberDisplayHint = nullableString(p.NumberDisplayHint)
	if p.Description == "" {
		// Preserve nullability so we don't flip null<->"" between plan & state.
		if m.Description.IsUnknown() {
			m.Description = types.StringNull()
		}
	} else {
		m.Description = types.StringValue(p.Description)
	}

	if len(p.Options) == 0 {
		// Keep null when the user didn't set options. If they set [] we'd see
		// it below; HubSpot returns nil for non-enum types.
		if m.Options.IsUnknown() {
			m.Options = types.ListNull(optionObjectType())
		}
		return
	}

	objs := make([]attr.Value, 0, len(p.Options))
	for _, o := range p.Options {
		obj, d := types.ObjectValue(optionObjectType().AttrTypes, map[string]attr.Value{
			"label":         types.StringValue(o.Label),
			"value":         types.StringValue(o.Value),
			"description":   nullableString(o.Description),
			"display_order": types.Int64Value(o.DisplayOrder),
			"hidden":        types.BoolValue(o.Hidden),
		})
		diags.Append(d...)
		objs = append(objs, obj)
	}
	list, d := types.ListValue(optionObjectType(), objs)
	diags.Append(d...)
	m.Options = list
}

func nullableString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
