#!/bin/bash

# Default server address if not provided
SERVER=${1:-"http://localhost:9990"}

# Function to create a feature flag
create_flag() {
    local store=$1
    local key=$2
    local value=$3
    local expression=$4

    echo "Creating flag: $store/$key"
    # Create a properly escaped JSON payload
    local payload=$(jq -n \
        --arg value "$value" \
        --arg expression "$expression" \
        '{"value": $value, "expression": $expression}')
    
    curl -X PUT "$SERVER/$store/$key" \
        -H "Content-Type: application/json" \
        -d "$payload"
    echo -e "\n"
}

# Example 1: IP-based feature flag
create_flag "demo" "internal_access" "enabled" \
    "request.ip.startsWith('10.') || request.ip.startsWith('192.168.') ? \"enabled\" : \"disabled\""

# Example 2: Device-based feature flag
create_flag "demo" "mobile_optimized" "enabled" \
    "device.mobile ? \"enabled\" : \"disabled\""

# Example 3: Browser-based feature flag
create_flag "demo" "chrome_only" "enabled" \
    "device.browser.name == \"Chrome\" ? \"enabled\" : \"disabled\""

# Example 4: Time-based feature flag
create_flag "demo" "business_hours" "enabled" \
    "time.getHours() >= 9 && time.getHours() < 17 ? \"enabled\" : \"disabled\""

# Example 5: Bot detection
create_flag "demo" "human_only" "enabled" \
    "!device.bot ? \"enabled\" : \"disabled\""

# Example 6: OS-specific feature
create_flag "demo" "macos_feature" "enabled" \
    "device.os == \"macOS\" ? \"enabled\" : \"disabled\""

# Example 7: Browser version check
create_flag "demo" "modern_browser" "enabled" \
    "device.browser.name == \"Chrome\" && device.browser.version >= \"100\" ? \"enabled\" : \"disabled\""

# Example 8: Complex device targeting
create_flag "demo" "premium_mobile" "enabled" \
    "device.mobile && device.os == \"iOS\" && device.browser.name == \"Safari\" ? \"enabled\" : \"disabled\""

# Example 9: Time-based with device targeting
create_flag "demo" "mobile_promo" "enabled" \
    "device.mobile && time.getHours() >= 18 && time.getHours() < 22 ? \"enabled\" : \"disabled\""

# Example 10: IP range with device type
create_flag "demo" "corporate_devices" "enabled" \
    "request.ip.startsWith('172.16.') && !device.mobile ? \"enabled\" : \"disabled\""

echo "All feature flags have been created successfully!" 