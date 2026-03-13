terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = "ap-northeast-2"
}

resource "aws_db_instance" "gigbridge" {
  identifier          = "gigbridge-db"
  engine              = "postgres"
  engine_version      = "15"
  instance_class      = "db.t3.micro"
  allocated_storage   = 20
  db_name             = "gigbridge"
  username            = "postgres"
  password            = var.db_password
  skip_final_snapshot = true
}

variable "db_password" {
  type      = string
  sensitive = true
}

output "db_endpoint" {
  value = aws_db_instance.gigbridge.endpoint
}
