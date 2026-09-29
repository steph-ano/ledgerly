resource "aws_db_subnet_group" "rds" {
  name       = "${var.project_name}-${var.environment}-db-subnet-group"
  subnet_ids = aws_subnet.private_db[*].id

  tags = {
    Name = "${var.project_name}-${var.environment}-db-subnet-group"
  }
}

resource "aws_security_group" "rds" {
  name        = "${var.project_name}-${var.environment}-rds-sg"
  description = "Controls database access from private compute subnets"
  vpc_id      = aws_vpc.main.id

  ingress {
    description     = "PostgreSQL access from EKS nodes"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.project_name}-${var.environment}-rds-sg"
  }
}

resource "aws_kms_key" "rds" {
  description             = "KMS key for Ledgerly RDS encryption"
  deletion_window_in_days = 30
  enable_key_rotation     = true

  tags = {
    Name = "${var.project_name}-${var.environment}-rds-kms"
  }
}

resource "random_password" "db_master_password" {
  length           = 32
  special          = true
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

resource "aws_secretsmanager_secret" "db_credentials" {
  name                    = "${var.project_name}/${var.environment}/database"
  kms_key_id              = aws_kms_key.rds.id
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "db_credentials" {
  secret_id = aws_secretsmanager_secret.db_credentials.id
  secret_string = jsonencode({
    engine   = "postgres"
    host     = aws_db_instance.postgres.address
    port     = aws_db_instance.postgres.port
    username = var.db_username
    password = random_password.db_master_password.result
    database = var.db_name
  })
}

resource "aws_db_parameter_group" "postgres16" {
  name   = "${var.project_name}-${var.environment}-pg16-params"
  family = "postgres16"

  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }

  parameter {
    name  = "log_connections"
    value = "1"
  }

  parameter {
    name  = "log_disconnections"
    value = "1"
  }

  parameter {
    name  = "log_duration"
    value = "0"
  }

  parameter {
    name  = "log_statement"
    value = "ddl"
  }
}

resource "aws_db_instance" "postgres" {
  identifier                  = "${var.project_name}-${var.environment}-postgres"
  engine                      = "postgres"
  engine_version              = "16.3"
  instance_class              = var.db_instance_class
  allocated_storage           = var.db_allocated_storage
  max_allocated_storage       = var.db_max_allocated_storage
  storage_type                = "gp3"
  storage_encrypted           = true
  kms_key_id                  = aws_kms_key.rds.arn

  db_name                     = var.db_name
  username                    = var.db_username
  password                    = random_password.db_master_password.result

  db_subnet_group_name        = aws_db_subnet_group.rds.name
  vpc_security_group_ids      = [aws_security_group.rds.id]
  parameter_group_name        = aws_db_parameter_group.postgres16.name

  multi_az                    = true
  publicly_accessible         = false
  deletion_protection         = true
  skip_final_snapshot         = false
  final_snapshot_identifier   = "${var.project_name}-${var.environment}-final-snapshot"
  backup_retention_period     = 30
  backup_window               = "03:00-04:00"
  maintenance_window          = "Sun:04:30-Sun:05:30"
  auto_minor_version_upgrade  = true

  tags = {
    Name = "${var.project_name}-${var.environment}-rds"
  }
}
