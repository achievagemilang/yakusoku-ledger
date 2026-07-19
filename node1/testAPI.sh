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

chaincode='{"peers":["peer1","peer2"],"chaincodeName":"studentuniversity","chaincodePath":"chaincode","chaincodeVersion":"v5"}'
request "$org1_token" POST /chaincodes "$chaincode"
request "$org2_token" POST /chaincodes "$chaincode"

request "$org1_token" POST /channels/channel1/chaincodes \
	'{"peers":["peer1","peer2"],"chaincodeName":"studentuniversity","chaincodeVersion":"v5","fcn":"Init","args":[]}'

document_hash=$(printf 'yakusoku sample agreement' | sha256sum | cut -d ' ' -f1)
create_response=$(request "$org2_token" POST /api/agreements \
	"{\"studentName\":\"Ada Lovelace\",\"email\":\"ada@example.com\",\"date\":\"2026-08-01\",\"expiresOn\":\"2027-08-01\",\"amount\":\"24000\",\"currency\":\"USD\",\"universityName\":\"Clemson University\",\"documentHash\":\"$document_hash\"}")
echo "$create_response"

second_create_response=$(request "$org2_token" POST /api/agreements \
	"{\"studentName\":\"Ada Lovelace\",\"email\":\"ada@example.com\",\"date\":\"2027-08-01\",\"expiresOn\":\"2028-08-01\",\"amount\":\"25000.50\",\"currency\":\"USD\",\"universityName\":\"Clemson University\",\"documentHash\":\"$document_hash\"}")
echo "$second_create_response"

agreements=$(request "$org2_token" GET /api/agreements)
agreement_count=$(printf '%s' "$agreements" | jq -er '[.[] | select(.Value.Email == "ada@example.com")] | length')
if [ "$agreement_count" -ne 2 ]; then
	echo "Expected two agreements for the same student/university pair, got $agreement_count"
	exit 1
fi
agreement_id=$(printf '%s' "$agreements" | jq -er '[.[] | select(.Value.Email == "ada@example.com")][0].Key')
history=$(request "$org2_token" GET "/api/agreements/$agreement_id/history")
printf '%s' "$history" | jq -e '
	.[0].Value
	| has("StudentName") == false
	and has("Email") == false
	and (.StudentCommitment | test("^[a-f0-9]{64}$"))
	' >/dev/null
identity_response=$(request "$org2_token" POST "/api/agreements/$agreement_id/identity/verify" \
	'{"email":"ada@example.com"}')
printf '%s' "$identity_response" | jq -e '.verified == true' >/dev/null
request "$org2_token" POST "/api/agreements/$agreement_id/verify" \
	"{\"documentHash\":\"$document_hash\"}"
request "$org2_token" POST "/api/agreements/$agreement_id/sign" '{}'
request "$org1_token" POST "/api/agreements/$agreement_id/review" \
	'{"decision":"approved"}'
active=$(request "$org1_token" GET "/api/agreements/$agreement_id")
printf '%s' "$active" | jq -e '
	.Status == "active"
	and (.StudentSignedBy | length > 0)
	and (.UniversitySignedBy | length > 0)
	and .Revision == 1
	' >/dev/null

request "$org2_token" POST "/api/agreements/$agreement_id/amendments" \
	"{\"date\":\"2026-09-01\",\"expiresOn\":\"2027-09-01\",\"amount\":\"26000.75\",\"currency\":\"USD\",\"documentHash\":\"$document_hash\"}"
request "$org1_token" POST "/api/agreements/$agreement_id/amendments/decision" \
	'{"decision":"approved"}'
amended=$(request "$org1_token" GET "/api/agreements/$agreement_id")
printf '%s' "$amended" | jq -e '
	.Status == "active"
	and .Revision == 2
	and .AmountMinor == 2600075
	and .Amendments[-1].State == "applied"
	and .Amendments[-1].SupersededRevision == 1
	' >/dev/null

echo "Yakusoku Ledger API workflow completed successfully."
