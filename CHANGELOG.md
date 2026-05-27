# Changelog

## v1.2.0-sdk.9

- Added fail-fast enum validation for generated query and form parameters.
- Wrapped malformed JSON responses in SDK errors with response status and raw
  body details.

## v1.2.0-sdk.8

- Regenerated from the SDK spec that excludes deprecated endpoints.
- Removed the deprecated Google Lens example and generated SDK surface.

## v1.2.0-sdk.7

- Added fail-fast validation for required query, form, and body parameters.
- Normalized negative retry settings and made JWT auth scheme detection
  case-insensitive.

## v1.2.0-sdk.6

- Added runnable Bing search, YouTube transcript, and Google Lens upload
  examples.
- Documented optional live smoke-test commands without requiring live API
  credentials in default tests.

## v1.2.0-sdk.5

- Prepared the SDK for future module and registry-facing documentation.
- Added package documentation and refreshed beta install references.

## v1.2.0-sdk.4

- Added release-readiness files, CI, license, and fuller public README guidance.
- Kept endpoint behavior and generated operation contract unchanged.

## v1.2.0-sdk.3

- Added generated typed endpoint parameter structs and typed service variants.

## v1.2.0-sdk.2

- Improved retries, request options, user agent handling, multipart support,
  response parsing, and SDK error details.

## v1.2.0-sdk.1

- Cleaned public SDK docs to avoid maintainer-only generation details.

## Initial SDK

- Added the first Git-installable Crawlora Go SDK generated from the public API
  contract.
