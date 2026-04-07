resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "mdq-standard-workers"
  node_role_arn   = aws_iam_role.eks_node_role.arn
  
  # Deploy nodes only in the private subnets
  subnet_ids      = module.vpc.private_subnet_ids

  # SOTA: Define the size and type of the compute instances
  instance_types = ["t3.small"]
  capacity_type  = "ON_DEMAND" # You can use "SPOT" later to save 70% on AWS bills!

  scaling_config {
    desired_size = 2 # Start with 2 nodes
    min_size     = 1 # Never drop below 1
    max_size     = 4 # Allow K8s to scale up to 4 if load gets heavy
  }

  # Ensure IAM policies are attached before creating the nodes
  depends_on = [
    aws_iam_role_policy_attachment.eks_worker_node_policy,
    aws_iam_role_policy_attachment.eks_cni_policy,
    aws_iam_role_policy_attachment.ecr_read_only,
  ]
}