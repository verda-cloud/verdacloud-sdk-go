# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- feat(workflow): Establish changelog and automated release workflow
- feat(clusters): Integrate with cluster APIs

### Fixed
- fix(serverless-jobs): Fix serverless job validation

### Changed
- refactor(instances): Remove legacy support for instance
- test(integration): Improve integration tests

### Removed
- docs: Remove outdated coverage improvement plan

## [v1.1.3] - 2025-12-30
### Fixed
- fix(types): Change size of ComputeResource to an integer, add location and contract to volumes
- fix(serverless-jobs): Use correct type in scaling endpoints
### CI
- ci: add govulncheck to CI pipeline for branch protection

## [v1.1.2] - 2025-12-10
### Fixed
- fix(serverless-jobs): Change an incorrect scaling struct from container scaling options to job scaling options
### Other
- add release template

## [v1.1.1] - 2025-12-05
### Changed
- Container type restructuring

## [v1.1.0] - 2025-12-03
### Added
- feat!: Standardize location codes and complete serverless APIs
### Fixed
- Fix location_code in volume creation

## [v1.0.2] - 2025-11-19
### Fixed
- Fix instance actions to support single ID only and correct golangci-lint installation
- Fix GitHub workflows and upgrade to Go 1.24

## [v1.0.1] - 2025-11-17
### Fixed
- fix: resolve linting issues for CI compatibility

## [v1.0.0] - 2025-11-17
### Added
- Initial release
