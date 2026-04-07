# File: infra/terraform/modules/network/variables.tf

variable "cluster_name" {
  description = "The name of the EKS cluster (used for tagging)"
  type        = string
}

variable "vpc_cidr" {
  description = "The IP address range for the entire VPC"
  type        = string
  default     = "10.0.0.0/16" # This gives us 65,536 IP addresses to play with
}

variable "public_subnets" {
  description = "A list of CIDR blocks for public subnets"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24"] # 256 IPs each
}

variable "private_subnets" {
  description = "A list of CIDR blocks for private subnets"
  type        = list(string)
  default     = ["10.0.101.0/24", "10.0.102.0/24"]
}