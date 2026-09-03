# peekgit Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-07

## Active Technologies
- Go 1.24.0 + `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss` (002-ui-ux-beautification)
- N/A (In-memory state, local config files) (002-ui-ux-beautification)

- Go (1.21+) + `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `google/go-github` (001-pr-list-diff)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for Go (1.21+)

## Code Style

Go (1.21+): Follow standard conventions

## Recent Changes
- 003-cli-help-version: Added [if applicable, e.g., PostgreSQL, CoreData, files or N/A]
- 002-ui-ux-beautification: Added Go 1.24.0 + `github.com/charmbracelet/bubbles`, `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`

- 001-pr-list-diff: Added Go (1.21+) + `charmbracelet/bubbletea`, `charmbracelet/lipgloss`, `google/go-github`

<!-- MANUAL ADDITIONS START -->
## Quality Standards & Tooling
- **Test Coverage**: Mandatory 100% statement coverage.
- **CRAP Threshold**: Maximum score of 8 per function (calculated via `go-crap`).
- **Mutation Testing**: Evaluated with `gremlins` and integrated into CRAP scoring.

## Quality Commands
```bash
make test       # Run unit tests
make cover      # Run tests with coverage profile
make crap       # Check CRAP scores (threshold <= 8)
make mutate     # Run mutation testing
make quality    # Run full coverage + mutation + CRAP quality gate
```
<!-- MANUAL ADDITIONS END -->
