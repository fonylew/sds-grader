#!/bin/bash

# --- BigQuery Dataset Configuration ---
# IMPORTANT: Update these variables with your actual class/major/semester/year
class_id="2110415"  # e.g., "2110XXX"
major="CEDT"        # e.g., "CP or CEDT"
semester="1"        # e.g., "1 or 2"
year="2025"         # e.g., "2025"

# Construct the dataset name
dataset_name="${class_id}_${major}_${semester}_${year}"
# Example: 2110415_CP_1_2025

# --- User Input Prompts ---
read -p "Enter the activity ID (e.g., activity1): " sa_name
read -p "Enter your Google Cloud Project ID: " project_id
read -p "Enter the directory to save the JSON key file (leave empty for current directory): " key_output_dir
read -p "Enter the path to your BigQuery table schema JSON file (e.g., ./header.json): " schema_file_path

# --- Handle empty input ---
if [ -z "$sa_name" ]; then
    echo "Error: Service account ID cannot be empty. Exiting."
    exit 1
fi
if [ -z "$key_output_dir" ]; then
    key_output_dir="."
    echo "No key output directory specified. Key file will be saved in the current directory."
fi

if [ -z "$schema_file_path" ]; then
    schema_file_path="./header.json"
fi

# --- Main Script Logic ---

# Define derived names
table_name="${sa_name}" # BigQuery table name will be same as SA ID
topic_name="${sa_name}_${major}" # Pub/Sub topic name
subscription_id="${sa_name}_${major}_bq_sub" # Pub/Sub subscription name

# Full path
bigquery_full_table_path="${project_id}:${dataset_name}.${table_name}"
temp_key_file="${key_output_dir}/${sa_name}.json"

echo "Starting Google Cloud resource setup for project: ${project_id}"

# --- 1. Create the Service Account ---
echo "Creating service account: ${sa_name}..."
display_name="${sa_name} Service Account"
description="Service account for ${sa_name} for Pub/Sub and BigQuery subscription in project ${project_id}"

gcloud iam service-accounts create "${sa_name}" \
    --display-name="${display_name}" \
    --description="${description}" \
    --project="${project_id}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to create service account '${sa_name}'. Exiting."
    exit 1
fi
echo "Service account '${sa_name}' created successfully."

service_account_email="${sa_name}@${project_id}.iam.gserviceaccount.com"

# --- 2. Grant Pub/Sub Publisher Role to the Service Account ---
echo "Granting Pub/Sub Publisher role to ${service_account_email} in project: ${project_id}..."
gcloud projects add-iam-policy-binding "${project_id}" \
    --member="serviceAccount:${service_account_email}" \
    --role="roles/pubsub.publisher"

if [ $? -ne 0 ]; then
    echo "Error: Failed to grant Pub/Sub Publisher role. Exiting."
    gcloud iam service-accounts delete "${service_account_email}" --project="${project_id}" --quiet
    exit 1
fi
echo "Pub/Sub Publisher role granted successfully."


# --- 3. Create JSON Service Account Key ---
echo "Creating JSON key for service account: ${service_account_email}..."
mkdir -p "${key_output_dir}" # Ensure the output directory exists

gcloud iam service-accounts keys create "${temp_key_file}" \
    --iam-account="${service_account_email}" \
    --project="${project_id}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to create JSON key. Exiting."
    exit 1
fi
echo "JSON key created successfully at: ${temp_key_file}"

# --- 4. Create BigQuery Dataset if it doesn't exist ---
echo "Checking if BigQuery dataset '${dataset_name}' exists in project '${project_id}'..."
bq show --dataset "${project_id}:${dataset_name}" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo "Dataset '${dataset_name}' already exists."
else
    echo "Dataset '${dataset_name}' does not exist. Creating it..."
    bq mk --dataset --project_id="${project_id}" "${dataset_name}"
    if [ $? -ne 0 ]; then
        echo "Error: Failed to create dataset '${dataset_name}'. Exiting."
        exit 1
    fi
    echo "Dataset '${dataset_name}' created successfully."
fi

# --- 5. Create Empty BigQuery Table using Schema File ---
echo "Creating empty BigQuery table '${table_name}' in dataset '${dataset_name}' using schema from '${schema_file_path}'..."

# The bq mk command is used to create a table from a JSON schema file.
bq mk \
    --table \
    --project_id="${project_id}" \
    "${dataset_name}.${table_name}" \
    "${schema_file_path}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to create BigQuery table '${table_name}'. Please ensure the schema file is valid."
    echo "You may need to manually inspect the schema file and your permissions."
    exit 1
fi
echo "BigQuery table '${table_name}' created successfully in dataset '${dataset_name}'."


# --- 6. Create Pub/Sub Topic ---
echo "Creating Pub/Sub topic: ${topic_name}..."
gcloud pubsub topics create "${topic_name}" --project="${project_id}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to create Pub/Sub topic '${topic_name}'. Exiting."
    exit 1
fi
echo "Pub/Sub topic '${topic_name}' created successfully."

# --- 7. Create BigQuery Subscription ---
echo "Creating BigQuery subscription '${subscription_id}' for topic '${topic_name}'..."

gcloud pubsub subscriptions create "${subscription_id}" \
    --topic="${topic_name}" \
    --bigquery-table="${project_id}:${dataset_name}.${table_name}" \
    --use-table-schema \
    --project="${project_id}"

if [ $? -ne 0 ]; then
    echo "Error: Failed to create BigQuery subscription '${subscription_id}'. Exiting."
    exit 1
fi
echo "BigQuery subscription '${subscription_id}' created successfully, writing to ${bigquery_full_table_path}."


echo "Script finished successfully."
echo "--------------------------------------------------------------------------------"
