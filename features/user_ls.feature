@audience:A3
Feature: Listing what is in a BOM
  As a packager inspecting what gets redistributed
  I want to list a BOM's components without knowing its schema
  So that I can see what is in a bundle without a browser or a jq incantation

  Background:
    Given the BOM fixture "diamond"

  Scenario: Listing every component
    When I run "ls"
    Then the output lists 4 components
    And the output reports coverage

  @audience:A1
  Scenario: Listing what one component depends on
    When I run "ls" from "pkg:generic/app@1.0.0"
    Then the output lists 2 components
    And the output contains "a@1.0.0"
    And the output contains "b@1.0.0"

  @audience:A4
  Scenario: Listing what depends on a component
    When I run "ls --reverse" from "pkg:generic/shared@1.0.0"
    Then the output lists 2 components

  Scenario: An unknown component is explained, not silently empty
    When I run "ls" from "pkg:generic/nope@9"
    Then the output explains that the component is unknown

