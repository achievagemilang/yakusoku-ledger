# Privacy architecture

## Problem

Earlier Yakusoku Ledger versions wrote student names and email addresses into public
channel state. Every channel peer and ledger backup could retain those values
indefinitely. Encryption alone would not provide deletion because ciphertext and its
history remain immutable and keys can be copied.

Chaincode v4 separates agreement identity from public business metadata. It uses
Fabric private data collections for selective disclosure and keeps only a salted
commitment in public state.

## Data classification

| Data | Classification | Storage |
| --- | --- | --- |
| Student name and email | Direct personal data | `agreementPIICollection` only |
| Per-agreement random salt | Confidential security material | Private collection only |
| Salted student commitment | Pseudonymous identifier | Public channel state |
| Agreement reference, university, date, value, currency, status | Shared agreement metadata | Public channel state |
| Document SHA-256 | Pseudonymous integrity metadata | Public channel state |
| Source agreement document | Confidential document | Browser only; never uploaded |
| Fabric identity and transaction ID | Audit/security metadata | Public channel state |

Agreement values, dates, and document hashes can still be sensitive when combined
with outside information. Production deployments should minimize channel membership
and evaluate whether those fields require additional collections.

## Design

`agreementPIICollection` explicitly includes `StudentMSP` and `UniversityMSP`.
Organizations added to the channel later do not receive its private data unless the
collection definition is deliberately upgraded. Chaincode also rejects PII reads from
any other MSP.

The API generates a cryptographically random 32-byte salt for each agreement. It sends
this JSON only in the Fabric proposal transient map:

```json
{
  "StudentName": "aiko tanaka",
  "Email": "aiko@example.edu",
  "Salt": "<64 lowercase hexadecimal characters>"
}
```

Chaincode stores that object in the private collection. Public state stores:

```text
SHA-256(hex-salt + ":" + normalized-email)
```

The random private salt prevents practical dictionary matching of common email
addresses against the public commitment. Creation events contain only the public
record, and the API no longer logs chaincode query payloads.

Authorized users can verify an email without putting it in a block. The API sends the
candidate email as transient data; chaincode recomputes and constant-time compares the
commitment, then returns only `verified`.

If a peer is temporarily missing a private collection entry, registry queries still
return the public agreement with private fields omitted rather than failing the entire
result set. Identity verification remains fail-closed until that peer receives or
recovers the private value.

## Threat model

This design protects direct student PII from:

- orderers and block inspection;
- channel members outside the collection;
- public-state exports and public transaction history;
- accidental disclosure through chaincode events or API query logs.

It does not protect against:

- a compromised Student or University peer, application, administrator, or backup;
- an authorized user copying data after disclosure;
- traffic inspection when TLS is disabled;
- inference from public agreement metadata;
- plaintext PII already committed by chaincode v3 or earlier.

The collection policy controls data distribution, not business endorsement. A
production network must still require both organizations to endorse protected writes,
as tracked in issue #12.

## Retention and deletion

The collection uses `blockToLive: 0`, so current private values do not expire. This is
intentional until institutions define a retention period that will not break active
agreement workflows. Fabric private-data purge does not remove public hashes, public
history, copied data, snapshots, logs, or backups.

Changing or deleting a current private value therefore cannot guarantee erasure.
Production policy must define retention periods, backup expiration, access review,
legal holds, and the consequences of an unavailable private record. Do not claim that
Fabric provides a right-to-erasure mechanism.

## Legacy migration

Upgrading does not rewrite old blocks. For each current plaintext agreement, a Student
organization administrator calls the privacy migration endpoint with the matching name
and email. Chaincode:

1. verifies the transient details against the current legacy record;
2. creates a fresh salt and private collection entry;
3. writes the salted commitment to current public state;
4. omits the name and email from the new public version.

Old public versions still contain the original PII and remain visible through block
and history APIs. A network containing real legacy PII needs a governance and legal
decision; technical migration cannot retroactively make that ledger private.
