variable "filename" {
  type        = string
  description = "Name of the local file to create"
  default     = "local_test_file.txt"
}

variable "file_content" {
  type        = string
  description = "Content written to the local file"
  default     = "Hello from local Terraform running on Windows!"
}