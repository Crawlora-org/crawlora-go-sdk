"""Canonical, language-neutral core for the Crawlora SDK generators.

This module owns every part of SDK generation that does not depend on the
target language: tokenizing identifiers, grouping operations by tag, aliasing
method names, building the normalized runtime operation metadata, and rendering
the shared `docs/operations.md` table.

Each SDK repository vendors a byte-identical copy at `scripts/_sdkgen/core.py`
and supplies only a small `NamingPolicy` (casing + dedup rules) plus its
language-specific type mappers and code emitters. The canonical copy lives in
the API repo at `tools/sdkgen/core.py`; `make sdk-sync` copies it into the SDK
repos and `make sdk-sync-check` fails if any vendored copy drifts.

Do not add language-specific logic here. Keep type mappers and emitters in the
per-repo `scripts/generate.py`.
"""

from __future__ import annotations

import re
from dataclasses import dataclass, field
from typing import Any, Callable

# Tag-name prefixes stripped from operation ids when deriving method aliases.
# These values are language-neutral (lowercase, hyphenated) and identical across
# every SDK. Group-name casing differs per language and lives in NamingPolicy.
TAG_PREFIX_OVERRIDES = {
    "AppStore": "appstore",
    "CoinGecko": "coingecko",
    "GooglePlay": "googleplay",
    "ProductHunt": "producthunt",
    "SimilarWeb": "similarweb",
    "SpotifyPodcasts": "spotify-podcasts",
    "TikTok": "tiktok",
    "YouTube": "youtube",
}

# Parameter locations that become typed method arguments.
TYPED_PARAM_LOCATIONS = {"path", "query", "formData", "body"}


def words(value: str) -> list[str]:
    """Split an identifier into lowercase word parts.

    Handles camelCase, PascalCase, snake_case, kebab-case, and digits.
    """
    value = re.sub(r"([a-z0-9])([A-Z])", r"\1-\2", value)
    return [part for part in re.split(r"[^A-Za-z0-9]+", value.lower()) if part]


def type_name(*parts: str) -> str:
    """PascalCase type base shared by all languages, e.g. ('google_play',
    'search') -> 'GooglePlaySearch'. Returns 'Operation' for empty input."""
    raw = "".join(part[:1].upper() + part[1:] for part in words("-".join(parts)))
    return raw or "Operation"


def enum_values(param: dict) -> list[str]:
    """Stringified enum values for a parameter, covering both direct `enum`
    and array `items.enum`."""
    return [str(value) for value in (param.get("enum") or param.get("items", {}).get("enum") or [])]


def operation_note(operation_id: str, operation: dict) -> str:
    """Doc note for special response modes."""
    produces = [str(item).lower() for item in operation.get("produces", [])]
    if "text/plain" in produces or operation_id == "youtube-transcript":
        return "Supports text response mode."
    return ""


def md_escape(value: Any) -> str:
    return str(value).replace("|", "\\|").replace("\n", " ")


def md_code(value: Any) -> str:
    return f"`{md_escape(value)}`"


