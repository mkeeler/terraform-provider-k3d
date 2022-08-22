package provider

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	client "github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ provider.DataSourceType = k3dNodesDataSourceType{}
var _ datasource.DataSource = k3dNodesDataSource{}

type k3dNodesDataSourceType struct{}

var portBindingType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"port":      types.Int64Type,
		"host_ip":   types.StringType,
		"host_port": types.Int64Type,
	},
}

func (t k3dNodesDataSourceType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "K3d Cluster Node Listing Data Source",
		Attributes: map[string]tfsdk.Attribute{
			"cluster_name": {
				MarkdownDescription: "Name of the K3D cluster for which to retrieve node information",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.UseStateForUnknown(),
				},
			},
			"id": {
				MarkdownDescription: "Unique cluster identifier",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.UseStateForUnknown(),
				},
			},

			"nodes": {
				MarkdownDescription: "Map of node names to node information",
				Computed:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					resource.UseStateForUnknown(),
				},
				Attributes: tfsdk.MapNestedAttributes(map[string]tfsdk.Attribute{
					"name": {
						MarkdownDescription: "The name of the nodes docker container",
						Computed:            true,
						Type:                types.StringType,
					},
					"role": {
						MarkdownDescription: "The K3d cluster role of the node",
						Computed:            true,
						Type:                types.StringType,
					},
					"ports": {
						MarkdownDescription: "Node port binding set",
						Computed:            true,
						Type: types.SetType{
							ElemType: portBindingType,
						},
					},
					"runtime_labels": {
						MarkdownDescription: "A map of runtime labels to their values",
						Computed:            true,
						Type: types.MapType{
							ElemType: types.StringType,
						},
					},
					"node_labels": {
						MarkdownDescription: "A map of K3s node labels to their values",
						Computed:            true,
						Type: types.MapType{
							ElemType: types.StringType,
						},
					},
					"networks": {
						MarkdownDescription: "The list of docker networks to which the node is attached ",
						Computed:            true,
						Type: types.ListType{
							ElemType: types.StringType,
						},
					},
					"ip": {
						MarkdownDescription: "The IP address of the node's container",
						Computed:            true,
						Type:                types.StringType,
					},
				}),
			},
		},
	}, nil
}

func (t k3dNodesDataSourceType) NewDataSource(ctx context.Context, in provider.Provider) (datasource.DataSource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return k3dNodesDataSource{
		provider: provider,
	}, diags
}

type k3dNodesData struct {
	ClusterName types.String       `tfsdk:"cluster_name"`
	Id          types.String       `tfsdk:"id"`
	Nodes       map[string]k3dNode `tfsdk:"nodes"`
}

type k3dNode struct {
	Name          string            `tfsdk:"name"`
	Role          string            `tfsdk:"role"`
	Ports         []k3dPort         `tfsdk:"ports"`
	RuntimeLabels map[string]string `tfsdk:"runtime_labels"`
	NodeLabels    map[string]string `tfsdk:"node_labels"`
	Networks      []string          `tfsdk:"networks"`
	IP            string            `tfsdk:"ip"`
}

type k3dPort struct {
	Port     int64  `tfsdk:"port"`
	HostIP   string `tfsdk:"host_ip"`
	HostPort int64  `tfsdk:"host_port"`
}

type k3dNodesDataSource struct {
	provider k3dProvider
}

func (d k3dNodesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data k3dNodesData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	nodes, err := client.NodeList(ctx, runtimes.SelectedRuntime)
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to lister K3d nodes", err.Error()))
		return
	}

	newNodes := make(map[string]k3dNode)
	for _, node := range nodes {
		var ports []k3dPort

		for port, bindings := range node.Ports {
			for _, binding := range bindings {
				hostPort, err := strconv.ParseInt(binding.HostPort, 10, 16)
				if err != nil {
					continue
				}

				ports = append(ports, k3dPort{
					Port:     int64(port.Int()),
					HostIP:   binding.HostIP,
					HostPort: hostPort,
				})
			}
		}

		newNodes[node.Name] = k3dNode{
			Name:          node.Name,
			Role:          string(node.Role),
			Ports:         ports,
			RuntimeLabels: node.RuntimeLabels,
			NodeLabels:    node.K3sNodeLabels,
			Networks:      node.Networks,
			IP:            node.IP.IP.String(),
		}
	}

	// For the purposes of this example code, hardcoding a response value to
	// save into the Terraform state.
	data.Id = data.ClusterName
	data.Nodes = newNodes

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
