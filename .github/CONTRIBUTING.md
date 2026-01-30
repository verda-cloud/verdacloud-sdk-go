# Contributing to Verda Cloud Go SDK

Thank you for your interest in contributing to the Verda Cloud Go SDK! This document provides guidelines and instructions for contributing.

## Quick Start

1. **Fork and clone** the repository
2. **Set up your development environment**: `make setup`
3. **Create a feature branch**: `git checkout -b feature/your-feature`
4. **Make your changes** following our guidelines
5. **Test locally**: `make ci` (runs all CI checks)
6. **Commit** (pre-commit hooks run automatically on each commit)
7. **Push and create a PR** (CI checks run automatically on every commit you push)
8. **Get 1 maintainer approval** and merge!

## Development Setup

You have **two options** for development:

### Option 1: Native Development (Quick Start)

**Prerequisites:**
- Go 1.21 or higher
- Git
- pre-commit (install via `brew install pre-commit` or `pip install pre-commit`)

**Setup:**
```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/verdacloud-sdk-go.git
cd verdacloud-sdk-go

# Set up development tools
make setup

# Verify setup works
make check
```

This installs:
- golangci-lint (Go linter)
- pre-commit hooks (automatic checks on commit)

### Option 2: Docker Development (Guaranteed Consistency) 🐳

**Why Docker?**
- ✅ Fixed Go 1.21 and golangci-lint 2.0+ versions
- ✅ No local tool installation needed
- ✅ Matches CI/CD environment exactly
- ✅ Works on macOS, Linux, Windows

**Prerequisites:**
- Docker installed

**Setup:**
```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/verdacloud-sdk-go.git
cd verdacloud-sdk-go

# Build the dev container (one time)
make docker-build

# Run any command in the container
make docker-lint     # Linting
make docker-test     # Unit tests
make docker-ci       # All CI checks
```

**All commands available with `docker-` prefix:**
- `make docker-lint` - Run linting
- `make docker-test` - Run unit tests
- `make docker-coverage` - Generate coverage
- `make docker-ci` - Run all CI checks
- `make docker-shell` - Open shell in container
- `make docker-help` - Show Docker commands

**Integration tests with Docker:**
```bash
# Export credentials first
export VERDA_CLIENT_ID="your_id"
export VERDA_CLIENT_SECRET="your_secret"

# Run integration tests
make docker-test-integration
```

## Code Standards

### Style Guide

- Follow standard Go conventions
- Use `gofmt` for formatting (automatic with pre-commit hooks)
- Use `goimports` for import organization (automatic)
- Pass all `golangci-lint` checks

### Testing

- **Unit tests** are required for new features
- Maintain **>80% code coverage**
- Tests must pass: `make test-unit`
- Add integration tests when appropriate

### Documentation

- Update README.md for user-facing changes
- Add code comments for complex logic
- Update CONTRIBUTING.md for workflow or process changes
- Follow GoDoc conventions

## Development Workflow

### Choosing Your Workflow

**Native (fast iteration):**
```bash
make test-unit
make lint
make ci
```

**Docker (guaranteed consistency):**
```bash
make docker-test
make docker-lint
make docker-ci
```

💡 **Tip:** Use native for fast development, Docker before pushing to ensure CI will pass!

### Making Changes

```bash
# Create feature branch
git checkout -b feature/your-feature-name

# Make your changes
vim pkg/verda/client.go

# Test your changes
make test-unit              # or: make docker-test

# Check code quality
make lint                   # or: make docker-lint

# Run all CI checks (matches GitHub Actions)
make ci                     # or: make docker-ci
```

### Committing

Pre-commit hooks run automatically:
- Code formatting (gofmt, goimports)
- Linting (golangci-lint with 15+ linters)
- Unit tests
- Module tidying

```bash
git add .
git commit -m "feat: add new feature"
# Hooks run automatically
```

If hooks fail, fix the issues and commit again.

### Commit Message Format

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting)
- `refactor`: Code refactoring
- `test`: Test updates
- `chore`: Build process or auxiliary tool changes
- `perf`: Performance improvements
- `ci`: CI/CD changes

**Examples:**
```
feat(client): add retry logic for failed requests
fix(auth): handle token refresh race condition
docs(readme): update installation instructions
test(instances): add test for instance creation
```

## Pull Request Process

### Before Submitting

