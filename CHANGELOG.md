# Changelog
All notable changes to the `accord` project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added
- Defined `MirrorUpgradeRequest` CRD for the upgrader pipeline.
- Implemented YAML normalization and SHA-256 hash calculation for drift detection and loop breaking.
- Set up independent command entrypoints for `inventory-controller`, `sync-operator`, and `mirror-upgrader`.

### Changed
- Shifted initial architecture to a Webhook-based `sync-operator` to mitigate Argo CD self-heal race conditions.