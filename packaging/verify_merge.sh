#!/bin/bash
set -e

echo "Verifying jq merge logic..."

# Create dummy example config
cat <<EOF > test_example.json
{
  "device": "/dev/ttyUSB0",
  "baud": 2400,
  "broker": "tcp://localhost:1883",
  "topic": "serial/bridge",
  "new_key": "new_value"
}
EOF

# Create dummy current config
cat <<EOF > test_current.json
{
  "device": "/dev/ttyUSB99",
  "baud": 9600,
  "broker": "tcp://mqtt.local:1883",
  "topic": "my/custom/topic"
}
EOF

# Perform merge
# .[0] is example, .[1] is current. * operator merges, with right side winning.
# So if current has a value, it keeps it. If example has a new key, it's added.
MERGED=$(jq -s '.[0] * .[1]' test_example.json test_current.json)

echo "Merged config:"
echo "$MERGED"

# Verification
DEVICE=$(echo "$MERGED" | jq -r .device)
BAUD=$(echo "$MERGED" | jq -r .baud)
NEW_KEY=$(echo "$MERGED" | jq -r .new_key)

if [[ "$DEVICE" == "/dev/ttyUSB99" ]] && [[ "$BAUD" == "9600" ]] && [[ "$NEW_KEY" == "new_value" ]]; then
    echo "SUCCESS: Merge logic works as expected. Existing values preserved, new keys added."
else
    echo "FAILURE: Merge logic failed."
    echo "Expected device: /dev/ttyUSB99, got: $DEVICE"
    echo "Expected baud: 9600, got: $BAUD"
    echo "Expected new_key: new_value, got: $NEW_KEY"
    exit 1
fi

rm test_example.json test_current.json