1. ✅ All tests pass: `make test-unit`
2. ✅ Linting passes: `make lint`
3. ✅ Code is formatted: `make fmt`
4. ✅ CI checks pass: `make ci`
5. ✅ Documentation is updated
6. ✅ Commit messages follow convention

### Submitting

1. Push your branch: `git push origin feature/your-feature`
2. Create a Pull Request on GitHub
3. Fill out the PR template completely
4. Wait for CI checks to pass
5. Respond to review feedback

### PR Requirements

- All CI checks must pass (see CI checks below)
- **At least one approving review from a maintainer**
- No merge conflicts with main
- All conversations resolved
- Pre-commit hooks passed locally

### CI Checks

**All CI checks run automatically on every commit** you push to your PR branch. This ensures code quality is maintained throughout the development process.

PRs must pass these checks:
- **Lint**: Static analysis with 15+ linters (golangci-lint)
- **Test**: Unit tests on Go 1.21 and 1.22
- **Build**: Compilation verification
- **Format**: Code formatting check (gofmt, goimports)
- **Mod Tidy**: Go modules cleanliness
- **Integration Tests**: Real API tests (requires credentials, may be skipped for PRs from forks)

**Note:** Integration tests will run if API credentials are available. For PRs from forks, they'll be skipped automatically but run after merge.

Run all checks locally before pushing: `make ci`

## Code Review

### What Reviewers Look For

- **Correctness**: Does it solve the problem?
- **Tests**: Are there adequate tests?
- **Documentation**: Is it well-documented?
- **Style**: Does it follow conventions?
- **Performance**: Is it efficient?
- **Security**: Are there security concerns?

### Responding to Feedback

