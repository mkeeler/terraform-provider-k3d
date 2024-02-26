package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccK3dNodesDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccExampleDataSourceConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "cluster_name", "test"),
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "id", "test"),
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "nodes.k3d-test-server-0.name", "k3d-test-server-0"),
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "nodes.k3d-test-server-0.role", "server"),
					resource.TestCheckResourceAttrSet("data.k3d_nodes.test", "nodes.k3d-test-server-0.runtime_labels.%"),
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "nodes.k3d-test-server-0.networks.0", "k3d-test"),
					resource.TestCheckResourceAttrSet("data.k3d_nodes.test", "nodes.k3d-test-server-0.ip"),
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "nodes.k3d-test-agent-0.role", "agent"),
					resource.TestCheckResourceAttr("data.k3d_nodes.test", "nodes.k3d-test-serverlb.role", "loadbalancer"),
					resource.TestCheckResourceAttrSet("data.k3d_nodes.test", "nodes.k3d-test-serverlb.ports.#"),
				),
			},
		},
	})
}

const testAccExampleDataSourceConfig = `
resource "k3d_cluster" "test" {
	name = "test"
	servers = 1
	agents = 1
	k8s_api_host_port = 6552
}

data "k3d_nodes" "test" {
  cluster_name = k3d_cluster.test.name
}
`
