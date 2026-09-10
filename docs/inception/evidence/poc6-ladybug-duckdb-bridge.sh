#!/usr/bin/env bash
# THE COMBO: Cypher over DuckDB-held BOM data, without any DuckDB Cypher extension.
#
# There is no Cypher extension for DuckDB. But DuckDB reads CycloneDX with zero ingest,
# and LadybugDB reads DuckDB and Parquet — so the two compose into what neither does
# alone: SQL/zero-ingest on one side, variable-length graph traversal on the other.
#
# This script proves BOTH handoffs and asserts they agree:
#   path A  CDX JSON -> DuckDB -> Parquet -> lbug COPY FROM
#   path B  CDX JSON -> DuckDB file -> lbug ATTACH (dbtype duckdb)
#
# Requires: duckdb, lbug (LadybugDB shell). Path B additionally needs network on first
# run, because `INSTALL duckdb` fetches a Ladybug extension at runtime — which is itself
# one of the findings, see the POC-6 document.
#
# Usage: poc6-ladybug-duckdb-bridge.sh [bom.cdx.json]
set -uo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BOM="${1:-$here/../../../testdata/diamond.cdx.json}"

for t in duckdb lbug; do
    command -v "$t" >/dev/null 2>&1 || {
        echo "CANNOT RUN: $t not found. This is a hard failure, not a skip." >&2; exit 2; }
done
[ -f "$BOM" ] || { echo "CANNOT RUN: no BOM at $BOM" >&2; exit 2; }

work="$(mktemp -d)"; trap 'rm -rf "$work"' EXIT
echo "BOM: $(basename "$BOM")"

# ---- DuckDB: read CycloneDX with no ingest, emit node/edge tables ------------
duckdb "$work/bom.duckdb" -c "
CREATE TABLE nodes AS
  SELECT c.\"bom-ref\" AS id, c.name AS name, c.version AS version, c.type AS type
  FROM (SELECT unnest(components) AS c FROM read_json_auto('$BOM'))
  WHERE c.\"bom-ref\" IS NOT NULL;
CREATE TABLE edges AS
  WITH d AS (SELECT unnest(dependencies) AS dep FROM read_json_auto('$BOM'))
  SELECT dep.ref AS src, unnest(dep.dependsOn) AS dst FROM d;
COPY nodes TO '$work/nodes.parquet' (FORMAT parquet);
COPY edges TO '$work/edges.parquet' (FORMAT parquet);
" >/dev/null || { echo "duckdb stage failed" >&2; exit 1; }

want_n=$(duckdb "$work/bom.duckdb" -noheader -list -c "SELECT count(*) FROM nodes;")
want_e=$(duckdb "$work/bom.duckdb" -noheader -list -c "SELECT count(*) FROM edges;")
echo "  duckdb read it directly: nodes=$want_n edges=$want_e"

# lbug prints a progress bar to stdout; keep only the value lines.
lb() { gtimeout 300 lbug "$1" -m csv 2>&1 | sed 's/\x1b\[[0-9;]*m//g' \
        | grep -vE 'Pipelines Finished|Current Pipeline|^\(|^Time:|^$'; }

# ---- path A: Parquet handoff -------------------------------------------------
got_a=$(lb "$work/dbA" <<CY | tail -1
CREATE NODE TABLE Component(id STRING, name STRING, version STRING, type STRING, PRIMARY KEY(id));
CREATE REL TABLE DEPENDS_ON(FROM Component TO Component);
COPY Component FROM '$work/nodes.parquet';
COPY DEPENDS_ON FROM '$work/edges.parquet';
MATCH (c:Component) RETURN count(*);
CY
)
echo "  path A  parquet -> lbug        : nodes=$got_a"

# ---- path B: attach the DuckDB file directly ---------------------------------
got_b=$(lb "$work/dbB" <<CY | tail -1
INSTALL duckdb;
LOAD duckdb;
ATTACH '$work/bom.duckdb' AS bomdb (dbtype duckdb);
LOAD FROM bomdb.nodes RETURN count(*);
CY
)
echo "  path B  ATTACH duckdb -> lbug  : nodes=$got_b"

# ---- the query neither tool does alone ---------------------------------------
leaf=$(duckdb "$work/bom.duckdb" -noheader -list -c "
  SELECT n.name FROM nodes n
  WHERE n.id IN (SELECT dst FROM edges) AND n.id NOT IN (SELECT src FROM edges WHERE dst IS NOT NULL)
  LIMIT 1;")
reach=$(lb "$work/dbA" <<CY | tail -1
MATCH (a:Component)-[:DEPENDS_ON*1..12]->(b:Component) WHERE b.name = '$leaf'
RETURN count(DISTINCT a.name);
CY
)
echo "  transitively reach '$leaf'      : $reach   (variable-length Cypher over DuckDB-read data)"

fail=0
[ "$got_a" = "$want_n" ] || { echo "✗ path A disagrees: $got_a vs $want_n" >&2; fail=1; }
[ "$got_b" = "$want_n" ] || { echo "✗ path B disagrees: $got_b vs $want_n" >&2; fail=1; }
[ -n "$reach" ] && [ "$reach" -ge 1 ] 2>/dev/null || { echo "✗ traversal returned nothing" >&2; fail=1; }
[ $fail -eq 0 ] && echo "OK: both handoffs agree with DuckDB, and Cypher traverses the result"
exit $fail