- Be open to suggestions
- Ask questions if unclear
- Make requested changes
- Push updates to your branch (don't force push)
- Re-request review when ready

## Testing Guidelines

### Unit Tests

**Required for:**
- All new features
- Bug fixes
- Code refactoring

**Location:** `pkg/verda/*_test.go`

**Example:**
```go
func TestClientGet(t *testing.T) {
    // Setup
    client := setupTestClient(t)

    // Test
    result, err := client.Get(context.Background())

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
}
```

**Run:**
```bash
make test-unit           # All unit tests
go test -v ./pkg/verda  # Verbose output
make coverage           # With coverage report
```

### Integration Tests

**Location:** `test/integration/*_test.go`

**When to Add:**
- Real API interactions
- End-to-end workflows
- Authentication flows

**Setup:**
See [API Credentials Setup](#api-credentials-setup-for-maintainers) for detailed instructions on configuring credentials.

**Run:**
```bash
# After setting up credentials (see above)
make test-integration

# Or with inline credentials
VERDA_CLIENT_ID="your_id" VERDA_CLIENT_SECRET="your_secret" make test-integration
```

## Common Tasks

### Add a New Feature

```bash
# 1. Create branch
git checkout -b feature/my-feature

# 2. Implement feature
vim pkg/verda/new_feature.go

# 3. Add tests
vim pkg/verda/new_feature_test.go

# 4. Update documentation
vim README.md

# 5. Verify locally
make ci

# 6. Commit and push
git add .
git commit -m "feat: add my feature"
git push origin feature/my-feature

# 7. Create PR on GitHub
```

### Fix a Bug

```bash
# 1. Create branch
git checkout -b fix/bug-description

# 2. Write failing test
vim pkg/verda/buggy_code_test.go

# 3. Fix the bug
vim pkg/verda/buggy_code.go

# 4. Verify test passes
make test-unit

# 5. Commit and push
git add .
git commit -m "fix: resolve bug description"
git push origin fix/bug-description
```

### Update Documentation

```bash
# 1. Create branch
git checkout -b docs/update-readme

# 2. Update docs
vim README.md

# 3. Verify
make check

# 4. Commit and push
git add .
git commit -m "docs: update README with examples"
git push origin docs/update-readme
```

## Debugging CI Failures

### Lint Failures

```bash
# Run locally
make lint

# See specific issues
golangci-lint run ./...

# Fix and rerun
make lint
```

### Test Failures

```bash
# Run with verbose output
go test -v ./pkg/verda

# Run specific test
go test -v -run TestClientGet ./pkg/verda

# Debug with prints
go test -v ./pkg/verda -count=1  # Disable cache
```

### Format Failures

```bash
# Fix formatting
make fmt

# Verify
git diff

# Commit
git add .
git commit -m "style: format code"
```

## API Credentials Setup (For Maintainers)

Integration tests require Verda API credentials. You need to set these up in two places:

### GitHub Secrets (For CI/CD)

To enable integration tests in GitHub Actions:

1. **Navigate to Secrets:**
   - Go to your repository on GitHub
   - Click **Settings** → **Secrets and variables** → **Actions**
   - Click **New repository secret**

2. **Add Required Secrets:**

   **Secret 1: VERDA_CLIENT_ID**
   - Name: `VERDA_CLIENT_ID`
   - Value: Your Verda API Client ID
   - Click **Add secret**

   **Secret 2: VERDA_CLIENT_SECRET**
   - Name: `VERDA_CLIENT_SECRET`
   - Value: Your Verda API Client Secret
   - Click **Add secret**

   **Secret 3: VERDA_BASE_URL** (optional)
   - Name: `VERDA_BASE_URL`
   - Value: Custom API base URL (if needed)
   - Click **Add secret**

3. **Verify Setup:**
   - Create a test PR
   - Check that "Integration Tests" job runs successfully
   - If secrets are missing, the job will skip gracefully

### Local Environment (For Development)

To run integration tests locally:

**Option 1: Export Variables (Temporary)**
```bash
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"
export VERDA_BASE_URL="https://api.verda.cloud"  # optional

# Run tests
make test-integration
```

**Option 2: Create .env File (Persistent)**
```bash
# Create .env file (already in .gitignore)
cat > .env << 'EOF'
export VERDA_CLIENT_ID="your_client_id"
export VERDA_CLIENT_SECRET="your_client_secret"
export VERDA_BASE_URL="https://api.verda.cloud"
EOF

# Load and run
source .env
make test-integration
```

**Option 3: Add to Shell Profile (Permanent)**
```bash
# Add to ~/.zshrc or ~/.bashrc
echo 'export VERDA_CLIENT_ID="your_client_id"' >> ~/.zshrc
echo 'export VERDA_CLIENT_SECRET="your_client_secret"' >> ~/.zshrc

# Reload shell
source ~/.zshrc

# Run tests
make test-integration
```

**Security Note:** Never commit credentials to git! The `.env` file is already in `.gitignore`.

## Branch Protection Setup (For Maintainers)

To enforce the PR requirements, configure branch protection rules in GitHub:

### Step 1: Navigate to Branch Protection
1. Go to repository **Settings** → **Branches**
2. Click **Add branch protection rule**
3. Set **Branch name pattern**: `main` (or `develop`)

### Step 2: Configure Protection Rules
Enable these settings:
- ✅ **Require a pull request before merging**
  - ✅ **Require approvals**: Set to **1**
  - ✅ **Dismiss stale pull request approvals when new commits are pushed**
- ✅ **Require status checks to pass before merging**
  - ✅ **Require branches to be up to date before merging**
  - Add required checks:
    - `All CI Checks Passed` (this is the ci-success job - includes all checks below)
    - `Lint`
    - `Test (Go 1.21)`
    - `Test (Go 1.22)`
    - `Build`
    - `Format Check`
    - `Go Mod Tidy Check`
    - `Integration Tests`
- ✅ **Require conversation resolution before merging**
- ✅ **Do not allow bypassing the above settings** (optional, recommended)

### Step 3: Save
Click **Create** or **Save changes**

This ensures:
- Every PR needs **1 maintainer approval**
- All CI checks must pass
- Code is automatically tested on every commit

## Getting Help

- **Documentation**: Check [README.md](../README.md)
- **Issues**: Search existing issues or create a new one
- **Discussions**: Use GitHub Discussions for questions
- **Contact**: Reach out to maintainers

## Code of Conduct

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Follow professional standards

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

---

**Thank you for contributing!** 🎉

## Release Workflow

This project tracks changes in [CHANGELOG.md](../../CHANGELOG.md).

### 1. Development (Unreleased)
When merging PRs, ensure significant changes are added to the `[Unreleased]` section of `CHANGELOG.md`.

### 2. Creating a Release
To cut a new release (e.g., `v1.2.3`):

1. **Run the release script:**
   ```bash
   make release VERSION=v1.2.3
   ```
   This moves "Unreleased" changes to a new `[v1.2.3]` section.

2. **Verify CHANGELOG.md:**
   Check that the entries look correct.

3. **Commit and Tag:**
   ```bash
   git add CHANGELOG.md
   git commit -m "chore: release v1.2.3"
   git tag v1.2.3
   git push origin main v1.2.3
   ```
