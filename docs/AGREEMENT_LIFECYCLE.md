# Agreement lifecycle and amendments

## Problem

The original workflow treated submission as a complete student action and university
approval as a generic status change. It did not record both signing identities or
prevent an active agreement from being replaced unilaterally.

## Design

Chaincode v5 enforces this lifecycle:

```text
draft -> pending_university -> active -> amendment_proposed -> active
                             \-> rejected                 \-> active (rejected amendment)
active -> expired
```

A Student organization member creates a draft and signs it. A University organization
member then countersigns, activating the agreement, or rejects it. Signatures record
the signer MSP, a SHA-256 fingerprint of the Fabric identity, and the transaction
timestamp. Raw certificate subjects are not added to public state.

An amendment proposal contains a new effective date, expiration date, exact monetary
value, currency, and document fingerprint. Proposing records the proposer's approval
but does not change active terms. Chaincode applies the terms only after the other MSP
also approves. The previous revision is marked as superseded in the append-only
`Amendments` history.

Expiration is deterministic: either member organization may record it only after the
ledger transaction date reaches `ExpiresOn`. Rejected agreements and expired
agreements are terminal.

## Alternatives

- Treat creation as a student signature: simpler, but drafts could not be reviewed
  before commitment.
- Allow the university to edit during approval: rejected because it permits unilateral
  changes.
- Create a new ledger key per revision: useful for very large histories, but it makes
  agreement lookup and compatibility more complex than an embedded revision history.

## Edge cases

- A party cannot sign twice or sign in the wrong state.
- University countersignature requires a student signature.
- Only active agreements accept amendment proposals.
- A proposing organization cannot approve its own amendment twice.
- Active terms remain unchanged while an amendment is pending.
- Effective and expiration dates and exact monetary values are revalidated on every
  proposal.
- Legacy `submitted` and `approved` states remain compatible; new records use the v5
  lifecycle.

## Validation

`node1/testAPI.sh` creates two drafts for one student/university pair, signs and
countersigns one, verifies both identity fingerprints, proposes an amendment from the
Student organization, approves it from the University organization, and verifies that
revision 1 is superseded by active revision 2.
