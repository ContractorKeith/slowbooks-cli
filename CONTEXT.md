# slowbooks-cli

The words this command-line client for SlowBooks Pro uses. The server's own
accounting terms (invoice, bill, journal entry) mean what they mean in
SlowBooks Pro and are not redefined here.

## Language

**Server**:
One running SlowBooks Pro install the CLI talks to, named by its URL.
_Avoid_: Instance, host, backend

**Company**:
One set of books on a server, chosen per request with the `X-SlowBooks-Company` header.
_Avoid_: Company file, database, tenant

**Token**:
An `sbp_` API token made in SlowBooks Pro Settings, with one role (bookkeeper, accountant or readonly).
_Avoid_: API key, password, credential

**Profile**:
A saved server URL plus one token, with an optional default company.
_Avoid_: Account, connection, context

**Operation**:
One method and path from the server's `/openapi.json`, such as `POST /api/payments`.
_Avoid_: Endpoint, route, action

**Catalog**:
The server's list of operations, cached per profile and keyed by server version.
_Avoid_: Schema, spec, API map

**Named command**:
A hand-written command, such as `sbp invoices list`, that maps flags onto one operation.
_Avoid_: Shortcut, alias, wrapper

**Generic layer**:
The `api` and `ops find|describe|call` commands, which reach any operation in the catalog.
_Avoid_: Passthrough, raw mode

**Payment key**:
The caller-chosen string sent as `Idempotency-Key` on a payment write, so a retry can never book the payment twice.
_Avoid_: Request id, nonce, idempotency token

**Envelope**:
The JSON object every command prints when piped or given `--json`: `{ok, data, summary, next}`, or `{ok: false, error}`.
_Avoid_: Response, wrapper, payload
