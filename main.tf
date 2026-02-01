resource "aws_instance" "web_server" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "p3.8xlarge" 
  tags = {
    Environment = "Production"
  }
}

resource "aws_security_group" "allow_all" {
  name = "open-access"
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}
