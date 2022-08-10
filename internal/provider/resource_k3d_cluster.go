package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	client "github.com/k3d-io/k3d/v5/pkg/client"
	confutils "github.com/k3d-io/k3d/v5/pkg/config"
	conftypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3dtypes "github.com/k3d-io/k3d/v5/pkg/types"
	k3dutil "github.com/k3d-io/k3d/v5/pkg/util"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ tfsdk.ResourceType = k3dClusterType{}
var _ tfsdk.Resource = k3dCluster{}

type k3dClusterType struct{}

func (t k3dClusterType) GetSchema(ctx context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "K3D Cluster",

		Attributes: map[string]tfsdk.Attribute{
			"name": {
				MarkdownDescription: "Name that you want to give to your cluster (will still be prefixed with `k3d-`)",
				Required:            true,
				Type:                types.StringType,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.RequiresReplace(),
				},
			},
			"servers": {
				MarkdownDescription: "Number of servers to create",
				Optional:            true,
				Type:                types.Int64Type,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.RequiresReplace(),
				},
			},
			"agents": {
				MarkdownDescription: "Number of agents to create",
				Optional:            true,
				Type:                types.Int64Type,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.RequiresReplace(),
				},
			},
			"expose_api": {
				MarkdownDescription: "",
				Optional:            true,
				Computed:            true,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.RequiresReplace(),
				},
				Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
					"host": {
						MarkdownDescription: "The hostname to serve the Kubernetes APIs with",
						Optional:            true,
						Type:                types.StringType,
						PlanModifiers: tfsdk.AttributePlanModifiers{
							tfsdk.UseStateForUnknown(),
							tfsdk.RequiresReplace(),
						},
						Validators: []tfsdk.AttributeValidator{
							// These are example validators from terraform-plugin-framework-validators
							stringvalidator.LengthBetween(10, 256),
							stringvalidator.RegexMatches(
								regexp.MustCompile(`^[a-z0-9]+$`),
								"must contain only lowercase alphanumeric characters",
							),
						},
					},
					"host_ip": {
						MarkdownDescription: "The IP to bind the Kubernetes API",
						Optional:            true,
						Type:                types.StringType,
						PlanModifiers: tfsdk.AttributePlanModifiers{
							tfsdk.UseStateForUnknown(),
							tfsdk.RequiresReplace(),
						},
						Validators: []tfsdk.AttributeValidator{
							validateIP,
						},
					},
					"host_port": {
						MarkdownDescription: "The port to bind the Kubernetes API",
						Optional:            true,
						Type:                types.Int64Type,
						PlanModifiers: tfsdk.AttributePlanModifiers{
							tfsdk.UseStateForUnknown(),
							tfsdk.RequiresReplace(),
						},
						Validators: []tfsdk.AttributeValidator{
							validatePort,
						},
					},
				}),
			},
			"image": {
				MarkdownDescription: "Name of the K3s node image",
				Optional:            true,
				Computed:            true,
				Type:                types.StringType,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.UseStateForUnknown(),
					tfsdk.RequiresReplace(),
				},
			},
			"network": {
				MarkdownDescription: "Name of the network the K3s nodes get attached to. If unset, a new network will be created.",
				Optional:            true,
				Computed:            true,
				Type:                types.StringType,
				PlanModifiers: tfsdk.AttributePlanModifiers{
					tfsdk.UseStateForUnknown(),
					tfsdk.RequiresReplace(),
				},
			},
		},
	}, nil
}

func (t k3dClusterType) NewResource(ctx context.Context, in tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	provider, diags := convertProviderType(in)

	return k3dCluster{
		provider: provider,
	}, diags
}

type k3dClusterData struct {
	Name      types.String `tfsdk:"name"`
	Servers   types.Int64  `tfsdk:"servers"`
	Agents    types.Int64  `tfsdk:"agents"`
	ExposeAPI struct {
		Host     types.String `tfsdk:"host"`
		HostIP   types.String `tfsdk:"host_ip"`
		HostPort types.Int64  `tfsdk:"host_port"`
	} `tfsdk:"expose_api"`
	Image   types.String `tfsdk:"image"`
	Network types.String `tfsdk:"network"`
}

type k3dCluster struct {
	provider provider
}

