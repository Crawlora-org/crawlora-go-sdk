# Changelog

## v1.22.0-sdk.1

- Regenerated from the public API contract (784 to 787 operations). Adds Threads search, cursor-paginated profile posts, and public post replies.

## v1.21.0-sdk.1

- Regenerated from the public API contract (782 to 784 operations). Adds credential-free Threads public profile and single-post lookups.

## v1.20.0-sdk.2

- Validate all supported ATS values for jobs dataset provider filters, including iCIMS and Eightfold.

## v1.20.0-sdk.1

- Regenerated from the public API contract (776 to 782 operations). Adds jobs dataset company detail and nearby search, plus iCIMS and Eightfold board and job operations.

## v1.19.0-sdk.1

- Regenerated from the public API contract (737 to 776 operations). Adds **7 more
  ATS job-board providers** — Workable, Recruitee, Rippling, Personio, Teamtailor,
  Oracle Recruiting Cloud, and UKG Pro — bringing the `/jobs` family to 12 providers,
  plus the **jobs dataset** (`/datasets/jobs` search, item, facets, companies) over
  live postings crawled from every discovered company ATS board.

## v1.18.0-sdk.1

- Regenerated from the public API contract (697 to 776 operations). Adds
  **Capterra** software discovery and reviews (3), **Metacritic** games, movies,
  TV titles and reviews (10), **Numbeo** cost-of-living and quality indices (8),
  and **Walmart** search, products, and reviews (3).
- Adds 16 dataset operations: Chrome Web Store extension search, facets,
  history, metrics, changes, and trending (7); journalist discovery (3);
  Numbeo city and country search (5); and TrustMRR revenue history (1).

## v1.17.0-sdk.1

- Regenerated from the public API contract (685 to 697 operations). Adds two
  credential-free platforms (12 endpoints):
  **Anime** (9) — search, details, characters, staff, recommendations, rankings,
  the upcoming airing schedule, plus character lookup and search.
  **Manga** (3) — search, details, and rankings.
  Both draw on AniList's public catalog: scores, popularity, favourites, genres,
  ranked tags, studios, and MyAnimeList cross-reference ids.

## v1.16.0-sdk.1

- Regenerated from the public API contract (658 to 685 operations). Adds four
  credential-free media platforms (27 endpoints):
  **Discogs** (7) — release, master, artist and artist releases, label and label
  releases, and search across the Discogs music database.
  **Letterboxd** (8) — film details, rating histogram, reviews, similar films,
  search, person, popular films, and member profiles.
  **TMDB** (6) — movie, TV, and person details, multi-search, and curated
  movie/TV lists from The Movie Database.
  **Goodreads** (6) — book details and reviews, search, author details and
  author books, and Listopia lists.
  All over credential-free public pages and JSON endpoints.

## v1.15.0-sdk.1

- Regenerated from the public API contract (603 to 625 operations). Adds the
  **Jobs platform** (11 endpoints): public ATS job boards across Greenhouse,
  Lever, Ashby, Workday, and SmartRecruiters -- board listings and single
  postings, a company hiring-signals aggregate (open roles, department and
  location breakdowns, remote share, and newly-posted trends), and cross-provider
  company search. Adds the **Steam platform** (12 endpoints): app, package,
  reviews and review histogram, search and search results, featured and featured
  categories, player counts, achievements, news, and SteamSpy stats. SEC
  company-intelligence now supports opt-in cross-source enrichment (market quote,
  news, and hiring signals) via the `enrich` parameter. All over credential-free
  public data.


## v1.14.0-sdk.1

- Regenerated from the public API contract (559 to 603 operations). Adds the
  **SEC EDGAR platform** (10 endpoints): company search, filings list, single
  filing, 10-K/10-Q/8-K section extraction, full-text search, XBRL frames,
  normalized financial statements (income/balance/cash-flow with computed
  margins and ratios), insider transactions (Forms 3/4/5), 13F institutional
  holdings, and a company-intelligence overview -- all over credential-free
  official SEC data. Also catches up accumulated public-contract coverage that
  had drifted since the last regeneration.


## v1.13.0-sdk.1

- Regenerated from the public API contract (555 to 559 operations). Adds the
  **Chrome Web Store** platform (9 endpoints): item detail, search, related
  items, reviews, category / collection / top-chart listings, search
  suggestions, and the category reference taxonomy — covering Chrome Web Store
  extensions and themes.


## v1.12.0-sdk.1

- Regenerated from the public API contract (532 to 555 operations). Adds the
  **TrustMRR** platform (5 endpoints): a public database of verified startup
  revenues and a startup-acquisition marketplace. The endpoints cover the
  marketplace snapshot (recently listed startups and best deals), the verified
  revenue leaderboard (rank by MRR, 30-day revenue, all-time revenue, growth,
  traffic, or revenue per visitor), startup detail, the category directory, and
  category detail.
- Also catches the client up with public endpoints from earlier API releases that
  had not yet been regenerated into the SDKs: the ESPN and Reddit platforms; the
  Airbnb Markets, GitHub Users, and Product Hunt dataset families; Product Hunt
  category products; and the website tech-stack endpoint.

## v1.11.0-sdk.1

