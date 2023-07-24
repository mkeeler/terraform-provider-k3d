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
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	client "github.com/k3d-io/k3d/v5/pkg/client"
	confutils "github.com/k3d-io/k3d/v5/pkg/config"
	conftypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	config "github.com/k3d-io/k3d/v5/pkg/config/v1alpha5"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3dtypes "github.com/k3d-io/k3d/v5/pkg/types"
	k3dutil "github.com/k3d-io/k3d/v5/pkg/util"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = k3dCluster{}

func NewClusterResource() resource.Resource {
	return k3dCluster{}
}

type k3dClusterData struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Servers     types.Int64  `tfsdk:"servers"`
	Agents      types.Int64  `tfsdk:"agents"`
	K8sHost     types.String `tfsdk:"k8s_api_host"`
	K8sHostIP   types.String `tfsdk:"k8s_api_host_ip"`
	K8sHostPort types.Int64  `tfsdk:"k8s_api_host_port"`
	Image       types.String `tfsdk:"image"`
	ImageSHA    types.String `tfsdk:"image_sha"`
	Network     types.String `tfsdk:"network"`
}

type k3dCluster struct {
}

func (k3dCluster) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (k3dCluster) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "K3D Cluster",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name that you want to give to your cluster (will still be prefixed with `k3d-`)",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"servers": schema.Int64Attribute{
				MarkdownDescription: "Number of servers to create",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Default: int64default.StaticInt64(1),
			},
			"agents": schema.Int64Attribute{
				MarkdownDescription: "Number of agents to create",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Default: int64default.StaticInt64(0),
			},
			"k8s_api_host": schema.StringAttribute{
				MarkdownDescription: "The hostname to serve the Kubernetes APIs with",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					// These are example validators from terraform-plugin-framework-validators
					stringvalidator.LengthBetween(10, 256),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z0-9]+$`),
						"must contain only lowercase alphanumeric characters",
					),
				},
			},
			"k8s_api_host_ip": schema.StringAttribute{
				MarkdownDescription: "The IP to bind the Kubernetes API",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Default: stringdefault.StaticString("127.0.0.1"),
				Validators: []validator.String{
					validateIP,
				},
			},
			"k8s_api_host_port": schema.Int64Attribute{
				MarkdownDescription: "The port to bind the Kubernetes API",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
				Default: int64default.StaticInt64(6550),
				Validators: []validator.Int64{
					validatePort,
				},
			},
			"image": schema.StringAttribute{
				MarkdownDescription: "Name of the K3s node image",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Default: stringdefault.StaticString("latest"),
			},
			"image_sha": schema.StringAttribute{
				MarkdownDescription: "SHA of the docker image that was used",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network": schema.StringAttribute{
				MarkdownDescription: "Name of the network the K3s nodes get attached to. If unset, a new network will be created.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "The ID of the cluster",
				Computed:            true,
			},
		},
	}
}

func (c k3dCluster) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data k3dClusterData

	diags := req.Plan.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := client.ClusterGet(ctx, runtimes.SelectedRuntime, &k3dtypes.Cluster{Name: data.Name.ValueString()})
	if err == nil {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic("A cluster with the same name already exists", data.Name.ValueString()))
		return
	}

	tflog.Trace(ctx, "synthesizing configuration")
	simpleConf := config.SimpleConfig{
		Servers: int(data.Servers.ValueInt64()),
		Agents:  int(data.Agents.ValueInt64()),
		Image:   data.Image.ValueString(),
		ObjectMeta: conftypes.ObjectMeta{
			Name: data.Name.ValueString(),
		},
		Options: config.SimpleConfigOptions{
			K3dOptions: config.SimpleConfigOptionsK3d{
				Wait:    true,
				Timeout: 90 * time.Second,
			},
		},
	}

	if !data.Network.IsNull() {
		simpleConf.Network = data.Network.ValueString()
	}

	if !data.K8sHost.IsNull() {
		simpleConf.ExposeAPI.Host = data.K8sHost.ValueString()
	}

	if !data.K8sHostIP.IsNull() {
		simpleConf.ExposeAPI.HostIP = data.K8sHostIP.ValueString()
	}

	if !data.K8sHostPort.IsNull() {
		simpleConf.ExposeAPI.HostPort = data.K8sHostPort.String()
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
		if err := client.ClusterDelete(ctx, runtimes.SelectedRuntime, &clusterConfig.Cluster, k3dtypes.ClusterDeleteOpts{SkipRegistryCheck: true}); err != nil {
			resp.Diagnostics.Append(diag.NewWarningDiagnostic("Error rolling back failed cluster creation", err.Error()))
		}
		return
	}
	tflog.Info(ctx, "cluster successfully created")

	tflog.Trace(ctx, "updating kubeconfig")
	_, err = client.KubeconfigGetWrite(ctx, runtimes.SelectedRuntime, &clusterConfig.Cluster, "", &client.WriteKubeConfigOptions{UpdateExisting: true, OverwriteExisting: false})
	if err != nil {
		resp.Diagnostics.Append(diag.NewWarningDiagnostic("Error writing kubeconfig", err.Error()))
	}

	resp.Diagnostics.Append(c.readCluster(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (k3dCluster) readCluster(ctx context.Context, data *k3dClusterData) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	tflog.Info(ctx, fmt.Sprintf("reading cluster: %s", data.Name.ValueString()))
	cluster, err := client.ClusterGet(ctx, runtimes.SelectedRuntime, &k3dtypes.Cluster{Name: data.Name.ValueString()})
	if err != nil {
		diagnostics.Append(diag.NewErrorDiagnostic("Error reading k3d cluster", err.Error()))
		return diagnostics
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

		diagnostics.Append(diag.NewWarningDiagnostic("Multiple node images found", buf.String()))
	} else {
		for image := range images {
			data.ImageSHA = types.StringValue(image)
		}
	}

	data.Agents = types.Int64Value(int64(agentCount))
	data.Servers = types.Int64Value(int64(serverCount))
	data.Network = types.StringValue(cluster.Network.Name)
	data.ID = data.Name

	if cluster.KubeAPI != nil {
		data.K8sHost = types.StringValue(cluster.KubeAPI.Host)
		data.K8sHostIP = types.StringValue(cluster.KubeAPI.Binding.HostIP)
		port, err := strconv.ParseInt(cluster.KubeAPI.Binding.HostPort, 10, 16)
		if err != nil {
			diagnostics.Append(diag.NewWarningDiagnostic("Invalid port found in cluster settings", cluster.KubeAPI.Binding.HostPort))
		} else {
			data.K8sHostPort = types.Int64Value(port)
		}
	}

	return diagnostics
}

func (c k3dCluster) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data k3dClusterData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(c.readCluster(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}

func (r k3dCluster) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.Append(diag.NewErrorDiagnostic("Updates are unsupported", ""))
	return
}

func (r k3dCluster) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data k3dClusterData

	diags := req.State.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "reading cluster info")
	cluster, err := client.ClusterGet(ctx, runtimes.SelectedRuntime, &k3dtypes.Cluster{Name: data.Name.ValueString()})
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
