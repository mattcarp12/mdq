# SOTA: We use a 'for_each' loop to create multiple repositories dynamically
# rather than copying and pasting the resource block three times.

locals {
  repositories = [
    "mdq-api",
    "mdq-worker",
    "mdq-migrator"
  ]
}

resource "aws_ecr_repository" "repos" {
  for_each = toset(local.repositories)
  
  name                 = each.key
  image_tag_mutability = "MUTABLE" # Allows you to overwrite 'latest' during dev

  image_scanning_configuration {
    scan_on_push = true # SOTA Security: AWS will automatically scan your Go images for vulnerabilities
  }
}

# Output the base URL of the registry so we know where to push our Docker images
output "ecr_registry_url" {
  description = "The URL of the ECR registry"
  value       = split("/", aws_ecr_repository.repos["mdq-api"].repository_url)[0]
}