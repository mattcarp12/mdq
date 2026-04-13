# 1. Get your AWS Account ID dynamically so we don't have to hardcode it
data "aws_caller_identity" "current" {}

variable "github_repo" {
  description = "The GitHub organization and repository name (e.g., account/repo)"
  type        = string
}

# 2. The Trust Policy: Tell AWS to trust GitHub Actions from your specific repo
data "aws_iam_policy_document" "github_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRoleWithWebIdentity"]

    principals {
      type        = "Federated"
      identifiers = [aws_iam_openid_connect_provider.github.arn]
    }

    # SOTA SECURITY: If you don't include this, ANY repository on GitHub 
    # could theoretically assume your role and push images to your AWS account.
    condition {
      test     = "StringLike"
      variable = "token.actions.githubusercontent.com:sub"
      values   = ["repo:${var.github_repo}:*"]
    }

    condition {
      test     = "StringEquals"
      variable = "token.actions.githubusercontent.com:aud"
      values   = ["sts.amazonaws.com"]
    }
  }
}

# 3. Create the Role
resource "aws_iam_role" "github_actions" {
  name               = "mdq-github-actions-ecr-role"
  assume_role_policy = data.aws_iam_policy_document.github_assume_role.json
}

# 4. The Permissions: Allow pushing ONLY to the repos created in ecr.tf
data "aws_iam_policy_document" "ecr_push" {
  # Getting the auth token is account-wide
  statement {
    effect    = "Allow"
    actions   = ["ecr:GetAuthorizationToken"]
    resources = ["*"]
  }

  # Pushing images is restricted to our specific ECR repos
  statement {
    effect = "Allow"
    actions = [
      "ecr:BatchCheckLayerAvailability",
      "ecr:GetDownloadUrlForLayer",
      "ecr:GetRepositoryPolicy",
      "ecr:DescribeRepositories",
      "ecr:ListImages",
      "ecr:DescribeImages",
      "ecr:BatchGetImage",
      "ecr:InitiateLayerUpload",
      "ecr:UploadLayerPart",
      "ecr:CompleteLayerUpload",
      "ecr:PutImage"
    ]
    # We dynamically grab the ARNs of the repos we created in ecr.tf
    resources = [for repo in aws_ecr_repository.repos : repo.arn]
  }
}

resource "aws_iam_policy" "github_actions_ecr_policy" {
  name        = "mdq-github-actions-ecr-policy"
  description = "Allows GitHub Actions to push to ECR"
  policy      = data.aws_iam_policy_document.ecr_push.json
}

resource "aws_iam_role_policy_attachment" "github_actions_ecr" {
  role       = aws_iam_role.github_actions.name
  policy_arn = aws_iam_policy.github_actions_ecr_policy.arn
}

resource "aws_iam_openid_connect_provider" "github" {
  url            = "https://token.actions.githubusercontent.com"
  client_id_list = ["sts.amazonaws.com"]
}

# 5. Output the exact string you need to paste into your GitHub workflow!
output "github_actions_role_arn" {
  description = "Paste this ARN into your GitHub Actions workflow"
  value       = aws_iam_role.github_actions.arn
}
