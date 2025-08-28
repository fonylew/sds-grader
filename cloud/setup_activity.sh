#!/bin/bash

# --- BigQuery Dataset Configuration ---
# IMPORTANT: Update these variables with your actual class/major/semester/year
class_id="2110415"    # e.g., "2110XXX"
majors=("CEDT" "CP")  # Array of majors to set up for
semester="1"          # e.g., "1 or 2"
year="2025"           # e.g., "2025"

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
table_name="${sa_name}" # BigQuery table name will be the same as the activity ID
temp_key_file="${key_output_dir}/${sa_name}.json"

echo "Starting Google Cloud resource setup for project: ${project_id}"

# --- 1. Create the Service Account ---
service_account_email="${sa_name}@${project_id}.iam.gserviceaccount.com"
echo "Checking for service account: ${service_account_email}..."

gcloud iam service-accounts describe "${service_account_email}" --project="${project_id}" > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "Service account '${sa_name}' already exists. Skipping creation."
else
    echo "Service account not found. Creating it..."
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
fi

# --- 2. Grant Pub/Sub Publisher Role to the Service Account ---
role="roles/pubsub.publisher"
member="serviceAccount:${service_account_email}"
echo "Checking for Pub/Sub Publisher role for ${service_account_email}..."

# Check if the binding already exists
existing_binding=$(gcloud projects get-iam-policy "${project_id}" \
  --flatten="bindings" \
  --filter="bindings.role=${role} AND bindings.members:${member}" \
  --format="value(bindings.role)")

if [ -n "$existing_binding" ]; then
    echo "Pub/Sub Publisher role is already granted to ${service_account_email}."
else
    echo "Granting Pub/Sub Publisher role to ${service_account_email}..."
    gcloud projects add-iam-policy-binding "${project_id}" \
        --member="${member}" \
        --role="${role}" \
        --condition=None > /dev/null

    if [ $? -ne 0 ]; then
        echo "Error: Failed to grant Pub/Sub Publisher role. Exiting."
        exit 1
    fi
    echo "Pub/Sub Publisher role granted successfully."
fi

# --- 3. Create JSON Service Account Key ---
if [ -f "${temp_key_file}" ]; then
    echo "JSON key file '${temp_key_file}' already exists. Skipping key creation."
else
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
fi

# --- Loop through each major to create resources ---
for major in "${majors[@]}"; do
    echo ""
    echo "================================================================================"
    echo "Setting up resources for major: ${major}"
    echo "================================================================================"

    # Construct names specific to this major
    dataset_name="${class_id}_${major}_${semester}_${year}"
    topic_name="${sa_name}_${major}"
    subscription_id="${sa_name}_${major}_bq_sub"
    bigquery_full_table_path="${project_id}:${dataset_name}.${table_name}"

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
    echo "Checking for BigQuery table '${table_name}' in dataset '${dataset_name}'..."
    bq show "${project_id}:${dataset_name}.${table_name}" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Table '${table_name}' already exists in dataset '${dataset_name}'. Skipping creation."
    else
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
    fi

    # --- 6. Create Pub/Sub Topic ---
    echo "Checking for Pub/Sub topic: ${topic_name}..."
    gcloud pubsub topics describe "${topic_name}" --project="${project_id}" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Topic '${topic_name}' already exists. Skipping creation."
    else
        echo "Creating Pub/Sub topic: ${topic_name}..."
        gcloud pubsub topics create "${topic_name}" --project="${project_id}"
        if [ $? -ne 0 ]; then
            echo "Error: Failed to create Pub/Sub topic '${topic_name}'. Exiting."
            exit 1
        fi
        echo "Pub/Sub topic '${topic_name}' created successfully."
    fi

    # --- 7. Create BigQuery Subscription ---
    echo "Checking for Pub/Sub subscription: ${subscription_id}..."
    gcloud pubsub subscriptions describe "${subscription_id}" --project="${project_id}" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "Subscription '${subscription_id}' already exists. Skipping creation."
    else
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
    fi
done

echo ""
echo "Script finished successfully."
echo "--------------------------------------------------------------------------------"
