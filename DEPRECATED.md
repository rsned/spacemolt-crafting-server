# Deprecated Functionality

This document tracks functionality that has been migrated elsewhere and can be removed from this repository.

## Catalog Generation (Migrated to `/home/robert/spacemolt/kb/`)

The following catalog generation functionality has been migrated to the SpaceMolt knowledge base repository and should be removed:

### Files/Directories to Remove

- `cmd/generate-catalog/` - Command that generates markdown and HTML catalog pages from the database
- `.github/workflows/deploy-catalog.yml` - GitHub Actions workflow that deploys catalog to GitHub Pages
- `catalog/` - Generated catalog documentation (directory with all subdirectories)

### Reason for Migration

The catalog generation and documentation has been centralized in the SpaceMolt knowledge base repository at `/home/robert/spacemolt/kb/`. This provides a single source of truth for game documentation.

### Removal Timeline

This functionality can be removed at any time. Verify that the knowledge base repository has working catalog generation before removing.

### Post-Removal Cleanup

After removal, also verify no references to `generate-catalog` remain in:
- Documentation files (README.md, etc.)
- Build scripts or Makefiles
- CI/CD configurations
