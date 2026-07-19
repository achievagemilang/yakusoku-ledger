# Yakusoku Ledger features

## Student workflow

1. Enroll a Student organization identity.
2. Enter the agreement details and choose the supporting document.
3. The browser calculates the document's SHA-256 fingerprint locally.
4. The API sends the student name, email, and random salt as Fabric transient data.
5. Peers store those fields in the member-only agreement PII collection; public state
   receives only a salted commitment.
6. Sign the draft and track its university countersignature in the dashboard.

Each submission receives a stable `AGR-<year>-<random>` reference, so the same student
and university can create separate agreements for different terms or programs.

The source document is never uploaded to Yakusoku Ledger. A later copy can be selected
in the verification panel; its local fingerprint is compared with the immutable value.

Student and University organization peers can resolve private student details for
authorized dashboard queries. Orderers, channel blocks, and organizations outside the
collection receive only the salted identity commitment. See
[Privacy architecture](PRIVACY.md) for the threat model and retention limitations.

## University workflow

University organization members enrolled with `UNIVERSITY_ENROLLMENT_SECRET` see
submitted agreements in the review queue. A countersignature or rejection creates a new ledger version rather than replacing
history. The chaincode independently verifies that the signer belongs to
`UniversityMSP`.

## Agreement lifecycle

New agreements progress from draft to student-signed, university countersigned, and
active states. Both Fabric identity fingerprints and transaction timestamps are
recorded. Active terms can change only through an amendment approved by both
organizations. Applied amendments identify the superseded and activated revisions.
See [Agreement lifecycle and amendments](AGREEMENT_LIFECYCLE.md).

## Dashboard and analytics

The dashboard calculates these values from live ledger records:

- total agreements
- pending approvals
- approved agreement value
- agreements carrying verified document fingerprints

Agreement values are stored as integer minor units with an explicit currency. The
dashboard formats each record in its own currency and does not combine unlike
currencies into a misleading total.

Before authentication, clearly marked preview records demonstrate the product without
pretending that sample data came from Fabric.

## Audit history and notifications

Every agreement row opens its complete Fabric key history, including transaction ID,
timestamp, status, and value. Successful enrollment, submission, review, and document
checks also appear in the activity feed as browser notifications.

## Roles

| Identity | Capabilities |
| --- | --- |
| Student organization member | Submit, browse, verify, and audit agreements |
| University organization member | All read operations plus approve/reject |
| Network administrator | Channel, peer, chaincode installation, and instantiation |

The network administrator secret only grants API lifecycle permissions. Agreement
review authorization is enforced from the transaction creator's Fabric MSP.
