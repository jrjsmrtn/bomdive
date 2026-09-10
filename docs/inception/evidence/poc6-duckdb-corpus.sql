-- POC-6: DuckDB reading CycloneDX with ZERO ingest.
--
-- No schema, no import, no database file. This is the finding that most affects the
-- design: the friction in cross-BOM questions was never the query language, it was
-- building and refreshing a store — and DuckDB removes that entirely.
--
--   duckdb -c ".read poc6-duckdb-corpus.sql"     (edit the paths first)
--
-- Substitute your own paths. The corpus glob is deliberately not a real path here:
-- BOMs of a private estate carry host identifiers, and this file is committed.

-- 1. Read one CycloneDX file directly ---------------------------------------
SELECT bomFormat, specVersion, len(components) AS components, len(dependencies) AS deps
FROM read_json_auto('testdata/diamond.cdx.json');

-- 2. Components as a relational table — the `ls` shape ------------------------
SELECT c.type, count(*) AS n
FROM (SELECT unnest(components) AS c FROM read_json_auto('testdata/diamond.cdx.json'))
GROUP BY 1 ORDER BY 2 DESC;

-- 3. Explode the dependency graph into an edge list ---------------------------
WITH d AS (SELECT unnest(dependencies) AS dep FROM read_json_auto('testdata/diamond.cdx.json'))
SELECT dep.ref AS src, unnest(dep.dependsOn) AS dst FROM d;

-- 4. A WHOLE CORPUS in one statement, via glob --------------------------------
-- Reproduced POC-2's entire finding as a single query. `union_by_name` is required
-- because BOM types do not share a column set.
-- COMMENTED OUT so this file runs start to finish. Uncomment and substitute a real
-- corpus directory; it is left unexecuted because a committed script must not
-- carry a path into a private estate.
-- SELECT
--   CASE WHEN filename LIKE '%/obom-%' THEN 'OBOM'
--        WHEN filename LIKE '%/hbom-%' THEN 'HBOM'
--        ELSE 'SBOM' END        AS kind,
--   count(*)                    AS files,
--   sum(len(components))        AS components,
--   sum(len(dependencies))      AS dep_entries,
--   min(len(components))        AS min_comp,
--   max(len(components))        AS max_comp
-- FROM read_json_auto('<corpus-dir>/*/*.json', filename=true, union_by_name=true)
-- GROUP BY 1 ORDER BY 2 DESC;

-- 5. Transitive closure straight off the JSON ---------------------------------
-- WARNING, and it is the same lesson as the cycle-semantics split: this terminates
-- ONLY because of the depth cap. With `depth` in the recursion tuple, UNION does not
-- dedupe cycles away — a cyclic BOM will run until the cap stops it. Cycle handling
-- is a design decision in every backend, never a default you inherit.
CREATE OR REPLACE TEMP TABLE edge AS
  WITH d AS (SELECT unnest(dependencies) AS dep FROM read_json_auto('testdata/diamond.cdx.json'))
  SELECT dep.ref AS src, unnest(dep.dependsOn) AS dst FROM d;

WITH RECURSIVE reach(root, node, depth) AS (
    SELECT src, dst, 1 FROM edge
  UNION
    SELECT r.root, e.dst, r.depth + 1
      FROM reach r JOIN edge e ON e.src = r.node
     WHERE r.depth < 20
)
SELECT count(DISTINCT root) AS roots_with_reach,
       count(*)             AS closure_pairs,
       max(depth)           AS max_depth
FROM reach;
