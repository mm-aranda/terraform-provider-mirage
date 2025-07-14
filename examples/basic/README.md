# Basic Mirage Provider Example

This example demonstrates the basic usage of the Mirage provider to generate a simple Airflow DAG from a Jinja2 template stored in Google Cloud Storage.

## Prerequisites

1. A backend service running and accessible at your configured URL
2. Google Cloud Storage bucket with appropriate permissions
3. A Jinja2 template file uploaded to GCS
4. GCP credentials configured (if using service account auth)

## Usage

1. Update the configuration in `main.tf` with your actual values:
   - `dag_generator_backend_url`: Your backend service URL
   - `template_gcs_path`: Path to your Jinja2 template in GCS
   - `target_gcs_path`: Desired output path for the generated DAG

2. Initialize and apply the configuration:
   ```bash
   terraform init
   terraform plan
   terraform apply
   ```

3. The provider will:
   - Send the template and context to your backend service
   - Generate the DAG file and store it in GCS
   - Track the file's checksum and generation number

## Example Template

Here's a simple Jinja2 template that could be used with this example:

```python
from datetime import datetime, timedelta
from airflow import DAG
from airflow.operators.bash import BashOperator

default_args = {
    'owner': '{{ owner }}',
    'depends_on_past': False,
    'start_date': datetime.strptime('{{ start_date }}', '%Y-%m-%d'),
    'email_on_failure': False,
    'email_on_retry': False,
    'retries': 1,
    'retry_delay': timedelta(minutes=5),
}

dag = DAG(
    '{{ dag_id }}',
    default_args=default_args,
    description='A simple example DAG',
    schedule_interval='{{ schedule_interval }}',
    catchup={{ catchup | lower }},
)

hello_task = BashOperator(
    task_id='hello_world',
    bash_command='echo "Hello World from {{ dag_id }}!"',
    dag=dag,
)
```

## Outputs

This example outputs:
- `dag_checksum`: The CRC32C checksum of the generated file
- `dag_generation`: The GCS generation number
- `dag_path`: The full GCS path of the generated file 