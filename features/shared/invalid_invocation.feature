Feature: invalid invocation

  As a developer providing the wrong number of arguments or invalid arguments
  I should be reminded of the usage
  So that I can use it correctly without having to look at documentation


  Scenario Outline: <DESCRIPTION>
    When I run `git-town <CMD>`
    Then it runs no commands
    And it prints the error "Usage:"

    Examples:
      | CMD                              |
      | alias                            |
      | alias invalid                    |
      | alias arg1 arg2                  |
      | append                           |
      | append arg1 arg2                 |
      | config arg1                      |
      | hack                             |
      | hack arg1 arg2                   |
      | install-fish-autocompletion arg1 |
      | kill arg1 arg2                   |
      | main-branch arg1 arg2            |
      | new-branch-push-flag arg1 arg2   |
      | new-branch-push-flag invalid     |
      | new-pull-request arg1            |
      | offline invalid                  |
      | offline arg1 arg2                |
      | perennial-branches arg1          |
      | perennial-branches --add         |
      | perennial-branches --remove      |
      | prune-branches arg1              |
      | pull-branch-strategy arg1 arg2   |
      | pull-branch-strategy invalid     |
      | rename-branch                    |
      | rename-branch arg1 arg2 arg3     |
      | repo arg1                        |
      | set-parent-branch arg1           |
      | ship arg1 arg2                   |
      | sync arg1                        |
      | version arg1                     |
