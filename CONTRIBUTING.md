# Contributing to histui

Thank you for your interest in contributing to histui! This document provides guidelines for contributing code and documentation.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/YOUR_USERNAME/histui.git`
3. Create a branch: `git checkout -b feature/your-feature-name`

## Development Setup

### Prerequisites

- Go 1.21 or later
- For histuid daemon: GTK4 development libraries
- For documentation: Node.js 20+

### Building

```bash
# Build histui CLI
task build

# Build histuid daemon (requires GTK4)
task build-daemon

# Run tests
task test

# Run linter
task lint
```

## Contributing Code

### Code Style

- Follow standard Go conventions (`gofmt`, `goimports`)
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions focused and small

### Testing

- Write tests for new functionality
- Ensure existing tests pass: `task test`
- Include both unit tests and integration tests where appropriate

### Commit Messages

Use conventional commits format:

```
type(scope): description

[optional body]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:
- `feat(cli): add JSON output format for get command`
- `fix(daemon): handle empty notification body`
- `docs: update installation instructions`

## Contributing Documentation

Documentation is in the `docs/` directory, built with Docusaurus.

### Local Development

```bash
cd docs
npm install
npm start
```

Visit http://localhost:3000/histui/ to preview changes.

### Documentation Structure

- `docs/docs/` - Main documentation content
- `docs/src/pages/` - Custom React pages
- `docs/static/` - Static assets (images, examples)

### Writing Guidelines

- Use sentence case for headings
- Include code examples for CLI commands
- Use appropriate language tags for code blocks (`bash`, `toml`, `css`, `go`)
- Keep pages focused on a single topic

### Building Documentation

```bash
cd docs
npm run build
npm run serve  # Preview production build (required for search)
```

## Pull Request Process

1. Ensure your code passes all tests and linting
2. Update documentation if you're changing user-facing behavior
3. Write a clear PR description explaining what and why
4. Reference any related issues

### PR Template

When creating a pull request, include:

- **Summary**: What does this PR do?
- **Motivation**: Why is this change needed?
- **Testing**: How was this tested?
- **Breaking Changes**: Any breaking changes?

## Documentation Versioning

Documentation versions are created automatically when releases are published. The workflow:

1. A release is published with tag `vX.Y.Z`
2. GitHub Actions runs `npm run docusaurus docs:version X.Y.Z`
3. A snapshot is created in `versioned_docs/version-X.Y.Z/`
4. The version appears in the documentation version dropdown

## Questions?

- Open an issue for bugs or feature requests
- Check existing issues before creating new ones

Thank you for contributing!
