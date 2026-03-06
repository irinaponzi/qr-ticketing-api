# Linting & Code Quality

Static analysis configuration and coding standards enforcement.

---

## golangci-lint

The project uses [golangci-lint](https://golangci-lint.run/) v2 for comprehensive static analysis.

### Running the Linter

```bash
make lint
```

This automatically installs `golangci-lint` if not present and runs all configured linters.

---

## Configuration

Defined in `.golangci.yml` at the project root.

### Enabled Linters

| Linter | Category | Purpose |
|---|---|---|
| `govet` | Correctness | Reports suspicious constructs |
| `errcheck` | Correctness | Checks unchecked errors |
| `staticcheck` | Correctness | Advanced static analysis |
| `unused` | Cleanliness | Detects unused code |
| `gosimple` | Style | Suggests code simplifications |
| `ineffassign` | Correctness | Detects ineffectual assignments |
| `revive` | Style | Extensible Go linter (replaces golint) |
| `gocritic` | Style | Opinionated code patterns |
| `gofmt` | Formatting | Enforces standard formatting |
| `goimports` | Formatting | Enforces import ordering |

### Exclusions

The configuration includes targeted exclusions for common false positives:

| Pattern | Linter | Reason |
|---|---|---|
| `defer .*\.Close\(\)` | errcheck | Deferred close errors are intentionally ignored |
| `msg\.Ack\(` / `msg\.Nack\(` | errcheck | RabbitMQ ack/nack in consumer loops |
| `stutters` | revive | Allows `ticket.TicketService` naming pattern |
| `package name` | revive | Allows `metrics.HTTPMetricsMiddleware` |
| `os.Exit` | gocritic | Allowed in `main()` functions |

---

## GoDoc Standards

All exported types, functions, methods, and constants must have GoDoc comments.

### Format

```go
// FunctionName does something specific.
//
// Parameters:
//   - param1: Description of the first parameter.
//   - param2: Description of the second parameter.
//
// Returns:
//   - *Type: Description of the return value.
//   - error: Conditions that cause an error.
func FunctionName(param1 string, param2 int) (*Type, error) {
```

### Rules

- **First line** must start with the identifier name
- **Parameters** section documents each input
- **Returns** section documents each output
- Keep descriptions concise but informative
- Document error conditions explicitly

---

## Code Quality Checklist

Before submitting code, ensure:

- [ ] `make build` compiles without errors
- [ ] `make lint` reports zero issues
- [ ] `make test` passes all tests
- [ ] All exported identifiers have GoDoc comments
- [ ] Domain entities follow the attributes pattern
- [ ] Repository interfaces stay minimal
- [ ] No infrastructure imports in domain packages
- [ ] Error handling follows domain conventions (`ErrNotFound`, `ErrBusinessRule`)

---

## CI Integration

To add linting to a CI pipeline:

```yaml
# Example GitHub Actions step
- name: Lint
  uses: golangci/golangci-lint-action@v6
  with:
    version: latest
```

The `.golangci.yml` configuration is automatically picked up by the action.
