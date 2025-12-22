// Package github provides types and utilities for GitHub PR review integration.
//
// This adapter layer handles GitHub-specific concerns without polluting the
// domain layer. Key components include:
//
// # Types
//   - PositionedFinding: Wraps domain.Finding with GitHub diff position
//   - ReviewEvent: Review actions (APPROVE, REQUEST_CHANGES, COMMENT)
//   - CreateReviewRequest/Response: GitHub API request/response types
//
// # Functions
//   - MapFindings: Enriches findings with diff positions for inline comments
//   - BuildReviewComments: Converts positioned findings to review comments
//   - DetermineReviewEvent: Determines event based on finding severities
//   - MapHTTPError: Maps GitHub HTTP errors to typed llmhttp.Error
//
// # Client
//   - Client: HTTP client for GitHub Pull Request Reviews API
//   - CreateReview: Posts a review with inline comments
//
// The design keeps the domain layer pure and platform-agnostic, enabling
// future support for GitLab, Bitbucket, or other platforms.
package github
