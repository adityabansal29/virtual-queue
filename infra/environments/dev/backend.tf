# Initialize with: terraform init -backend-config=backend.hcl
terraform {
  backend "s3" {
    key    = "dev/terraform.tfstate"
    region = "ap-south-1"
  }
}
