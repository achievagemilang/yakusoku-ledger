# Invitation and identity governance

## Problem

Shared organization enrollment secrets grant the same reusable credential to every
prospective member. They cannot expire per user, prove which administrator granted
access, or revoke one person without rotating access for everyone.

## Design

Yakusoku administrators now issue random, single-use invitations. Only the SHA-256
token hash is persisted in `node1/tmp/identity-governance.json`; the plaintext token is
returned once. Invitations have an organization, certificate role, issuer, expiry,
and lifecycle state.

The first organization administrator is bootstrapped locally:

```bash
cd node1
node app/invitation-cli.js create org1 organization_admin 60
node app/invitation-cli.js create org2 organization_admin 60
```

Running this command on the API host avoids a network bootstrap secret. The
administrator uses the returned token once in the normal enrollment form, then creates
member invitations from the dashboard or API.

During enrollment the API claims the invitation before contacting Fabric CA. The CA
registration has one permitted enrollment and embeds these signed certificate
attributes:

- `yakusoku.role`
- `yakusoku.invitation`

The claim becomes `used` only after enrollment succeeds and is released after a failed
CA operation. Chaincode requires the role attribute for agreement reads and writes:

| Organization | Permitted roles |
| --- | --- |
| `StudentMSP` | `student`, `organization_admin` |
| `UniversityMSP` | `university_reviewer`, `organization_admin` |

## Revocation

An administrator can revoke an unused invitation or an active member in their own
organization. Member revocation:

1. asks Fabric CA to revoke the enrollment identity;
2. marks the member revoked in the governance registry;
3. removes the local wallet entry;
4. causes API middleware to reject existing tokens immediately.

Production peers must also receive updated CA revocation lists so a revoked
certificate cannot submit proposals outside this API.

## Audit history

The registry records invitation creation, claim, release, use, revocation, reissue,
member enrollment, and member revocation events. Events include timestamps,
organization, actor or subject, and invitation ID without storing plaintext tokens.

## Persistence and recovery

Set `IDENTITY_GOVERNANCE_STORE` to place the registry on durable encrypted storage.
Back up this file with the Fabric wallet and CA database. Writes use a temporary file
and atomic rename. A production database migration belongs with the deployment work
tracked separately.

## Edge cases

- Used, expired, claimed, or revoked invitations cannot be reused.
- Invitation roles are restricted by organization.
- Claims prevent concurrent enrollment with one token.
- Administrators cannot revoke their currently authenticated identity.
- Revoked identities cannot use an unexpired API token.
- Existing pre-invitation wallet identities must be re-enrolled through an invitation
  to receive the required certificate role.
