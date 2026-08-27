resource "local_file" "sample_file" {
  filename = "${path.module}/${var.filename}"
  content  = var.file_content
}