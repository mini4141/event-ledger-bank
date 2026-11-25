resource "aws_security_group" "msk" {
  name        = "${var.name}-msk"
  description = "MSK Serverless: IAM auth port, from the EKS cluster security group only"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "Kafka IAM from EKS"
    from_port       = 9098
    to_port         = 9098
    protocol        = "tcp"
    security_groups = [aws_eks_cluster.this.vpc_config[0].cluster_security_group_id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [aws_vpc.main.cidr_block]
  }
}

resource "aws_msk_serverless_cluster" "this" {
  cluster_name = var.name

  vpc_config {
    subnet_ids         = aws_subnet.public[*].id
    security_group_ids = [aws_security_group.msk.id]
  }
  client_authentication {
    sasl {
      iam {
        enabled = true
      }
    }
  }
}
