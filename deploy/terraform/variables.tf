variable "aws_region" {
  description = "AWS region to deploy into."
  type        = string
  default     = "us-east-1"
}

variable "project_name" {
  description = "Project name, used as a prefix for created resources."
  type        = string
  default     = "ledgerline"
}

variable "environment" {
  description = "Deployment environment (e.g. production, staging)."
  type        = string
  default     = "production"
}

variable "image_retention_count" {
  description = "Number of tagged container images to retain in ECR."
  type        = number
  default     = 10
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention period in days."
  type        = number
  default     = 30
}

variable "tags" {
  description = "Additional tags applied to all resources."
  type        = map(string)
  default     = {}
}
