@audience:A1 @audience:A4
Feature: Navigate a document in Miller columns
  As someone reading a BOM at a terminal
  I want to walk the dependency path one column at a time
  So that "what pulled this in" is answered by moving, not by holding a schema in my head

  # These scenarios drive the column MODEL, not a drawn screen: `browse` refuses to run without
  # a terminal, so it cannot go through cli.Run like ls and tree. What is drawn is asserted in
  # internal/tui against a tcell simulation screen, and compared between builds with tmux
  # snapshots. What is pinned here is the contract a reader would notice breaking.

  Scenario: The entry column is chosen from the document, not assumed
    Given the document "tree-simple"
    When I browse it
    Then the entry column is titled "roots"
    And the entry column lists 1 row

  Scenario: A synthetic root is labelled as synthetic
    Given the document "rootless"
    When I browse it
    Then the entry column is titled "roots (derived)"

  Scenario: An OBOM with no dependency graph opens on its categories
    Given the document "obom-categories"
    When I browse it
    Then the entry column is titled "categories"

  Scenario: Descending follows the dependency path, and going back retraces it
    Given the document "tree-simple"
    When I browse it
    And I descend
    Then there are 2 columns
    And the path reads "app > a"
    When I go back
    Then there are 1 columns

  Scenario: Flipping direction is stated, because forward and reverse look identical
    Given the document "diamond"
    When I browse it
    Then the view says it is showing "dependencies"
    When I flip the direction
    Then the view says it is showing "dependents"

  Scenario: A document with no components opens on what it does carry
    Given the document "vex-standalone"
    When I browse it
    Then the entry column is titled "vulnerabilities"
    And the entry column lists 1 row

  Scenario: A document carrying only its subject opens on that subject
    Given the document "metadata-only"
    When I browse it
    Then the entry column is titled "subject"
    And the entry column lists 1 row

  Scenario: A partial graph is never presented as complete
    Given the document "partial-coverage"
    When I browse it
    Then the view reports coverage below 100 percent
    And the view explains the coverage