@dataclass(frozen=True)
class NamingPolicy:
    """Per-language casing and identifier rules.

    case_fn:               turn word parts into a group/method name
    dedup_sep:             separator inserted before the numeric dedup suffix
    tag_group_overrides:   tag -> group name overrides (already language-cased)
    keywords:              reserved words the target language forbids as a method
                           name; a generated name that collides gets a '_' suffix.
                           Each emitter supplies its own set (Python passes
                           keyword.kwlist, Ruby/Java/PHP their own); empty by
                           default so untyped/keyword-free targets opt out.
    """

    case_fn: Callable[[list[str]], str]
    dedup_sep: str = ""
    tag_group_overrides: dict[str, str] = field(default_factory=dict)
    keywords: frozenset[str] = frozenset()
    # How a group + method become the PascalCase type base for generated type
    # names. Defaults to re-tokenizing both (TS/Python). Go concatenates the
    # already-cased group and method verbatim to preserve internal caps such as
    # the "EBay" group, so it overrides this.
    type_base_fn: Callable[[str, str], str] | None = None

    def group_name(self, tag: str) -> str:
        return self.tag_group_overrides.get(tag, self.case_fn(words(tag)))

    def type_base(self, group: str, method: str) -> str:
        if self.type_base_fn is not None:
            return self.type_base_fn(group, method)
        return type_name(group, method)

    def alias(self, operation_id: str, tag: str, used: set[str]) -> str:
        op_words = words(operation_id)
        tag_words = words(TAG_PREFIX_OVERRIDES.get(tag, tag))
        if op_words[: len(tag_words)] == tag_words:
            op_words = op_words[len(tag_words):]
        name = self.case_fn(op_words)
        if not name or name in used:
            name = self.case_fn(words(operation_id))
        if name in self.keywords:
            name += "_"
        base = name
        i = 2
        while name in used:
            name = f"{base}{self.dedup_sep}{i}"
            i += 1
        used.add(name)
        return name


# Numeric page parameters the SDK pagination helpers can auto-increment, and
# token/cursor parameters that need a caller-supplied next-cursor extractor.
NUMERIC_PAGE_PARAMS = ("page", "offset")
CURSOR_PARAMS = ("cursor", "page_token", "next", "start")


def _is_paginatable(params: list[dict]) -> bool:
    """An operation is paginatable when it exposes a page/offset/cursor-style
    query parameter that the SDK pagination helpers can advance."""
    names = {p.get("name", "").lower() for p in params if p.get("in") == "query"}
    return any(name in names for name in NUMERIC_PAGE_PARAMS + CURSOR_PARAMS)


def _cursor_params(params: list[dict]) -> list[str]:
    """Names of token/cursor-style query parameters, in spec order, for cursor
    pagination."""
    return [
        p["name"]
        for p in params
        if p.get("in") == "query" and p.get("name", "").lower() in CURSOR_PARAMS
    ]


def operation_definition(operation_id: str, method: str, path: str, operation: dict) -> dict:
    """Normalized, language-neutral runtime metadata for one operation.

    This is the exact dict TypeScript and Python serialize directly; the Go
    emitter renders it into a Go struct literal. Keep field names and shapes
    stable: the SDK clients read this at runtime for validation and dispatch.
    """
    params = operation.get("parameters", [])
    security: list[str] = []
    for requirement in operation.get("security", []):
        security.extend(requirement.keys())
    return {
        "id": operation_id,
        "method": method.upper(),
        "path": path,
        "pathParams": [p["name"] for p in params if p.get("in") == "path"],
        "queryParams": [
            {
                "name": p["name"],
                "in": "query",
                **({"collectionFormat": p["collectionFormat"]} if "collectionFormat" in p else {}),
                **({"type": p["type"]} if "type" in p else {}),
                **({"required": True} if p.get("required") else {}),
                **({"enum": enum_values(p)} if enum_values(p) else {}),
            }
            for p in params
            if p.get("in") == "query"
        ],
        "formParams": [
            {
                "name": p["name"],
                "in": "formData",
                **({"type": p["type"]} if "type" in p else {}),
                **({"required": True} if p.get("required") else {}),
                **({"enum": enum_values(p)} if enum_values(p) else {}),
            }
            for p in params
            if p.get("in") == "formData"
        ],
        "bodyParam": next((p["name"] for p in params if p.get("in") == "body"), None),
        "bodyRequired": any(p.get("in") == "body" and p.get("required") for p in params),
        "consumes": operation.get("consumes", []),
        "produces": operation.get("produces", []),
        "security": security,
        **({"paginatable": True} if _is_paginatable(params) else {}),
        **({"cursorParams": _cursor_params(params)} if _cursor_params(params) else {}),
    }


def typed_params(operation: dict) -> list[dict]:
    return [p for p in operation.get("parameters", []) if p.get("in") in TYPED_PARAM_LOCATIONS]


