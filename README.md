# Yakusoku Ledger

Yakusoku Ledger is a Hyperledger Fabric prototype for recording financial
agreements between students and universities. It provides an immutable agreement
history, email-based lookup, a two-organization Fabric network, and a REST API for
network and chaincode operations.

Originally built at CUHackit 2020, the project has been completed and renamed to
**Yakusoku**, the Japanese word for "promise," to reflect the agreements it records.

## Architecture

- **University organization:** two Fabric peers and one certificate authority
- **Student organization:** two Fabric peers and one certificate authority
- **Ledger:** Go chaincode backed by CouchDB
- **API:** Node.js/Express service using the bundled Fabric 1.4.5 SDK
- **Authentication:** enrollment endpoint issuing Bearer JWTs

The chaincode stores a deterministic agreement ID derived from the student and
university names. Updating the same agreement preserves its prior values in Fabric's
key history.

## Prerequisites

This is a legacy Hyperledger Fabric 1.2 application. Use a Linux host or WSL 2 with:

- Docker Engine with the Compose v2 plugin
- Node.js 8.9 or 10.15 (supported by the bundled Fabric 1.4.5 SDK)
- Bash, curl, and jq

The repository includes the Fabric 1.2 command-line binaries used to generate network
artifacts and the Node dependencies needed by the API. They are Linux executables;
no package installation command is required.

## Start the project

1. Generate the crypto material and start the Fabric network:

   ```bash
   cd fabric
   ./start.sh
   ```

2. Start the API in another terminal:

   ```bash
   cd node1
   export JWT_SECRET="$(openssl rand -hex 32)"
   export ADMIN_ENROLLMENT_SECRET="$(openssl rand -hex 32)"
   node server.js
   ```

3. Confirm that the API is available:

   ```bash
   curl http://localhost:4000/health
   ```

4. Run the complete enrollment, channel, deployment, invoke, and query workflow:

   ```bash
   cd node1
   ./testAPI.sh
   ```

5. Stop the network:

   ```bash
   cd fabric
   ./stop.sh
   ```

`fabric/generate-artifacts.sh` can be run directly to replace all generated
certificates and channel artifacts. Private keys and generated artifacts are ignored
by Git.

## REST API

Except for `GET /health` and `POST /users`, requests require:

```text
Authorization: Bearer <token returned by POST /users>
```

Channel creation, peer joining, installation, and instantiation additionally require an
administrator token. Supply the server's `ADMIN_ENROLLMENT_SECRET` as `adminSecret`
when enrolling the network administrator. Do not expose this secret to normal users.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/users` | Register/enroll a user and issue a token |
| `POST` | `/channels` | Create a channel |
| `POST` | `/channels/:channel/peers` | Join organization peers |
| `POST` | `/chaincodes` | Install chaincode |
| `POST` | `/channels/:channel/chaincodes` | Instantiate chaincode |
| `POST` | `/channels/:channel/chaincodes/:name` | Invoke chaincode |
| `GET` | `/channels/:channel/chaincodes/:name` | Query chaincode |
| `GET` | `/channels/:channel/blocks/:number` | Read a block |
| `GET` | `/channels/:channel/blocks?hash=...` | Read a block by hash |
| `GET` | `/channels/:channel/transactions/:id` | Read a transaction |
| `GET` | `/channels/:channel` | Read channel information |
| `GET` | `/chaincodes` | List installed or instantiated chaincode |
| `GET` | `/channels` | List peer channels |

The runnable `node1/testAPI.sh` script contains request examples for the full workflow.

## Chaincode functions

| Function | Arguments | Behavior |
| --- | --- | --- |
| `Init` / `initStudentUniversity` | student name, email, date, amount, university name | Create or update an agreement |
| `queryByStudentEmail` | email | Find agreements for an email |
| `getHistoryForStudent` | student name, university name | Return agreement history |
| `invokeFunctionStudentUniversity` | agreement ID | Read an agreement by ID |

## Project layout

```text
fabric/                 Fabric network, Docker Compose, and artifact scripts
node1/                  REST API and Fabric Node SDK integration
src/chaincode/          Student-university agreement chaincode
```

## Configuration

- `HOST` and `PORT` override the API listener.
- `JWT_SECRET` sets the signing secret. If omitted, a random development secret is
  created on startup.
- `ADMIN_ENROLLMENT_SECRET` authorizes creation of administrator tokens. Without it,
  privileged network-management routes remain unavailable.
- `KEY_VALUE_STORE` overrides the Fabric client credential store.
- `TARGET_NETWORK` selects an alternate `node1/app/network-config-<name>.json`.

## License

Apache-2.0
