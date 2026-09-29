output "vpc_id" {
  description = "ID of the VPC"
  value       = aws_vpc.main.id
}

output "rds_endpoint" {
  description = "Connection endpoint for the Multi-AZ RDS PostgreSQL instance"
  value       = aws_db_instance.postgres.address
}

output "rds_database_name" {
  description = "Database name"
  value       = aws_db_instance.postgres.db_name
}

output "eks_cluster_name" {
  description = "Name of the EKS Cluster"
  value       = aws_eks_cluster.main.name
}

output "eks_cluster_endpoint" {
  description = "Endpoint for EKS control plane API"
  value       = aws_eks_cluster.main.endpoint
}

output "secrets_manager_db_arn" {
  description = "ARN of Secrets Manager database credentials"
  value       = aws_secretsmanager_secret.db_credentials.arn
}
