# Load environment variables from .env file
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Default variables if not provided in .env
AWS_REGION ?= us-west-2
ENVIRONMENT ?= dev
GITHUB_ORG ?= mattcarp12
GITHUB_REPO ?= mdq

# Stack Names
OIDC_STACK := mdq-github-oidc
STATE_STACK := mdq-state-$(ENVIRONMENT)

.PHONY: help deploy-oidc destroy-oidc deploy-state destroy-state create-ecr destroy-ecr deploy-all destroy-all

# ==============================================================================
# Help Menu
# ==============================================================================
help: ## Display this help menu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ==============================================================================
# Deployment Commands
# ==============================================================================
deploy-oidc: ## Deploy the GitHub Actions OIDC IAM Role
	@echo "Deploying OIDC Stack..."
	aws cloudformation deploy \
		--template-file infra/github-oidc.yaml \
		--stack-name $(OIDC_STACK) \
		--capabilities CAPABILITY_NAMED_IAM \
		--region $(AWS_REGION) \
		--parameter-overrides \
			GitHubOrg=$(GITHUB_ORG) \
			GitHubRepo=$(GITHUB_REPO)

deploy-state: ## Deploy the VPC, Aurora PostgreSQL, and Redis
	@echo "Deploying Stateful Infrastructure (This may take 10-15 mins)..."
	aws cloudformation deploy \
		--template-file infra/state.yaml \
		--stack-name $(STATE_STACK) \
		--region $(AWS_REGION) \
		--parameter-overrides \
			EnvironmentName=$(ENVIRONMENT) \
			DBMasterUser=$(DB_USER) \
			DBMasterPassword=$(DB_PASS)

create-ecr: ## Create the private ECR repositories for API and Worker
	@echo "Creating ECR Repositories..."
	aws ecr create-repository --repository-name carpecode-task-queue-api --region $(AWS_REGION) || true
	aws ecr create-repository --repository-name carpecode-task-queue-worker --region $(AWS_REGION) || true

deploy-all: deploy-oidc create-ecr deploy-state ## Deploy OIDC, ECR, and Stateful Infra

# ==============================================================================
# Destruction Commands
# ==============================================================================
destroy-state: ## Destroy the VPC, Aurora, and Redis
	@echo "Destroying Stateful Infrastructure..."
	aws cloudformation delete-stack --stack-name $(STATE_STACK) --region $(AWS_REGION)
	aws cloudformation wait stack-delete-complete --stack-name $(STATE_STACK) --region $(AWS_REGION)
	@echo "Stateful Infrastructure destroyed."

destroy-oidc: ## Destroy the GitHub OIDC Role
	@echo "Destroying OIDC Stack..."
	aws cloudformation delete-stack --stack-name $(OIDC_STACK) --region $(AWS_REGION)

destroy-ecr: ## Force delete the ECR repositories AND all images inside them
	@echo "Destroying ECR Repositories..."
	aws ecr delete-repository --repository-name carpecode-task-queue-api --force --region $(AWS_REGION) || true
	aws ecr delete-repository --repository-name carpecode-task-queue-worker --force --region $(AWS_REGION) || true

destroy-all: destroy-state destroy-ecr destroy-oidc ## Destroy EVERYTHING (State, ECR, OIDC)