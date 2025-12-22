// Package github provides types and utilities for GitHub PR review integration.
//
// This adapter layer handles GitHub-specific concerns without polluting the
// domain layer. Key types include:
//
//   - PositionedFinding: Wraps domain.Finding with GitHub diff position
//   - MapFindings: Enriches findings with diff positions for inline comments
//
// The design keeps the domain layer pure and platform-agnostic, enabling
// future support for GitLab, Bitbucket, or other platforms.
package github
