package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	client "github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ datasource.DataSource = k3dNodesDataSource{}

var portBindingType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"port":      types.Int64Type,
		"host_ip":   types.StringType,
		"host_port": types.Int64Type,
	},
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
}

func NewNodesDataSource() datasource.DataSource {
	return k3dNodesDataSource{}
}

func (d k3dNodesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data k3dNodesData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	fmt.Println("Reading list of existing K3d nodes")
	nodes, err := client.NodeList(ctx, runtimes.SelectedRuntime)
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to list K3d nodes", err.Error()))
		return
	}

	newNodes := make(map[string]k3dNode)
	for _, node := range nodes {
		// filter out nodes for other K3D clusters
		cluster, ok := node.RuntimeLabels["k3d.cluster"]
		if !ok || cluster != data.ClusterName.ValueString() {
			continue
		}

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

func (t k3dNodesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nodes"
}

func (t k3dNodesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "K3d Cluster Node Listing Data Source",
		Attributes: map[string]schema.Attribute{
			"cluster_name": schema.StringAttribute{
				MarkdownDescription: "Name of the K3D cluster for which to retrieve node information",
				Required:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique cluster identifier",
				Computed:            true,
			},

			"nodes": schema.MapNestedAttribute{
				MarkdownDescription: "Map of node names to node information",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the nodes docker container",
							Computed:            true,
						},
						"role": schema.StringAttribute{
							MarkdownDescription: "The K3d cluster role of the node",
							Computed:            true,
						},
						"ports": schema.SetAttribute{
							MarkdownDescription: "Node port binding set",
							Computed:            true,
							ElementType:         portBindingType,
						},
						"runtime_labels": schema.MapAttribute{
							MarkdownDescription: "A map of runtime labels to their values",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"node_labels": schema.MapAttribute{
							MarkdownDescription: "A map of K3s node labels to their values",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"networks": schema.ListAttribute{
							MarkdownDescription: "The list of docker networks to which the node is attached ",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"ip": schema.StringAttribute{
							MarkdownDescription: "The IP address of the node's container",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}
