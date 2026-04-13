# File: infra/terraform/eks/argocd.tf

resource "helm_release" "argocd" {
  name             = "argocd"
  repository       = "https://argoproj.github.io/argo-helm"
  chart            = "argo-cd"
  namespace        = "argocd"
  create_namespace = true
  version          = "7.0.0" # SOTA: Always pin Helm chart versions

  # We set this so we can access the Argo UI easily via port-forward later
  set {
    name  = "server.service.type"
    value = "ClusterIP"
  }

  # Ensure the cluster and node groups exist before installing
  depends_on = [
    aws_eks_node_group.main
  ]
}