@dataclass
class Model:
    """The full normalized intermediate consumed by language emitters.

    groups:     ordered {group_name: {method_name: operation_id}}
    operations: ordered {operation_id: operation_definition dict}
    meta:       {operation_id: per-op typing metadata}
    definitions: raw OpenAPI `definitions` (schema models)
    operation_count: total operation count
    """

    groups: dict[str, dict[str, str]]
    operations: dict[str, dict]
    meta: dict[str, dict]
    definitions: dict[str, dict]
    operation_count: int


def build_model(spec: dict, policy: NamingPolicy) -> Model:
    """Walk the spec in sorted (path, method) order and produce the normalized
    Model. Iteration order is deterministic and identical across languages, so
    group/operation ordering in generated files stays stable."""
    groups: dict[str, dict[str, str]] = {}
    operations: dict[str, dict] = {}
    meta: dict[str, dict] = {}
    used_by_group: dict[str, set[str]] = {}

    for path, methods in sorted(spec["paths"].items()):
        for method, operation in sorted(methods.items()):
            operation_id = operation["operationId"]
            tag = (operation.get("tags") or ["default"])[0]
            group = policy.group_name(tag)
            groups.setdefault(group, {})
            used_by_group.setdefault(group, set())
            method_name = policy.alias(operation_id, tag, used_by_group[group])
            groups[group][method_name] = operation_id
            operations[operation_id] = operation_definition(operation_id, method, path, operation)
            params = typed_params(operation)
            body_schema = next((p.get("schema") for p in operation.get("parameters", []) if p.get("in") == "body"), None)
            response_schema = operation.get("responses", {}).get("200", {}).get("schema")
            meta[operation_id] = {
                "type_base": policy.type_base(group, method_name),
                "group": group,
                "method": method.upper(),
                "path": path,
                "params": params,
                "body_schema": body_schema,
                "response_schema": response_schema,
                "has_required_params": any(p.get("required") for p in params),
                "paginatable": _is_paginatable(operation.get("parameters", [])),
                "security": [key for req in operation.get("security", []) for key in req.keys()],
                "note": operation_note(operation_id, operation),
            }

    return Model(
        groups=groups,
        operations=operations,
        meta=meta,
        definitions=spec.get("definitions", {}),
        operation_count=sum(len(m) for m in spec["paths"].values()),
    )


def operation_docs(model: Model, *, title: str, type_render: Callable[[dict], str]) -> str:
    """Render the shared `docs/operations.md` table. `type_render` maps a
    parameter to its language type string for the Params column."""

    def param_doc(params: list[dict]) -> str:
        if not params:
            return "none"
        entries = []
        for param in params:
            required = " required" if param.get("required") else ""
            location = param.get("in", "param")
            entries.append(f"{md_code(param['name'])} ({location} {md_escape(type_render(param))}{required})")
        return "<br>".join(entries)

    def auth_doc(security: list[str]) -> str:
        return ", ".join(md_code(item) for item in security) if security else "none"

    lines = [
        f"# {title}",
        "",
        "Generated from `openapi/public.json`. Deprecated, admin, and internal operations are excluded from this SDK contract.",
        "",
        f"Total operations: `{model.operation_count}`",
        "",
        "| Group | SDK method | Operation ID | HTTP | Params | Auth | Response | Notes |",
        "| --- | --- | --- | --- | --- | --- | --- | --- |",
    ]
    for group_name, methods in model.groups.items():
        for method_name, operation_id in methods.items():
            m = model.meta[operation_id]
            lines.append(
                "| "
                + " | ".join(
                    [
                        md_escape(group_name),
                        md_code(f"{group_name}.{method_name}"),
                        md_code(operation_id),
                        md_code(f"{m['method']} {m['path']}"),
                        param_doc(m["params"]),
                        auth_doc(m["security"]),
                        md_code(m["type_base"] + "Response"),
                        md_escape(m["note"]),
                    ]
                )
                + " |"
            )
    lines.append("")
    return "\n".join(lines)
