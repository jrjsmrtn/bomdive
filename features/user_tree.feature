@audience:A4
Feature: Walking the dependency graph
  As a security engineer triaging a BOM
  I want to walk the dependency graph without it hanging or lying to me
  So that I can trust what I am shown, including what I am not shown

  Scenario: A shared component is expanded once and back-referenced after
    Given the BOM fixture "diamond"
    When I run "tree"
    Then the output marks a back-reference
    And the output does not mark a cycle

  Scenario: A cycle terminates and is marked
    Given the BOM fixture "cycle-direct"
    When I run "tree"
    Then the output marks a cycle

  @audience:A2
  Scenario: A partial graph says so rather than looking complete
    Given the BOM fixture "partial-coverage"
    When I run "tree"
    Then the output reports coverage of 3 out of 10
    And the output warns that not every component is shown

  Scenario: Derived roots are labelled as derived
    Given the BOM fixture "rootless"
    When I run "tree"
    Then the output says the roots were derived

  @audience:A6
  Scenario: A BOM that declares no graph refuses informatively
    Given the BOM fixture "obom-categories"
    When I run "tree"
    Then the output explains that the BOM declares no dependency graph
    And the output suggests listing by category
