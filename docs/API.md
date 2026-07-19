# Yakusoku Ledger API

The web dashboard uses the domain endpoints below. All require a token returned by
`POST /users`; lifecycle endpoints additionally require an administrator token.

## Agreements

### List agreements

```http
GET /api/agreements
```

### Submit an agreement

```http
POST /api/agreements
Content-Type: application/json

{
  "studentName": "Aiko Tanaka",
  "email": "aiko@example.edu",
  "date": "2026-07-18",
  "expiresOn": "2027-07-18",
  "amount": "6800.25",
  "currency": "USD",
  "universityName": "Kyoto International University",
  "documentHash": "<64-character SHA-256>"
}
```

The API generates a unique reference such as `AGR-2026-12AB34CD56EF`. Money is
converted to exact integer minor units before it reaches Fabric: `6800.25 USD` becomes
`AmountMinor: 680025` with `Currency: "USD"`.

Supported currencies are AUD, CAD, EUR, GBP, INR, JPY, KRW, and USD.

The API converts `studentName` and `email` into Fabric transient data with a fresh
32-byte salt. They are stored in `agreementPIICollection`, not in transaction
arguments, channel blocks, public state, events, or public agreement history.

### Verify a student identity

```http
POST /api/agreements/:agreementId/identity/verify
Content-Type: application/json

{ "email": "aiko@example.edu" }
```

The email is sent to chaincode through transient data. Chaincode combines it with the
private salt and compares the result with the public salted commitment. The response
contains only the agreement ID and `verified`.

### Migrate legacy PII

```http
POST /api/agreements/:agreementId/privacy/migrate
Content-Type: application/json

{
  "studentName": "Aiko Tanaka",
  "email": "aiko@example.edu"
}
```

This endpoint requires a Student organization administrator. Chaincode verifies that
the supplied details match the current legacy record before moving them into the
private collection and redacting the current public state. Historical blocks remain
unchanged.

### Sign or countersign

```http
POST /api/agreements/:agreementId/sign
```

A Student organization member signs a draft, moving it to `pending_university`. A
University organization member countersigns that state, moving it to `active`.

### Propose an amendment

```http
POST /api/agreements/:agreementId/amendments
Content-Type: application/json

{
  "date": "2026-09-01",
  "expiresOn": "2027-09-01",
  "amount": "7200.00",
  "currency": "USD",
  "documentHash": "<64-character SHA-256>"
}
```

The active terms do not change until the other organization approves.

### Decide an amendment

```http
POST /api/agreements/:agreementId/amendments/decision
Content-Type: application/json

{ "decision": "approved" }
```

The decision may be `approved` or `rejected`.

### Record expiration

```http
POST /api/agreements/:agreementId/expire
```

Chaincode accepts this transition only on or after the agreement's `ExpiresOn` date.

### Verify a document

```http
POST /api/agreements/:agreementId/verify
Content-Type: application/json

{ "documentHash": "<64-character SHA-256>" }
```

The response includes `verified`, the agreement ID, and current status.

### Review an agreement

```http
POST /api/agreements/:agreementId/review
Content-Type: application/json

{ "decision": "approved" }
```

The compatibility review endpoint accepts `approved` or `rejected`. Approval performs
the University countersignature and requires a student-signed agreement.

### Read history

```http
GET /api/agreements/:agreementId/history
```

Entries are returned in Fabric commit order and include the transaction ID, timestamp,
delete marker, and complete agreement value at that version.

## Errors

Validation errors use HTTP 400, missing or invalid tokens use 401, authorization
failures use 403, and Fabric/SDK failures use 500 with a JSON `message`.