- Regenerated from the public API contract (529 to 532 operations). Adds three
  Airbnb host endpoints: host profile, host listings, and host reviews.

## v1.10.0-sdk.1

- Regenerated from the public API contract (525 to 529 operations). Adds the
  **Airbnb Markets dataset** (4 endpoints): aggregate short-term-rental market
  statistics -- listing supply, Superhost share, ratings, and nightly-price bands
  -- rolled up by country, metro, and geo cell (search, item lookup, facets, and
  nearby density). Aggregate-only: no individual listings or hosts.

## v1.9.0-sdk.1

- Regenerated from the public API contract (499 to 525 operations). Adds four
  platforms/families to the client:
  - **GitHub** (16 endpoints): organizations, repositories (contributors,
    forks, languages, releases, stargazers), user profiles/events/pinned/repos,
    repository and user search, and trending repositories/developers.
  - **GitHub Users dataset** (4): search, facets, nearby, and item lookup.
  - **X** (3): post, profile, and profile posts.
  - **Apps datasets** (3): apps, apps-charts, and apps-reviews search.
  - **Creators dataset** (1): TikTok creators search.
- Removes the retired tiktok popular-trend/creator operation.

## v1.8.0-sdk.2

- Regenerated from the public API contract (499 operations, unchanged). Enriches
  the Web `antibot-check` diagnostic response with additional fields:
  `block_reason`, `block_detail`, `auth_required`, `captcha_type`,
  `captcha_types`, `captcha_mode`, `confidence_score`, `custom_vm`, and
  `vm_vendor`.
- Clarified the `google-search` and datasets `google-map-businesses/search`
  endpoint descriptions (wording only; no behavior change).

## v1.8.0-sdk.1

- Added two new platforms, regenerated from the public API contract (now 499
  operations): **Redfin** (real-estate `search`, `property`, `estimate`,
  `region-trends`, `similar`) and **Web** (generic `web-scrape`, `contact`, and
  the `antibot-check` diagnostic).
- Refreshed response schemas: `contact` gains `crawl_status`, `web-scrape` gains
  `cache_state`/`cached_at`/`max_age`, and the Spotify country-hub responses gain
  `partialErrors`.

## v1.7.0-sdk.1

- Added six new platforms, regenerated from the public API contract (now 491
  operations): **Polymarket**, **Kalshi**, and **Metaculus** (prediction
  markets); **IMDb**, **Rotten Tomatoes**, and **Box Office Mojo** (film/TV).
- Expanded **Reddit**: subreddit about/comments, multi-subreddit posts,
  domain posts, user posts/comments, and trends.

## v1.6.0-sdk.1

- Added the **Reddit** platform (`Reddit.Search`, `Reddit.Post`,
  `Reddit.Comments`, `Reddit.SubredditPosts`) and the **Brand** platform
  (`Brand.Retrieve`), plus Yahoo Finance `YahooFinance.Lookup`. Regenerated from
  the public API contract.

## v1.5.0-sdk.1

- Added `WithBeforeRequest`/`WithAfterResponse` middleware hooks (mutate the
  `*http.Request`, transform the parsed response).
- Added `WithIdempotencyKeys` (stable `Idempotency-Key` on POST/PATCH, reused
  across retries) and per-request `WithRequestRetries`/`WithRequestRetryPredicate`.
- Added `WithRateLimit` (requests/sec) and `WithMaxConcurrency` throttling.
- Errors now carry `RequestID`.

## v1.4.0-sdk.1

- Configurable retries: `WithMaxRetryDelay`, `WithRetryableStatuses`, and
  `WithRetryPredicate`; added `WithOnRetry`, `WithRequestID` (x-request-id, also
  on `(*Error).RequestID`), and `WithLogger`.
- Pagination: cursor/token mode (`WithCursorParam` + `WithNextCursor`) and
  `Client.PaginateItems`; operation metadata now carries `Paginatable` and
  `CursorParams`.
- Streaming: `WithResponseType(crawlora.ResponseStream)` returns an
  `io.ReadCloser` on success.
- Config: `CRAWLORA_API_KEY` / `CRAWLORA_BASE_URL` environment fallback.

## v1.3.0-sdk.1

- Added the `ErrClient`, `ErrServer`, and `ErrNetwork` sentinels plus
  `(*Error).IsClientError`, `IsServerError`, and `IsNetworkError`, so failures can
  be classified with `errors.Is(err, crawlora.ErrServer)`.
- Added `Client.Paginate`, which walks page/offset endpoints with a per-page
  callback and stops on an empty page; return `ErrStopPagination` to stop early.
- The generator now shares a single language-neutral core with the TypeScript and
  Python SDKs; generated output is unchanged.

## v1.2.0-sdk.19

- Regenerated the public SDK contract with the promoted Shopify endpoint family.
- Added the generated `Shopify` group with 11 active Shopify operations.
- Updated the generated operation reference to 330 public SDK operations.

## v1.2.0-sdk.16

- Documented response headers on SDK errors, case-insensitive header overrides,
  strict response modes, `Retry-After` retries, and context cancellation
  behavior.
- Added docs coverage checks for the release-polish behavior.
- Kept the generated operation contract unchanged.

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
