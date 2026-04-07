# 1. Define local variables to keep things clean
locals {
  cluster_name    = "mdq-prod"
  cluster_version = "1.30" # Always pin the K8s version
}

# 2. Consume our custom Network Module
module "vpc" {
  source = "../modules/network"

  cluster_name    = local.cluster_name
  vpc_cidr        = "10.0.0.0/16"
  public_subnets  = ["10.0.1.0/24", "10.0.2.0/24"]
  private_subnets = ["10.0.101.0/24", "10.0.102.0/24"]
}

# 3. Create the EKS Control Plane
resource "aws_eks_cluster" "main" {
  name     = local.cluster_name
  version  = local.cluster_version
  role_arn = aws_iam_role.eks_cluster_role.arn

  vpc_config {
    # We pass the private subnets from the module output. 
    # EKS will drop ENIs here to talk to our worker nodes.
    subnet_ids = module.vpc.private_subnet_ids
    
    # SOTA Security: We allow the API server to be accessible via the public internet 
    # (so you can run kubectl from your laptop), but restrict access to private nodes.
    endpoint_private_access = true
    endpoint_public_access  = true
  }

  # Ensure the IAM Policy is attached BEFORE creating the cluster, 
  # or EKS won't have permission to build the VPC config.
  depends_on = [
    aws_iam_role_policy_attachment.eks_cluster_policy
  ]
}