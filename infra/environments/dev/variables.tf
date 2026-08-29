variable "aws_region" {
  type    = string
  default = "ap-south-1"
}

variable "environment" {
  type    = string
  default = "dev"
}

variable "tf_state_bucket" {
  type    = string
  default = ""
}

variable "vpc_cidr" {
  type    = string
  default = "10.0.0.0/16"
}
