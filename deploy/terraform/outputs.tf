output "ecr_repository_url" {
  description = "URL of the ECR repository to push images to."
  value       = aws_ecr_repository.app.repository_url
}

output "cloudwatch_log_group" {
  description = "Name of the CloudWatch log group for the service."
  value       = aws_cloudwatch_log_group.app.name
}

output "task_execution_role_arn" {
  description = "ARN of the ECS task execution IAM role."
  value       = aws_iam_role.task_execution.arn
}
