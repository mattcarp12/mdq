# 1. The Terraform Settings Block
terraform {
  # We lock the required version so if you share this code with a teammate,
  # they don't accidentally use a newer version of Terraform that breaks things.
  required_version = ">= 1.5.0"

  # Providers are the "plugins" Terraform uses to talk to external APIs.
  # We tell it to download the official AWS plugin from HashiCorp.
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0" # Use any 5.x version, but don't jump to 6.0 automatically
    }
    
    http = {
      source  = "hashicorp/http"
      version = "~> 3.4"
    }
    
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
  }
}

# 2. The Provider Configuration Block
provider "aws" {
  region = "us-west-2" # Change this if you prefer a different AWS region

  # SOTA: Default Tags! 
  # This automatically attaches these tags to EVERY single resource Terraform creates.
  # This makes calculating your AWS bill for this specific project incredibly easy later.
  default_tags {
    tags = {
      Project     = "MDQ"
      Environment = "Production"
      ManagedBy   = "Terraform"
    }
  }
}

provider "helm" {
  kubernetes {
    host                   = aws_eks_cluster.main.endpoint
    cluster_ca_certificate = base64decode(aws_eks_cluster.main.certificate_authority[0].data)
    # This executes the AWS CLI in the background to grab a fresh auth token
    exec {
      api_version = "client.authentication.k8s.io/v1beta1"
      args        = ["eks", "get-token", "--cluster-name", aws_eks_cluster.main.name]
      command     = "aws"
    }
  }
}