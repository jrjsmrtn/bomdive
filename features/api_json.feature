@audience:A5
Feature: Machine-readable output
  As an SBOM toolchain consuming lsxbom in a pipeline
  I want the same caveats the human surface carries
  So that I cannot mistake a sparse graph for a complete one

  Scenario: JSON carries coverage
    Given the BOM fixture "partial-coverage"
    When I run "tree --json"
    Then the output is valid JSON
    And the JSON field "coverage.complete" is false
    And the JSON field "coverage.in_graph" is 3

  Scenario: JSON flags synthetic roots
    Given the BOM fixture "rootless"
    When I run "tree --json"
    Then the JSON field "synthetic_roots" is true

  Scenario: JSON distinguishes a declared-empty graph from an absent one
    Given the BOM fixture "obom-categories"
    When I run "tree --json"
    Then the JSON field "declares_no_graph" is true
    And the JSON field "has_dependencies" is true

  Scenario: Entries is an empty list, never null
    Given the BOM fixture "obom-categories"
    When I run "tree --json"
    Then the JSON field "entries" is an empty list

  Scenario: Diagnostics never reach stdout
    Given a BOM path that does not exist
    When I run "tree --json"
    Then the command fails
    And stdout is empty
