package provider

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	validatePort = &portValidator{}
	validateIP   = &ipValidator{}
)

type portValidator struct{}

// Description describes the validation in plain text formatting.
//
// This information may be automatically added to schema plain text
// descriptions by external tooling.
func (v *portValidator) Description(context.Context) string {
	return "A valid port in the range of 1-65535"
}

// MarkdownDescription describes the validation in Markdown formatting.
//
// This information may be automatically added to schema Markdown
// descriptions by external tooling.
func (v *portValidator) MarkdownDescription(context.Context) string {
	return "A valid port in the range of 1-65535"
}

// Validate performs the validation.
func (v *portValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	var i types.Int64
	diags := tfsdk.ValueAs(ctx, req.AttributeConfig, &i)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if i.IsUnknown() || i.IsNull() {
		return
	}

	if i.Value < 1 || i.Value > 65535 {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.AttributePath,
			v.Description(ctx),
			fmt.Sprintf("%d", i.Value),
		))

		return
	}
}

type ipValidator struct{}

func (v *ipValidator) Description(context.Context) string {
	return "A valid IPv4 address in dotted-quad notation"
}

func (v *ipValidator) MarkdownDescription(context.Context) string {
	return "A valid IPv4 address in dotted-quad notation"
}

func (v *ipValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	var ip types.String
	diags := tfsdk.ValueAs(ctx, req.AttributeConfig, &ip)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if ip.IsUnknown() || ip.IsNull() {
		return
	}

	if net.ParseIP(ip.Value) == nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.AttributePath,
			v.Description(ctx),
			ip.Value,
		))

		return
	}
}