func (r k3dCluster) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var data k3dClusterData

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("creating cluster: %s", data.Name.Value))
	_, err := client.ClusterGet(ctx, runtimes.SelectedRuntime, &k3dtypes.Cluster{Name: data.Name.Value})
	if err == nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("A cluster with the same name already exists", data.Name.Value))
		return
	}

	if data.Servers.Null {
		data.Servers.Value = 1
	}

	if data.Agents.Null {
		data.Agents.Value = 0
	}

	if data.Image.Null {
		data.Image.Value = "latest"
	}

	tflog.Trace(ctx, "synthesizing configuration")
	simpleConf := config.SimpleConfig{
		Servers: int(data.Servers.Value),
		Agents:  int(data.Agents.Value),
		ExposeAPI: config.SimpleExposureOpts{
			Host:     data.ExposeAPI.Host.Value,
			HostIP:   data.ExposeAPI.HostIP.Value,
			HostPort: data.ExposeAPI.HostPort.String(),
		},
		Image:   data.Image.Value,
		Network: data.Network.Value,
		ObjectMeta: conftypes.ObjectMeta{
			Name: data.Name.Value,
		},
	}

	tflog.Trace(ctx, "normalizing configuration")
	if err := confutils.ProcessSimpleConfig(&simpleConf); err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error processing K3D simple configuration", err.Error()))
		return
	}

	tflog.Trace(ctx, "generating k3d cluster configuration from simple config")
	clusterConfig, err := confutils.TransformSimpleToClusterConfig(ctx, runtimes.SelectedRuntime, simpleConf)
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error transforming simple config to cluster config", err.Error()))
		return
	}

	tflog.Trace(ctx, "normalizing cluster configuration")
	clusterConfig, err = confutils.ProcessClusterConfig(*clusterConfig)
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error processing cluster config", err.Error()))
		return
	}

	tflog.Trace(ctx, "validating cluster configuration")
	err = confutils.ValidateClusterConfig(ctx, runtimes.SelectedRuntime, *clusterConfig)
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error validating cluster config", err.Error()))
		return
	}

	tflog.Info(ctx, "creating k3d cluster")
	err = client.ClusterRun(ctx, runtimes.SelectedRuntime, clusterConfig)
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error creating cluster", err.Error()))
		// if err := client.ClusterDelete(ctx, runtimes.SelectedRuntime, &clusterConfig.Cluster, k3dtypes.ClusterDeleteOpts{SkipRegistryCheck: true}); err != nil {
		// 	resp.Diagnostics.Append(diag.NewWarningDiagnostic("Error rolling back failed cluster creation", err.Error()))
		// }
		return
	}
	tflog.Info(ctx, "cluster successfully created")

	tflog.Trace(ctx, "updating kubeconfig")
	_, err = client.KubeconfigGetWrite(ctx, runtimes.SelectedRuntime, &clusterConfig.Cluster, "", &client.WriteKubeConfigOptions{UpdateExisting: true, OverwriteExisting: false})
	if err != nil {
		resp.Diagnostics.Append(diag.NewWarningDiagnostic("Error writing kubeconfig", err.Error()))
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r k3dCluster) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var data k3dClusterData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, fmt.Sprintf("reading cluster: %s", data.Name.Value))
	cluster, err := client.ClusterGet(ctx, runtimes.SelectedRuntime, &k3dtypes.Cluster{Name: data.Name.Value})
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error reading k3d cluster", err.Error()))
		return
	}

	agentCount := 0
	serverCount := 0
	images := make(map[string]struct{})
	for _, node := range cluster.Nodes {
		if node.Role != k3dtypes.AgentRole && node.Role != k3dtypes.ServerRole {
			continue
		}

		images[node.Image] = struct{}{}

		if node.Role == k3dtypes.AgentRole {
			agentCount++
		} else if node.Role == k3dtypes.ServerRole {
			serverCount++
		}
	}

	if len(images) > 1 {
		var buf strings.Builder

		prefix := ""
		for image := range images {
			buf.WriteString(fmt.Sprintf("%s%s", prefix, image))
			prefix = ", "
		}

		resp.Diagnostics.Append(diag.NewWarningDiagnostic("Multiple node images found", buf.String()))
	} else {
		for image := range images {
			data.Image.Value = image
		}
	}

	data.Agents.Value = int64(agentCount)
	data.Servers.Value = int64(serverCount)
	data.Network.Value = cluster.Network.Name

	if cluster.KubeAPI != nil {
		data.ExposeAPI.Host.Value = cluster.KubeAPI.Host
		data.ExposeAPI.HostIP.Value = cluster.KubeAPI.Binding.HostIP
		port, err := strconv.ParseUint(cluster.KubeAPI.Binding.HostPort, 10, 16)
		if err != nil {
			resp.Diagnostics.Append(diag.NewWarningDiagnostic("Invalid port found in cluster settings", cluster.KubeAPI.Binding.HostPort))
		} else {
			data.ExposeAPI.HostPort.Value = int64(port)
		}
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r k3dCluster) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	resp.Diagnostics.Append(diag.NewErrorDiagnostic("Updates are unsupported", ""))
	return
}

func (r k3dCluster) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var data k3dClusterData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "reading cluster info")
	cluster, err := client.ClusterGet(ctx, runtimes.SelectedRuntime, &k3dtypes.Cluster{Name: data.Name.Value})
	if err != nil {
		if errors.Is(err, client.ClusterGetNoNodesFoundError) {
			return
		}

		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Error reading k3d cluster", err.Error()))
		return
	}

	tflog.Trace(ctx, "deleting the cluster")
	if err := client.ClusterDelete(ctx, runtimes.SelectedRuntime, cluster, k3dtypes.ClusterDeleteOpts{SkipRegistryCheck: false}); err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to delete the cluster", err.Error()))
	}

	tflog.Trace(ctx, "removing kubecfongig from default config")
	if err := client.KubeconfigRemoveClusterFromDefaultConfig(ctx, cluster); err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to remove kubeconfig from default config", err.Error()))
	}

	tflog.Trace(ctx, "removing standalone kubeconfig")
	configDir, err := k3dutil.GetConfigDirOrCreate()
	if err != nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("Failed to delete kubeconfig file", err.Error()))
	} else {
		kubeconfigfile := path.Join(configDir, fmt.Sprintf("kubeconfig-%s.yaml", cluster.Name))
		if err := os.Remove(kubeconfigfile); err != nil {
			if !os.IsNotExist(err) {
				resp.Diagnostics.Append(diag.NewErrorDiagnostic(fmt.Sprintf("Failed to delete kubeconfig file '%s'", kubeconfigfile), err.Error()))
			}
		}
	}
}
