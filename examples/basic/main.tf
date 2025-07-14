terraform {
  required_providers {
    mirage = {
      source  = "localhost/my-org/mirage"
      version = "~> 1.0"
    }
  }
}

provider "mirage" {
  # No configuration required
}

# Example: Generate a simple DAG from a GCS template
resource "mirage_dag_generator" "simple_dag" {
  dag_generator_backend_url = "https://your-backend-service.com"
  template_gcs_path        = "gs://your-bucket/templates/simple_dag.py.j2"
  target_gcs_path          = "gs://your-bucket/dags/simple_dag.py"
  
  context_json = jsonencode({
    dag_id            = "simple_example_dag"
    schedule_interval = "@daily"
    owner            = "data-team"
    start_date       = "2024-01-01"
    catchup          = false
  })
  
  use_gcp_service_account_auth = true
}

# Output the generated file information
output "dag_checksum" {
  value = mirage_dag_generator.simple_dag.generated_file_checksum
}

output "dag_generation" {
  value = mirage_dag_generator.simple_dag.gcs_generation_number
}

output "dag_path" {
  value = mirage_dag_generator.simple_dag.id
} 