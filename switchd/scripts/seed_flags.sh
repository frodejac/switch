#!/bin/bash

# Default server address if not provided
SERVER=${1:-"http://localhost:9990"}

# Function to create a feature flag
create_flag() {
    local store=$1
    local key=$2
    local value=$3
    local type=$4

    echo "Creating flag: $store/$key"
    # Create a properly escaped JSON payload
    local payload
    if [ "$type" != "cel" ]; then
      payload=$(jq -n \
        --argjson value "$value" \
        --arg type "$type" \
        '{"value": $value, "type": $type}')
    else
      payload=$(jq -n \
        --arg value "$value" \
        --arg type "$type" \
        '{"value": $value, "type": $type}')
    fi
    
    curl -X PUT "$SERVER/$store/$key" \
        -H "Content-Type: application/json" \
        -d "$payload"
    echo -e "\n"
}

# Example 1: IP-based feature flag
create_flag "demo" \
  "internal_access" \
  "request.ip.startsWith('10.') || request.ip.startsWith('192.168.') ? \"enabled\" : \"disabled\"" \
  "cel"

# Example 2: Device-based feature flag
create_flag "demo" \
  "mobile_optimized" \
  'device.mobile ? "enabled" : "disabled"' \
  "cel"

# Example 3: Browser-based feature flag
create_flag "demo" \
  "chrome_only" \
  'device.browser.name == "Chrome" ? "enabled" : "disabled"' \
  "cel"

# Example 4: Time-based feature flag
create_flag "demo" \
  "business_hours" \
  'time.getHours() >= 9 && time.getHours() < 17 ? "enabled" : "disabled"' \
  "cel"

# Example 5: Boolean flag
create_flag "demo" \
  "feature_enabled" \
  true \
  "boolean"

# Example 6: String flag
create_flag "demo" \
  "feature_name" \
  "\"New Feature\"" \
  "string"

# Example 7: Int flag
create_flag "demo" \
  "default_page_size" \
  100 \
  "int"

# Example 8: Float flag
create_flag "demo" \
  "feature_ratio" \
  0.7 \
  "float"

# Example 9: JSON value
create_flag "demo" \
  "feature_payload" \
  '{"key": "value"}' \
  "json"

echo "All feature flags have been created successfully!"