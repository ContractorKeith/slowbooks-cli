# A token-API client only, with no MCP server

The CLI talks to SlowBooks Pro only through its public `/api` with an `sbp_` token, under the same network rules as Switchboard (HTTPS except loopback or private addresses, no redirects, `/api/` and `/openapi.json` only). It never imports product code, reads the database, or ships its own MCP server: Switchboard owns MCP, and agents that have a shell use this CLI with a skill instead.
