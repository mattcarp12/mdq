ifneq (,$(wildcard ./.env))
    include .env
    export
endif

AWS_REGION ?= us-west-2
ENVIRONMENT ?= dev
GITHUB_ORG ?= mattcarp12
GITHUB_REPO ?= mdq
INFRA_DIR := infra/aws-ecs

OIDC_STACK := mdq-github-oidc
STATE_STACK := mdq-state-$(ENVIRONMENT)
API_STACK := mdq-api-$(ENVIRONMENT)
FRONTEND_STACK := mdq-frontend-$(ENVIRONMENT)

# Automatically get your AWS Account ID
AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query Account --output text)

.PHONY: help deploy-oidc destroy-oidc deploy-state destroy-state create-ecr destroy-ecr deploy-api destroy-api deploy-frontend destroy-frontend deploy-all destroy-all kind docker

help: ## Display this help menu
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Foundation ---
deploy-oidc: ## Deploy GitHub Actions OIDC Role
	aws cloudformation deploy --template-file $(INFRA_DIR)/github-oidc.yaml --stack-name $(OIDC_STACK) --capabilities CAPABILITY_NAMED_IAM --region $(AWS_REGION) --parameter-overrides GitHubOrg=$(GITHUB_ORG) GitHubRepo=$(GITHUB_REPO)

create-ecr: ## Create ECR repositories
	aws ecr create-repository --repository-name carpecode-task-queue-api --region $(AWS_REGION) || true
	aws ecr create-repository --repository-name carpecode-task-queue-worker --region $(AWS_REGION) || true

deploy-state: ## Deploy VPC, Aurora, and Redis
	aws cloudformation deploy \
		--template-file $(INFRA_DIR)/state.yaml \
		--stack-name $(STATE_STACK) \
		--region $(AWS_REGION) \
		--parameter-overrides \
			EnvironmentName=$(ENVIRONMENT) \
			DBMasterUser=$(DB_USER) \
			DBMasterPassword=$(DB_PASS)

# --- Compute & UI ---
deploy-api: ## Deploy Fargate API using outputs from the State stack
	@echo "Fetching state outputs..."
	$(eval VPC_ID := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='VpcId'].OutputValue" --output text))
	$(eval SUBNETS := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='PublicSubnetIds'].OutputValue" --output text))
	
	@echo "Deploying Compute Resources..."
	aws cloudformation deploy \
		--template-file $(INFRA_DIR)/compute.yaml \
		--stack-name $(API_STACK) \
		--capabilities CAPABILITY_IAM \
		--region $(AWS_REGION) \
		--parameter-overrides \
			EnvironmentName=$(ENVIRONMENT) \
			VpcId=$(VPC_ID) \
			Subnets=$(SUBNETS) \
			ApiImageUrl=$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/carpecode-task-queue-api:latest \
			WorkerImageUrl=$(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/carpecode-task-queue-worker:latest \
			AlbCertificateArn=$(ALB_CERTIFICATE_ARN) \
			AllowedOrigins=$(ALLOWED_ORIGINS)

deploy-frontend: ## Deploy S3 and CloudFront CDN
	aws cloudformation deploy \
		--template-file $(INFRA_DIR)/frontend-cdn.yaml \
		--stack-name $(FRONTEND_STACK) \
		--region $(AWS_REGION) \
		--parameter-overrides \
			EnvironmentName=$(ENVIRONMENT) \
			CustomDomainName=$(CUSTOM_DOMAIN_NAME) \
			CfCertificateArn=$(CF_CERTIFICATE_ARN)

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

get-state-urls: ## Get outputs from the State stack
	$(eval DB_ENDPOINT := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='DatabaseEndpoint'].OutputValue" --output text))
	$(eval REDIS_ENDPOINT := $(shell aws cloudformation describe-stacks --stack-name $(STATE_STACK) --query "Stacks[0].Outputs[?OutputKey=='RedisEndpoint'].OutputValue" --output text))
	@echo postgres://$(DB_USER):$(DB_PASS)@$(DB_ENDPOINT):5432/taskqueue?sslmode=require
	@echo redis://$(REDIS_ENDPOINT):6379/0

# Docker build 
docker.api:
	docker build \
		--build-arg APP_NAME=api \
		-t mdq-api:latest \
		-f backend/Dockerfile \
		backend/

docker.worker:
	docker build \
		--build-arg APP_NAME=worker \
		-t mdq-worker:latest \
		-f backend/Dockerfile \
		backend/

docker: docker.api docker.worker

docker.load.kind: docker
	kind load docker-image mdq-api:latest --name mdq-local
	kind load docker-image mdq-worker:latest --name mdq-local

kind:
	kind create cluster --name mdq-local