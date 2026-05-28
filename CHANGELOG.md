# Changelog

## v1.2.0-sdk.15

- Added strict response mode validation, response headers on SDK errors, and
  `Retry-After` aware retry delays capped at 30 seconds.
- Added coverage for case-insensitive request header overrides across auth and
  content headers.
- Preserved context cancellation and deadline errors instead of wrapping them as
  transport failures.

## v1.2.0-sdk.14

- Aligned the promoted SDK beta tag with the JavaScript and Python SDKs.
- Let request-level headers override generated auth and content headers.

## v1.2.0-sdk.12

- Added generated public operation reference docs and usage recipes.
- Refreshed examples and README links for typed dynamic operation calls.

## v1.2.0-sdk.11

- Added generated operation id constants and a public generic `RequestTyped`
  helper for typed dynamic calls.
- Added small pointer helpers for typed optional params such as
  `crawlora.Int(10)`.

## v1.2.0-sdk.10

- Generated OpenAPI schema model structs for typed endpoint responses and body
  parameters.
- Updated typed service methods to decode JSON responses into concrete response
  aliases while keeping dynamic methods unchanged.

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
