package provider

import (
	"context"
	"fmt"
	"net"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
func (v *portValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	port := req.ConfigValue.ValueInt64()

	if port < 1 || port > 65535 {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			v.Description(ctx),
			fmt.Sprintf("%d", port),
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

func (v *ipValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	ip := req.ConfigValue.ValueString()

	if net.ParseIP(ip) == nil {
		resp.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			req.Path,
			v.Description(ctx),
			ip,
		))

		return
	}
}
