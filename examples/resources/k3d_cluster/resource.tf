resource "k3d_cluster" "cluster" {
  name = "foo"
  expose_api = {
    host_port = 1234
  }
}
