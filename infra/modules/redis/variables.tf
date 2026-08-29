variable "environment" {
  type    = string
  default = "dev"
}

variable "redis_node_type" {
  type    = string
  default = "cache.t3.micro"
}

variable "vpc_id" {
  type = string
}

variable "private_subnet_ids" {
  type = list(string)
}

variable "sg_redis_id" {
  type = string
}
