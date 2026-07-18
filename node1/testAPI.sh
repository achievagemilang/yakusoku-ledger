#!/usr/bin/env bash
set -euo pipefail

command -v jq >/dev/null || {
	echo "jq is required: https://jqlang.github.io/jq/"
	exit 1
}
: "${ADMIN_ENROLLMENT_SECRET:?Set ADMIN_ENROLLMENT_SECRET to the value used by the API}"

api=http://localhost:4000

enroll() {
	curl --fail --silent --show-error -X POST "$api/users" \
		-H "content-type: application/json" \
		-d "{\"username\":\"$1\",\"orgName\":\"$2\",\"adminSecret\":\"$ADMIN_ENROLLMENT_SECRET\"}"
}

org1_response=$(enroll network-admin-org1 org1)
org2_response=$(enroll network-admin-org2 org2)
org1_token=$(printf '%s' "$org1_response" | jq -er .token)
org2_token=$(printf '%s' "$org2_response" | jq -er .token)

request() {
	local token=$1
	local method=$2
	local path=$3
	local body=${4:-}
	if [ -n "$body" ]; then
		curl --fail --silent --show-error -X "$method" "$api$path" \
			-H "authorization: Bearer $token" \
			-H "content-type: application/json" \
			-d "$body"
	else
		curl --fail --silent --show-error -X "$method" "$api$path" \
			-H "authorization: Bearer $token"
	fi
	echo
}

request "$org1_token" POST /channels \
	'{"channelName":"channel1","channelConfigPath":"../../fabric/channel-artifacts/channel/channel.tx"}'
sleep 5

request "$org1_token" POST /channels/channel1/peers '{"peers":["peer1","peer2"]}'
request "$org2_token" POST /channels/channel1/peers '{"peers":["peer1","peer2"]}'

chaincode='{"peers":["peer1","peer2"],"chaincodeName":"studentuniversity","chaincodePath":"chaincode","chaincodeVersion":"v1"}'
request "$org1_token" POST /chaincodes "$chaincode"
request "$org2_token" POST /chaincodes "$chaincode"

request "$org1_token" POST /channels/channel1/chaincodes \
	'{"peers":["peer1","peer2"],"chaincodeName":"studentuniversity","chaincodeVersion":"v1","fcn":"Init","args":["Ada Lovelace","ada@example.com","2026-07-18","25000","Clemson University"]}'

request "$org1_token" POST /channels/channel1/chaincodes/studentuniversity \
	'{"peers":["peer1","peer2"],"fcn":"initStudentUniversity","args":["Ada Lovelace","ada@example.com","2026-08-01","24000","Clemson University"]}'

encoded_args=$(jq -rn --arg value '["ada@example.com"]' '$value|@uri')
request "$org1_token" GET \
	"/channels/channel1/chaincodes/studentuniversity?peer=peer1&fcn=queryByStudentEmail&args=$encoded_args"

echo "Yakusoku Ledger API workflow completed successfully."
