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

# Example: Generate a DAG using inline template content
resource "mirage_dag_generator" "inline_dag" {
  dag_generator_backend_url = "https://your-backend-service.com"
  
  # Use inline template content instead of GCS path
  template_content = file("${path.module}/templates/data_pipeline.py.j2")
  
  target_gcs_path = "gs://your-bucket/dags/data_pipeline.py"
  
  context_json = jsonencode({
    dag_id            = "data_pipeline_dag"
    schedule_interval = "0 2 * * *"  # Daily at 2 AM
    owner            = "analytics-team"
    start_date       = "2024-01-01"
    catchup          = false
    max_active_runs  = 1
    
    # Pipeline configuration
    source_table     = "raw_data.events"
    target_table     = "processed_data.daily_metrics"
    notification_emails = ["team@company.com"]
    
    # Task configuration
    tasks = [
      {
        task_id = "extract_data"
        command = "python /scripts/extract.py"
        pool    = "default_pool"
        retries = 3
      },
      {
        task_id = "validate_data"
        command = "python /scripts/validate.py"
        pool    = "default_pool"
        retries = 2
      },
      {
        task_id = "transform_data"
        command = "python /scripts/transform.py"
        pool    = "heavy_pool"
        retries = 2
      },
      {
        task_id = "load_data"
        command = "python /scripts/load.py"
        pool    = "default_pool"
        retries = 3
      }
    ]
  })
  
  # Use service account authentication
  use_gcp_service_account_auth = true
}

# Output the generated DAG information
output "inline_dag_info" {
  value = {
    id         = mirage_dag_generator.inline_dag.id
    checksum   = mirage_dag_generator.inline_dag.generated_file_checksum
    generation = mirage_dag_generator.inline_dag.gcs_generation_number
  }
} 