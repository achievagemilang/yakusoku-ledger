# Chaincode v6 identity, lifecycle, and privacy migration

Chaincode v6 retains the v5 lifecycle and requires invitation-issued Fabric
certificate roles for agreement operations.

## New records

- The ledger key is a unique reference such as `AGR-2026-12AB34CD56EF`.
- `AmountMinor` is a positive integer in the currency's minor units.
- `Currency` is an explicit supported ISO 4217 code.
- Multiple references may identify agreements between the same student and university.
- `StudentCommitment` is a salted SHA-256 commitment in public state.
- `StudentName`, `Email`, and the random salt exist only in
  `agreementPIICollection`.

## Existing records

Earlier records remain at their original SHA-256 keys and keep their legacy numeric
`Amount`. Query, detail, verification, review, and history functions continue to
unmarshal these records. The dashboard treats a missing currency as JPY and converts
the legacy value for display only.

Legacy values remain readable until each record is explicitly migrated. Migration
redacts the current public value but cannot alter old blocks or historical versions.
See [Privacy architecture](PRIVACY.md) before migrating real personal data.

## Upgrade procedure

1. Back up the ledger and CouchDB state.
2. Update the existing channel's Application capability to `V1_2`. Fresh artifacts
   generated from `fabric/config/configtx.yaml` already enable it.
3. Install chaincode version `v6` on every peer in both organizations using the
   Fabric 1.x `peer chaincode install` command.
4. From a configured administrator CLI, upgrade the existing channel instance using
   the Fabric 1.x lifecycle:

   ```bash
   peer chaincode upgrade \
     -o orderer.clemson.com:7050 \
     -C channel1 \
     -n studentuniversity \
     -v v6 \
     -c '{"Args":["Init"]}' \
     -P "OR('UniversityMSP.peer','StudentMSP.peer')" \
     --collections-config /opt/gopath/src/chaincode/collections-config.json \
     --tls \
     --cafile /opt/gopath/channel-artifacts/certs/ordererOrganizations/clemson.com/orderers/orderer.clemson.com/tls/ca.crt
   ```

   This repository uses the legacy Fabric 1.x install/upgrade lifecycle, not Fabric
   2.x definition approval.
5. Run queries against representative legacy records.
6. For each legacy record, call
   `POST /api/agreements/:agreementId/privacy/migrate` as a Student organization
   administrator with the matching current name and email.
   Chaincode blocks review of an unmigrated plaintext record to avoid writing its PII
   into another public version or event.
7. Confirm the latest public history value omits `StudentName` and `Email`, contains a
   64-character `StudentCommitment`, and the normal detail/list APIs still resolve PII
   for both collection-member organizations.
8. Create local `organization_admin` bootstrap invitations for both organizations and
   enroll fresh identities. Existing certificates do not contain `yakusoku.role` and
   cannot invoke v6 agreement functions.
9. Submit two v6 agreements for the same student/university pair.
10. Confirm their references differ and their `AmountMinor`/`Currency` values are exact.
11. Keep the old chaincode package available for rollback. A rollback does not restore
    private fields to public state.

Integrations invoking `createAgreement` directly must switch to seven public arguments:

```text
reference, effective date, expiration date, amount minor units, currency,
university name, document SHA-256
```

They must also provide an `agreement_pii` transient-map value containing
`StudentName`, `Email`, and a cryptographically random 32-byte hexadecimal `Salt`.
