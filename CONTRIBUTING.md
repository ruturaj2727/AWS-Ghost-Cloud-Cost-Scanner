# Contributing to aws-ghost

Thank you for your interest in contributing to `aws-ghost`! This document provides guidelines for contributors.

## Getting Started

### Prerequisites

- Go 1.21 or later
- Git
- AWS account (for testing)
- Make (optional, for using Makefile)

### Development Setup

1. **Fork the repository**
   ```bash
   # Fork the repository on GitHub, then clone your fork
   git clone https://github.com/your-username/aws-ghost.git
   cd aws-ghost
   ```

2. **Add upstream remote**
   ```bash
   git remote add upstream https://github.com/NotHarshhaa/aws-ghost.git
   ```

3. **Install dependencies**
   ```bash
   go mod tidy
   ```

4. **Build the project**
   ```bash
   go build -o bin/aws-ghost ./cmd/aws-ghost
   # or use Makefile
   make build
   ```

5. **Run tests**
   ```bash
   go test ./...
   # or use Makefile
   make test
   ```

## Development Workflow

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/issue-description
```

### 2. Make Changes

- Follow the existing code style and patterns
- Add tests for new functionality
- Update documentation as needed
- Ensure all tests pass

### 3. Test Your Changes

```bash
# Run unit tests
go test ./...

# Run integration tests (requires AWS credentials)
go test -tags=integration ./...

# Build to ensure no compilation errors
go build -o bin/aws-ghost ./cmd/aws-ghost

# Test the CLI manually
./bin/aws-ghost --help
```

### 4. Commit Changes

Use clear, descriptive commit messages:

```
feat: add support for new resource type
fix: resolve issue with credential validation
docs: update README with new examples
test: add integration tests for EBS scanner
```

### 5. Submit Pull Request

- Push your branch to your fork
- Create a pull request against the `main` branch
- Fill out the pull request template
- Wait for review

## Code Style Guidelines

### Go Code Style

- Follow [Go's official code style](https://golang.org/doc/effective_go.html)
- Use `gofmt` to format code
- Use `golint` and `go vet` to check code quality
- Keep functions small and focused
- Use meaningful variable and function names

### Example Code Style

```go
// Good: Clear function name and documentation
func ScanEBSVolumes(ctx context.Context, client *ec2.Client, config ScanConfig) ([]Resource, error) {
    // Implementation here
}

// Bad: Unclear naming
func scan(c *ec2.Client, cfg ScanConfig) ([]Resource, error) {
    // Implementation here
}
```

### Error Handling

- Always handle errors explicitly
- Use descriptive error messages
- Wrap errors with context using `fmt.Errorf` or `errors.Wrap`

```go
// Good: Descriptive error with context
if err != nil {
    return nil, fmt.Errorf("failed to describe EBS volumes: %w", err)
}

// Bad: Generic error
if err != nil {
    return nil, err
}
```

## Adding New Resource Scanners

### 1. Create Scanner Implementation

Create a new file in `internal/scanner/`:

```go
package scanner

import (
    "context"
    "github.com/NotHarshhaa/aws-ghost/internal/aws"
    "github.com/NotHarshhaa/aws-ghost/pkg/types"
)

type NewResourceScanner struct {
    client *aws.Client
}

func NewNewResourceScanner(client *aws.Client) *NewResourceScanner {
    return &NewResourceScanner{client: client}
}

func (s *NewResourceScanner) Scan(config types.ScanConfig) ([]types.Resource, error) {
    // Implementation here
    return resources, nil
}
```

### 2. Register the Scanner

Update `internal/scanner/registry.go`:

```go
func NewRegistry(client *aws.Client) *Registry {
    return &Registry{
        scanners: map[string]Scanner{
            "ebs":       NewEBSScanner(client),
            "eip":       NewEIPScanner(client),
            "newresource": NewNewResourceScanner(client), // Add this line
        },
    }
}
```

### 3. Add Tests

Create `internal/scanner/newresource_test.go`:

```go
package scanner

import (
    "testing"
    "github.com/NotHarshhaa/aws-ghost/pkg/types"
)

func TestNewResourceScanner(t *testing.T) {
    // Test implementation here
}
```

### 4. Update Documentation

- Add the new resource type to `README.md`
- Update the "What it scans" table
- Add examples if needed

## Security Considerations

### Read-Only Operations

All scanners must only use read-only AWS API operations:

- ✅ `Describe*`, `List*`, `Get*`
- ❌ `Create*`, `Delete*`, `Update*`, `Put*`

### Credential Safety

- Never log AWS credentials
- Never transmit credentials to external services
- Use the security validator for credential checks

### API Call Tracking

Log all AWS API calls using the API tracker:

```go
apiTracker.LogCall("EC2", "DescribeVolumes", region, true, "")
```

## Testing

### Unit Tests

- Write tests for all new functionality
- Use table-driven tests where appropriate
- Mock external dependencies
- Aim for high code coverage

### Integration Tests

- Use the `integration` build tag
- Require real AWS credentials
- Test against real AWS resources
- Use test-specific AWS accounts when possible

### Example Test Structure

```go
func TestEBSScanner(t *testing.T) {
    tests := []struct {
        name     string
        config   types.ScanConfig
        expected []types.Resource
        wantErr  bool
    }{
        {
            name: "successful scan",
            config: types.ScanConfig{
                Region:   "us-east-1",
                IdleDays: 7,
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Documentation

### README Updates

- Update feature lists
- Add new usage examples
- Update required permissions
- Add new resource types to the table

### Code Documentation

- Document all public functions and types
- Use Go doc comments
- Include usage examples in documentation
- Update SECURITY.md for security-related changes

### Changelog

Maintain a changelog in `CHANGELOG.md`:

```markdown
## [1.1.0] - 2024-01-15

### Added
- Support for scanning EKS clusters
- Security audit command
- API call transparency

### Fixed
- Issue with credential validation
- Memory leak in scanner registry

### Changed
- Updated Go version requirement to 1.21
- Improved error messages
```

## Release Process

### 1. Update Version

Update version constants in:
- `cmd/aws-ghost/cmd/version.go`
- `go.mod` (if needed)

### 2. Update Documentation

- Update `README.md`
- Update `CHANGELOG.md`
- Review `SECURITY.md`

### 3. Create Release Tag

```bash
git tag -a v1.1.0 -m "Release version 1.1.0"
git push origin v1.1.0
```

### 4. Build Release Artifacts

```bash
make release
```

## Community Guidelines

### Code Review

- All contributions require code review
- Be constructive and respectful in reviews
- Address review feedback promptly
- Ask questions if anything is unclear

### Communication

- Use GitHub issues for bug reports and feature requests
- Use GitHub discussions for general questions
- Be patient with response times

### Security Issues

- Report security vulnerabilities privately
- Email: security@notHarshhaa.com
- Do not create public issues for security problems

## Getting Help

If you need help with contributing:

1. Check existing GitHub issues and discussions
2. Read the documentation thoroughly
3. Create a new issue with your question
4. Join our community discussions

## Recognition

Contributors will be recognized in:
- README.md contributors section
- Release notes
- GitHub contributor statistics

Thank you for contributing to `aws-ghost`! 🎉
