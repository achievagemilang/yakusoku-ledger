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

The decision must be `approved` or `rejected`, and the Fabric creator must belong to
`UniversityMSP`.

### Read history

```http
GET /api/agreements/:agreementId/history
```

Entries are returned in Fabric commit order and include the transaction ID, timestamp,
delete marker, and complete agreement value at that version.

## Errors

Validation errors use HTTP 400, missing or invalid tokens use 401, authorization
failures use 403, and Fabric/SDK failures use 500 with a JSON `message`.
