# Clean-room policy

SeshatOps must stay understandable, buildable, and demonstrable without any
private production system. **Ahoy is excluded** — never a source, schema,
dataset, screenshot, or runtime dependency.

## Forbidden

Do not inspect, copy, paraphrase, or commit:

1. Ahoy or other private application code, libraries, configs, or infrastructure.
2. Private schemas, migrations, API contracts, or data dictionaries.
3. Private operational data, logs, traces, metrics dumps, or backups.
4. Screenshots, recordings, or exports from private systems.
5. Production identifiers, hostnames, account IDs, tenant IDs, or internal URLs.
6. Private business-specific rules, recipes, prices, customers, or suppliers.
7. Production secrets, credentials, tokens, or private environment files.
8. Raw AI conversations that contain private context.
9. “Sanitized” versions of the above that still encode private structure.

Do not commit a private denylist of real identifiers.

## Permitted

1. Independently authored SeshatOps material.
2. Fictional **Northstar Foods** scenario.
3. Synthetic data created for SeshatOps, with origin, generation method,
   license, reproducibility, and independence from private production data.
4. Properly licensed public standards and documentation.
5. Generic industry knowledge that does not encode a private system’s schema.
6. Public open-source dependencies chosen on their own merits.

If material could only have been produced by studying a private system, exclude
it. If provenance is uncertain, exclude it. Do not sanitize and keep.

Northstar Event Spine fixtures: `northstar.Generate` with seed
`northstar-m1-order-line-v1`, and `northstar.GenerateLineage` with seed
`northstar-m3-lineage-v1`. Goldens under `northstar/testdata/` and
`event/testdata/`. Reproduce with `go test ./event ./northstar ./erp`.
Identifiers such as `item-flour-001` and `mill-northstar-001` are fictional.

## AI-assisted work

Prompts must exclude forbidden material. Treat AI output as untrusted until
reviewed. Do not commit raw private-context transcripts.

## Screenshots and fixtures

Northstar or generic SeshatOps only. Independently invented names and schemas.
Secret placeholders only (`REPLACE_ME`), never real credentials.

## If something leaked

Stop distribution. Remove it from the working tree and, if committed, from
history with explicit maintainer approval. Redact hosted artifacts. Rotate
exposed credentials. Re-check this policy before continuing.
