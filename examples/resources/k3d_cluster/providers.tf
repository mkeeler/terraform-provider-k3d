terraform {
  required_providers {
    k3d = {
      source = "terraform.local/mkeeler/k3d"
    }
  }
  required_version = ">= 1.2.0"
}

provider "k3d" {}
