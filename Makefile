ifneq (,$(wildcard ./.env))
    include .env
    export
endif

AWS_REGION ?= us-west-2
ENVIRONMENT ?= dev
GITHUB_ORG ?= mattcarp12
GITHUB_REPO ?= mdq

OIDC_STACK := mdq-github-oidc
STATE_STACK := mdq-state-$(ENVIRONMENT)
API_STACK := mdq-api-$(ENVIRONMENT)
FRONTEND_STACK := mdq-frontend-$(ENVIRONMENT)

# Automatically get your AWS Account ID
AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query Account --output text)

.PHONY: help deploy-oidc destroy-oidc deploy-state destroy-state create-ecr destroy-ecr deploy-api destroy-api deploy-frontend destroy-frontend deploy-all destroy-all

help: ## Display this help menu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Foundation ---
deploy-oidc: ## Deploy GitHub Actions OIDC Role
	aws cloudformation deploy --template-file infra/github-oidc.yaml --stack-name $(OIDC_STACK) --capabilities CAPABILITY_NAMED_IAM --region $(AWS_REGION) --parameter-overrides GitHubOrg=$(GITHUB_ORG) GitHubRepo=$(GITHUB_REPO)

create-ecr: ## Create ECR repositories
	aws ecr create-repository --repository-name carpecode-task-queue-api --region $(AWS_REGION) || true
	aws ecr create-repository --repository-name carpecode-task-queue-worker --region $(AWS_REGION) || true

deploy-state: ## Deploy VPC, Aurora, and Redis
	aws cloudformation deploy --template-file infra/state.yaml --stack-name $(STATE_STACK) --region $(AWS_REGION) --parameter-overrides EnvironmentName=$(ENVIRONMENT) DBMasterUser=$(DB_USER) DBMasterPassword=$(DB_PASS)

# --- Compute & UI ---
deploy-api: ## Deploy Fargate API using outputs from the State stack
	@echo "Fetching state outputs..."
	$(eval VPC_ID := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='VpcId'].OutputValue" --output text))
	$(eval SUBNETS := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='PublicSubnetIds'].OutputValue" --output text))
	$(eval DB_URL := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='DatabaseUrl'].OutputValue" --output text))
	$(eval REDIS_URL := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='RedisUrl'].OutputValue" --output text))
	
	@echo "Deploying API Fargate Service..."
	aws cloudformation deploy \
		--template-file infra/api-fargate.yaml \
		--stack-name $(API_STACK) \
		--capabilities CAPABILITY_IAM \
		--region $(AWS_REGION) \
		--parameter-overrides \
			EnvironmentName=$(ENVIRONMENT) \
			VpcId=$(VPC_ID) \
			Subnets=$(SUBNETS) \
			ApiImageUrl=$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/carpecode-task-queue-api:latest \
			DatabaseUrl="$(DB_URL)" \
			RedisUrl="$(REDIS_URL)"

deploy-frontend: ## Deploy S3 and CloudFront CDN
	aws cloudformation deploy --template-file infra/frontend-cdn.yaml --stack-name $(FRONTEND_STACK) --region $(AWS_REGION) --parameter-overrides EnvironmentName=$(ENVIRONMENT)

# --- Destroyers ---
destroy-api:
	aws cloudformation delete-stack --stack-name $(API_STACK) --region $(AWS_REGION)

destroy-frontend:
	aws cloudformation delete-stack --stack-name $(FRONTEND_STACK) --region $(AWS_REGION)

destroy-state: destroy-api
	aws cloudformation delete-stack --stack-name $(STATE_STACK) --region $(AWS_REGION)

destroy-ecr:
	aws ecr delete-repository --repository-name carpecode-task-queue-api --force --region $(AWS_REGION) || true
	aws ecr delete-repository --repository-name carpecode-task-queue-worker --force --region $(AWS_REGION) || true

destroy-oidc:
	aws cloudformation delete-stack --stack-name $(OIDC_STACK) --region $(AWS_REGION)

deploy-all: deploy-oidc create-ecr deploy-state deploy-api deploy-frontend ## Deploy Everything
destroy-all: destroy-frontend destroy-api destroy-state destroy-ecr destroy-oidc ## Destroy Everything