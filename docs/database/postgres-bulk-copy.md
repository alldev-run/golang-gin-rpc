# PostgreSQL Bulk Inserts

`postgres.Client.CopyFrom` uses the `database/sql` connection backed by `lib/pq`, which does not expose a pgx connection for `pgx.CopyFrom`. It therefore performs multi-row `INSERT` statements in a single transaction. Rows are split into batches that stay within PostgreSQL's 65,535-parameter limit, and any batch failure rolls back the transaction. Table and column names are quoted as SQL identifiers; values are always passed as parameters.

For very large imports where PostgreSQL's COPY protocol is required, use a dedicated pgx connection rather than expecting `Client.CopyFrom` to expose one.
