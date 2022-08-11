package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

type stringDefaultModifier struct {
	Default string
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m stringDefaultModifier) Description(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to %s", m.Default)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m stringDefaultModifier) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m.Default)
}

// Modify runs the logic of the plan modifier.
// Access to the configuration, plan, and state is available in `req`, while
// `resp` contains fields for updating the planned value, triggering resource
// replacement, and returning diagnostics.
func (m stringDefaultModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	// types.String must be the attr.Value produced by the attr.Type in the schema for this attribute
	// for generic plan modifiers, use
	// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/tfsdk#ConvertValue
	// to convert into a known type.
	var str types.String
	diags := tfsdk.ValueAs(ctx, req.AttributePlan, &str)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !str.Null && !str.Unknown {
		return
	}

	resp.AttributePlan = types.String{Value: m.Default}
}

type intDefaultModifier struct {
	Default int64
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m intDefaultModifier) Description(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to %d", m.Default)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m intDefaultModifier) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%d`", m.Default)
}

// Modify runs the logic of the plan modifier.
// Access to the configuration, plan, and state is available in `req`, while
// `resp` contains fields for updating the planned value, triggering resource
// replacement, and returning diagnostics.
func (m intDefaultModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	// types.String must be the attr.Value produced by the attr.Type in the schema for this attribute
	// for generic plan modifiers, use
	// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/tfsdk#ConvertValue
	// to convert into a known type.
	var i types.Int64
	diags := tfsdk.ValueAs(ctx, req.AttributePlan, &i)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !i.Null && !i.Unknown {
		return
	}

	resp.AttributePlan = types.Int64{Value: m.Default}
}

type objectDefaultModifier struct {
	Default types.Object
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m objectDefaultModifier) Description(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to %s", m.Default.String())
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m objectDefaultModifier) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("If value is not configured, defaults to `%s`", m.Default.String())
}

// Modify runs the logic of the plan modifier.
// Access to the configuration, plan, and state is available in `req`, while
// `resp` contains fields for updating the planned value, triggering resource
// replacement, and returning diagnostics.
func (m objectDefaultModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	tflog.Trace(ctx, "maybe defaulting object")
	// types.String must be the attr.Value produced by the attr.Type in the schema for this attribute
	// for generic plan modifiers, use
	// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-framework/tfsdk#ConvertValue
	// to convert into a known type.
	var obj types.Object
	diags := tfsdk.ValueAs(ctx, req.AttributePlan, &obj)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !obj.Null {
		return
	}

	resp.AttributePlan = m.Default
}
