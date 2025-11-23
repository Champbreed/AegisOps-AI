resource "aws_s3_bucket" "unsecure_logs" {
bucket = "champbreed-unsecured-logs-2025"
acl    = "public-read" # Critical Security Issue
​tags = {
Name        = "PublicLogBucket"
Environment = "Dev"
}
}
​resource "aws_security_group" "open_ssh" {
name        = "allow_all_ssh"
description = "Allow SSH inbound traffic from anywhere"
vpc_id      = "vpc-12345678"
​ingress {
description = "SSH from anywhere"
from_port   = 22
to_port     = 22
protocol    = "tcp"
cidr_blocks = ["0.0.0.0/0"] # Critical Security Issue (Too permissive)
}
​egress {
from_port   = 0
to_port     = 0
protocol    = "-1"
cidr_blocks = ["0.0.0.0/0"]
}
}
​resource "aws_iam_user" "admin_user" {
name = "temp_admin_user"
​INFO: No MFA is configured by default on IAM users
​}
