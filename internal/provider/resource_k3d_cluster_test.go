package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccK3DClusterResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccK3DClusterResourceConfig("acc-test"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("k3d_cluster.test", "name", "acc-test"),
					// resource.TestCheckResourceAttr("k3d_cluster.test", "servers", "1"),
					// resource.TestCheckResourceAttr("k3d_cluster.test", "agents", "0"),
					// resource.TestCheckResourceAttr("k3d_cluster.test", "network", "blah"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccK3DClusterResourceConfig(name string) string {
	return fmt.Sprintf(`
resource "k3d_cluster" "test" {
  name = %[1]q
}
`, name)
}
