terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

variable "region" {
  type    = string
  default = "ap-southeast-2"
}

variable "profile" {
  type    = string
  default = "event-ledger-bank-deployer"
}

variable "name" {
  type    = string
  default = "event-ledger-bank"
}

variable "kubernetes_version" {
  type    = string
  default = "1.36"
}

provider "aws" {
  region  = var.region
  profile = var.profile
  default_tags {
    tags = {
      Project   = "event-ledger-bank"
      Ephemeral = "true"
      ManagedBy = "terraform"
      Owner     = "alvieirvansyah"
    }
  }
}

data "aws_caller_identity" "me" {}
data "aws_availability_zones" "az" {
  state = "available"
}

locals {
  account_id = data.aws_caller_identity.me.account_id
  services   = ["accounts", "ledger", "transfers", "notifications", "analytics-sink", "topic-init", "web", "envoy"]
}

# ---------- network: public subnets only, no NAT gateway ----------
resource "aws_vpc" "main" {
  cidr_block           = "10.20.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true
  tags                 = { Name = var.name }
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.main.id
  tags   = { Name = var.name }
}

resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index)
  availability_zone       = data.aws_availability_zones.az.names[count.index]
  map_public_ip_on_launch = true
  tags = {
    Name                     = "${var.name}-public-${count.index}"
    "kubernetes.io/role/elb" = "1"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }
  tags = { Name = var.name }
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# ---------- ECR ----------
resource "aws_ecr_repository" "svc" {
  for_each             = toset(local.services)
  name                 = "${var.name}/${each.key}"
  force_delete         = true
  image_tag_mutability = "MUTABLE"
}

# ---------- logs (1-day retention, destroyed with the stack) ----------
resource "aws_cloudwatch_log_group" "eks" {
  name              = "/aws/eks/${var.name}/cluster"
  retention_in_days = 1
}


# Amazon Keyspaces needs cassandra:* permissions on the deploying principal. When they are missing
# (see docs/DECISIONS.md) deploy.sh sets this to false and the stack runs Cassandra inside EKS instead.
variable "enable_keyspaces" {
  type    = bool
  default = true
}
