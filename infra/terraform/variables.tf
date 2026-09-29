variable "aws_region" {
  type        = string
  description = "Target AWS Region"
  default     = "us-east-1"
}

variable "environment" {
  type        = string
  description = "Environment name (dev, staging, prod)"
  default     = "prod"
}

variable "project_name" {
  type        = string
  description = "Project identifier"
  default     = "ledgerly"
}

variable "vpc_cidr" {
  type        = string
  description = "VPC CIDR block"
  default     = "10.0.0.0/16"
}

variable "db_instance_class" {
  type        = string
  description = "RDS instance class"
  default     = "db.r6g.xlarge"
}

variable "db_allocated_storage" {
  type        = number
  description = "Allocated storage in GB"
  default     = 100
}

variable "db_max_allocated_storage" {
  type        = number
  description = "Maximum auto-scaling storage in GB"
  default     = 500
}

variable "db_name" {
  type        = string
  description = "Database name"
  default     = "ledgerly_production"
}

variable "db_username" {
  type        = string
  description = "PostgreSQL master username"
  default     = "ledgerly_admin"
}
