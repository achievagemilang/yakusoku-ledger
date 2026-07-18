#!/usr/bin/env bash
set -euo pipefail

command -v jq >/dev/null || {
	echo "jq is required: https://jqlang.github.io/jq/"
	exit 1
}
: "${ADMIN_ENROLLMENT_SECRET:?Set ADMIN_ENROLLMENT_SECRET to the value used by the API}"
: "${UNIVERSITY_ENROLLMENT_SECRET:?Set UNIVERSITY_ENROLLMENT_SECRET to the value used by the API}"

api=http://localhost:4000

enroll() {
	curl --fail --silent --show-error -X POST "$api/users" \
		-H "content-type: application/json" \
		-d "{\"username\":\"$1\",\"orgName\":\"$2\",\"adminSecret\":\"$ADMIN_ENROLLMENT_SECRET\",\"organizationSecret\":\"$3\"}"
}

org1_response=$(enroll network-admin-org1 org1 "$UNIVERSITY_ENROLLMENT_SECRET")
org2_response=$(enroll network-admin-org2 org2 "")
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

chaincode='{"peers":["peer1","peer2"],"chaincodeName":"studentuniversity","chaincodePath":"chaincode","chaincodeVersion":"v3"}'
request "$org1_token" POST /chaincodes "$chaincode"
request "$org2_token" POST /chaincodes "$chaincode"

request "$org1_token" POST /channels/channel1/chaincodes \
	'{"peers":["peer1","peer2"],"chaincodeName":"studentuniversity","chaincodeVersion":"v3","fcn":"Init","args":["AGR-2026-000000000001","Genesis Student","genesis@example.com","2026-07-18","1","JPY","Clemson University",""]}'

document_hash=$(printf 'yakusoku sample agreement' | sha256sum | cut -d ' ' -f1)
create_response=$(request "$org2_token" POST /api/agreements \
	"{\"studentName\":\"Ada Lovelace\",\"email\":\"ada@example.com\",\"date\":\"2026-08-01\",\"amount\":\"24000\",\"currency\":\"USD\",\"universityName\":\"Clemson University\",\"documentHash\":\"$document_hash\"}")
echo "$create_response"

second_create_response=$(request "$org2_token" POST /api/agreements \
	"{\"studentName\":\"Ada Lovelace\",\"email\":\"ada@example.com\",\"date\":\"2027-08-01\",\"amount\":\"25000.50\",\"currency\":\"USD\",\"universityName\":\"Clemson University\",\"documentHash\":\"$document_hash\"}")
echo "$second_create_response"

agreements=$(request "$org2_token" GET /api/agreements)
agreement_count=$(printf '%s' "$agreements" | jq -er '[.[] | select(.Value.Email == "ada@example.com")] | length')
if [ "$agreement_count" -ne 2 ]; then
	echo "Expected two agreements for the same student/university pair, got $agreement_count"
	exit 1
fi
agreement_id=$(printf '%s' "$agreements" | jq -er '[.[] | select(.Value.Email == "ada@example.com")][0].Key')
request "$org2_token" POST "/api/agreements/$agreement_id/verify" \
	"{\"documentHash\":\"$document_hash\"}"
request "$org1_token" POST "/api/agreements/$agreement_id/review" \
	'{"decision":"approved"}'
request "$org1_token" GET "/api/agreements/$agreement_id"

echo "Yakusoku Ledger API workflow completed successfully."
