<div align="center">
  <img src="logo.png" alt="GoGH Logo" width="200" height="200">
</div>

# GoGH 🚀

**GitHub Actions Local Runner** - Test and debug your GitHub Actions workflows locally with Docker

[![Go Version](https://img.shields.io/badge/go-1.19+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Contributions Welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg)](CONTRIBUTING.md)

## Overview

GoGH is a lightweight, fast local runner for GitHub Actions workflows. Instead of pushing to GitHub and waiting for Actions to run, test your workflows locally with full Docker support, real-time logging, and terminal-based progress tracking.

## ✨ Features

- **🔧 Local Workflow Execution** - Run GitHub Actions workflows on your machine
- **🐳 Docker Integration** - Full container support with automatic image management  
- **📊 Real-time Display** - Beautiful terminal UI showing workflow progress
- **📝 Detailed Logging** - Comprehensive logs with timestamps and structured output
- **🔄 Environment Variables** - Full support for workflow and step-level environment variables
- **⚡ Action Support** - Execute both `uses:` actions and `run:` commands
- **🎯 Expression Evaluation** - Support for GitHub Actions expressions (`${{ }}`)
- **🌳 Dependency Resolution** - Automatic job dependency and execution order calculation

## 🚀 Quick Start

### Prerequisites

⚠️ **Required Dependencies** - Both must be installed and running:

- **Go 1.19+** - [Install Go](https://golang.org/doc/install)
  - Verify: `go version`
- **Docker** - [Install Docker](https://docs.docker.com/get-docker/) 
  - Verify: `docker --version`
  - **Docker must be running** - Start Docker Desktop or `sudo systemctl start docker`
- **Git** (optional) - For cloning repositories

### Installation

```bash
# 1. Verify prerequisites
go version    # Should show Go 1.19+
docker --version && docker info  # Docker must be running

# 2. Clone the repository
git clone https://github.com/Neoxs/gogh.git
cd gogh

# 3. Build the binary
go build -o gogh ./cmd/runner

# Or install directly (still requires Docker running)
go install github.com/Neoxs/gogh/cmd/runner@latest
```

> **⚠️ Important**: Docker daemon must be running before executing workflows, as GoGH creates and manages Docker containers for job execution.

### Basic Usage

```bash
# First, ensure Docker is running
docker info  # Should show Docker system info without errors

# Run a workflow file
./gogh run .github/workflows/ci.yml

# Or if installed globally
runner run .github/workflows/ci.yml
```

**Common Issues:**
- `Cannot connect to the Docker daemon` → Start Docker Desktop or Docker service
- `docker: command not found` → Install Docker and add to PATH
- `permission denied` → On Linux, add user to docker group or use `sudo`

## 📖 Usage Examples

### Simple CI Workflow

Create a workflow file `.github/workflows/test.yml`:

```yaml
name: Test Workflow
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        
      - name: Setup Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '18'
          
      - name: Install dependencies
        run: npm install
        
      - name: Run tests
        run: npm test
```

Run it locally:

```bash
./gogh run .github/workflows/test.yml
```

### Multi-Job Workflow

```yaml
name: Build and Deploy
on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Build application
        run: |
          echo "Building application..."
          make build
          
  test:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run tests
        run: make test
        
  deploy:
    needs: [build, test]
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - name: Deploy to production
        run: echo "Deploying to production..."
```

## 🏗️ Architecture

GoGH is built with a modular architecture:

```
gogh/
├── cmd/runner/          # CLI entry point
├── container/           # Docker container management
├── internal/
│   ├── executor/        # Workflow execution engine
│   ├── workflow/        # YAML parsing and validation
│   ├── logging/         # Structured logging system
│   ├── display/         # Terminal UI and progress tracking
│   ├── environment/     # Environment variable management
│   ├── expressions/     # GitHub Actions expression evaluator
│   └── actions/         # Action resolution and execution
└── README.md
```

### Key Components

- **🎭 Executor Engine** - Orchestrates workflow execution with proper job dependency resolution
- **🐳 Container Manager** - Handles Docker container lifecycle, volume mounting, and command execution
- **📝 Logging System** - Multi-level logging with separate files for workflows and jobs
- **🖥️ Terminal Display** - Real-time progress updates with job and step status
- **🌍 Environment Manager** - Manages environment variables across workflow, job, and step scopes
- **⚡ Expression Evaluator** - Evaluates GitHub Actions expressions and context variables

## 🎯 Current Support

### ✅ Supported Features

- **Workflow Parsing** - Full YAML workflow parsing with validation
- **Job Execution** - Sequential job execution with dependency resolution
- **Docker Support** - Ubuntu runners (`ubuntu-latest`, `ubuntu-22.04`, `ubuntu-20.04`)
- **Environment Variables** - Workflow, job, and step-level environment variables
- **Actions** - Basic action execution (`uses:` syntax)
- **Run Commands** - Shell command execution (`run:` syntax)
- **Expression Evaluation** - `${{ }}` expressions with context access
- **Conditional Execution** - Basic `if:` condition support
- **Real-time Logging** - Structured logs with timestamps

### 🚧 Planned Features

- **Parallel Job Execution** - Run independent jobs concurrently
- **More Runners** - Windows and macOS runner support
- **Advanced Actions** - Full GitHub Actions marketplace compatibility
- **Secrets Management** - Local secrets and secure environment variables
- **Matrix Builds** - Strategy matrix support for multiple configurations
- **Caching** - Dependency and build caching
- **Artifacts** - Upload and download artifact support
- **Service Containers** - Database and service container support

## 🛠️ Configuration

### Runner Mapping

GoGH automatically maps GitHub runner types to Docker images:

| GitHub Runner | Docker Image |
|--------------|-------------|
| `ubuntu-latest` | `ubuntu:latest` |
| `ubuntu-22.04` | `ubuntu:22.04` |
| `ubuntu-20.04` | `ubuntu:20.04` |
| Custom images | Pass-through support |

### Environment Variables

GoGH supports all standard GitHub Actions environment variables:

- `GITHUB_WORKSPACE` - Workspace directory (`/workspace`)
- `GITHUB_REPOSITORY` - Repository name
- `GITHUB_SHA` - Commit SHA
- `GITHUB_REF` - Git reference
- `GITHUB_EVENT_NAME` - Event that triggered the workflow
- `GITHUB_ACTOR` - User who triggered the workflow

## 📁 Project Structure

When running workflows, GoGH expects this structure:

```
your-project/
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── deploy.yml
├── src/
├── tests/
└── gogh-logs/           # Generated log files
    └── workflow-TIMESTAMP/
        ├── workflow.log
        └── job-TIMESTAMP.log
```

## 🔍 Logging

GoGH provides comprehensive logging:

- **Workflow logs** - High-level workflow execution logs
- **Job logs** - Individual job execution with container details
- **Step logs** - Real-time output from each step
- **Structured format** - GitHub Actions compatible log format

Logs are stored in `gogh-logs/` with timestamps for easy debugging.

## 🤝 Contributing

We welcome contributions! Here's how you can help:

### Getting Started

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make your changes** and add tests
4. **Run tests**: `go test ./...`
5. **Commit changes**: `git commit -m 'Add amazing feature'`
6. **Push to branch**: `git push origin feature/amazing-feature`
7. **Open a Pull Request**

### Areas for Contribution

- 🚀 **New Features** - Implement planned features or suggest new ones
- 🐛 **Bug Fixes** - Help us squash bugs and improve stability
- 📖 **Documentation** - Improve docs, add examples, write tutorials
- 🧪 **Testing** - Add test cases and improve test coverage
- 🎨 **UI/UX** - Enhance the terminal display and user experience
- 🔧 **Actions Support** - Add support for more GitHub Actions

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 📞 Support

- 🐛 **Bug Reports** - [Open an issue](https://github.com/Neoxs/gogh/issues)
- 💡 **Feature Requests** - [Suggest a feature](https://github.com/Neoxs/gogh/issues)
- 💬 **Discussions** - [Join the conversation](https://github.com/Neoxs/gogh/discussions)

---