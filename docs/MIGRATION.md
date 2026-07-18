# Chaincode v3 data migration

Chaincode v3 changes agreement identity and money representation while preserving
read compatibility with existing records.

## New records

- The ledger key is a unique reference such as `AGR-2026-12AB34CD56EF`.
- `AmountMinor` is a positive integer in the currency's minor units.
- `Currency` is an explicit supported ISO 4217 code.
- Multiple references may identify agreements between the same student and university.

## Existing records

Earlier records remain at their original SHA-256 keys and keep their legacy numeric
`Amount`. Query, detail, verification, review, and history functions continue to
unmarshal these records. The dashboard treats a missing currency as JPY and converts
the legacy value for display only.

No automatic ledger rewrite is performed. Rewriting immutable records would obscure
their original history.

## Upgrade procedure

1. Back up the ledger and CouchDB state.
2. Install chaincode version `v3` on every peer in both organizations using the
   Fabric 1.x `peer chaincode install` command.
3. From a configured administrator CLI, upgrade the existing channel instance using
   the Fabric 1.x lifecycle:

   ```bash
   peer chaincode upgrade \
     -o orderer.clemson.com:7050 \
     -C channel1 \
     -n studentuniversity \
     -v v3 \
     -c '{"Args":["Init","AGR-2026-000000000001","Genesis Student","genesis@example.com","2026-07-18","1","JPY","Clemson University",""]}' \
     -P "OR('UniversityMSP.peer','StudentMSP.peer')" \
     --tls \
     --cafile /opt/gopath/channel-artifacts/certs/ordererOrganizations/clemson.com/orderers/orderer.clemson.com/tls/ca.crt
   ```

   This repository uses the legacy Fabric 1.x install/upgrade lifecycle, not Fabric
   2.x definition approval.
4. Run queries against representative legacy records.
5. Submit two v3 agreements for the same student/university pair.
6. Confirm their references differ and their `AmountMinor`/`Currency` values are exact.
7. Keep the old chaincode package available for rollback.

Integrations invoking `createAgreement` directly must switch to these eight arguments:

```text
reference, student name, email, date, amount minor units, currency,
university name, document SHA-256
```
