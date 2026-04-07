# File: infra/terraform/modules/network/main.tf

# 1. Ask AWS for the available data centers (Availability Zones)
data "aws_availability_zones" "available" {
  state = "available"
}

# 2. Create the VPC
resource "aws_vpc" "main" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true # Required for EKS
  enable_dns_support   = true

  tags = {
    Name = "${var.cluster_name}-vpc"
  }
}

# 3. Create the Public Subnets
# We use 'count' to loop through the list of public subnets in our variables
resource "aws_subnet" "public" {
  count                   = length(var.public_subnets)
  vpc_id                  = aws_vpc.main.id
  cidr_block              = var.public_subnets[count.index]
  
  # Put subnet 1 in AZ "a", subnet 2 in AZ "b", etc. for high availability
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  
  # Crucial: Any EC2 instance spawned here gets a public IP automatically
  map_public_ip_on_launch = true 

  tags = {
    Name = "${var.cluster_name}-public-${count.index + 1}"
    # EKS needs this specific tag to know where to put external Load Balancers
    "kubernetes.io/role/elb" = "1"
  }
}

# 4. Create the Private Subnets
resource "aws_subnet" "private" {
  count             = length(var.private_subnets)
  vpc_id            = aws_vpc.main.id
  cidr_block        = var.private_subnets[count.index]
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "${var.cluster_name}-private-${count.index + 1}"
    # EKS needs this specific tag to know where to put internal Load Balancers
    "kubernetes.io/role/internal-elb" = "1"
  }
}

# 5. The Internet Gateway (The front door)
# A VPC has no internet connection until you attach this.
resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.main.id

  tags = {
    Name = "${var.cluster_name}-igw"
  }
}

# ------------------------------------------------------------------------------
# 6. The NAT Gateway (The Mail Clerk)
# ------------------------------------------------------------------------------

# A NAT Gateway requires a static, public IP address to talk to the internet.
# In AWS, this is called an Elastic IP (EIP).
resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.cluster_name}-nat-eip"
  }
}

# We place the NAT Gateway in the FIRST Public Subnet.
resource "aws_nat_gateway" "nat" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id

  tags = {
    Name = "${var.cluster_name}-nat"
  }

  # Terraform tip: Don't build the NAT until the IGW is attached to the building
  depends_on = [aws_internet_gateway.igw]
}

# ------------------------------------------------------------------------------
# 7. Route Tables (The Maps)
# ------------------------------------------------------------------------------

# The Public Map: "If you are looking for the internet (0.0.0.0/0), go out the IGW."
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }

  tags = {
    Name = "${var.cluster_name}-public-rt"
  }
}

# The Private Map: "If you are looking for the internet (0.0.0.0/0), hand it to the NAT Gateway."
resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.nat.id
  }

  tags = {
    Name = "${var.cluster_name}-private-rt"
  }
}

# ------------------------------------------------------------------------------
# 8. Route Table Associations (Handing out the Maps)
# ------------------------------------------------------------------------------

# Force the Public Subnets to use the Public Map
resource "aws_route_table_association" "public" {
  count          = length(var.public_subnets)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

# Force the Private Subnets to use the Private Map
resource "aws_route_table_association" "private" {
  count          = length(var.private_subnets)
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}