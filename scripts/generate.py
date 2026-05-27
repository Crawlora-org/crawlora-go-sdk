#!/usr/bin/env python3
import json
import os
import re
import shutil
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SPEC = ROOT / "openapi" / "public.json"
SPEC_PATH = Path(os.environ.get("CRAWLORA_OPENAPI_SPEC", DEFAULT_SPEC))
TAG_GROUP_OVERRIDES = {
    "AppStore": "AppStore",
    "CoinGecko": "CoinGecko",
    "GooglePlay": "GooglePlay",
    "ProductHunt": "ProductHunt",
    "SimilarWeb": "SimilarWeb",
    "SpotifyPodcasts": "SpotifyPodcasts",
    "TikTok": "TikTok",
    "YouTube": "YouTube",
}
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


def words(value):
    value = re.sub(r"([a-z0-9])([A-Z])", r"\1-\2", value)
    return [part for part in re.split(r"[^A-Za-z0-9]+", value.lower()) if part]


def pascal(parts):
    return "".join(part[:1].upper() + part[1:] for part in parts) or "Call"


def alias(operation_id, tag, used):
    op_words = words(operation_id)
    tag_words = words(TAG_PREFIX_OVERRIDES.get(tag, tag))
    if op_words[: len(tag_words)] == tag_words:
        op_words = op_words[len(tag_words) :]
    name = pascal(op_words)
    if not name or name in used:
        name = pascal(words(operation_id))
    base = name
    i = 2
    while name in used:
        name = f"{base}{i}"
        i += 1
    used.add(name)
    return name


def go_string(value):
    return json.dumps(value)


def go_string_slice(values):
    return "[]string{" + ", ".join(go_string(v) for v in values) + "}"


def enum_values(param):
    values = param.get("enum") or param.get("items", {}).get("enum") or []
    return [str(v) for v in values]


def go_identifier(value):
    name = pascal(words(value))
    if not name:
        name = "Value"
    if name[0].isdigit():
        name = "Value" + name
    return name


def schema_type_name(value):
    return "Model" + go_identifier(value)


def ref_type(ref):
    return schema_type_name(ref.rsplit("/", 1)[-1]) if ref else "any"


def go_schema_type(schema):
    if not schema:
        return "any"
    if "$ref" in schema:
        return ref_type(schema["$ref"])
    if "allOf" in schema:
        parts = [go_schema_type(part) for part in schema.get("allOf", [])]
        concrete = [part for part in parts if part != "any"]
        return concrete[0] if len(concrete) == 1 else "any"
    typ = schema.get("type", "")
    if typ == "integer":
        return "int"
    if typ == "number":
        return "float64"
    if typ == "boolean":
        return "bool"
    if typ == "array":
        return "[]" + go_schema_type(schema.get("items", {"type": "string"}))
    if typ == "object":
        if schema.get("additionalProperties"):
            additional = schema.get("additionalProperties")
            value_type = go_schema_type(additional) if isinstance(additional, dict) else "any"
            return "map[string]" + value_type
        return "map[string]any"
    return "string" if typ == "string" or schema.get("enum") else "any"


def go_type(param):
    if param.get("in") == "body":
        return go_schema_type(param.get("schema"))
    typ = param.get("type", "")
    if param.get("in") == "formData" and typ == "file":
        return "any"
    if typ == "integer":
        return "int"
    if typ == "number":
        return "float64"
    if typ == "boolean":
        return "bool"
    if typ == "array":
        item_type = go_type(param.get("items", {"type": "string"}))
        return "[]" + item_type
    return "string"


def typed_field(param, used):
    base = go_identifier(param["name"])
    name = base
    i = 2
    while name in used:
        name = f"{base}{i}"
        i += 1
    used.add(name)
    typ = go_type(param)
    optional = not param.get("required") and not typ.startswith("[]") and typ != "any"
    if optional:
        typ = "*" + typ
    return name, typ, param["name"], optional


def param_slice(params):
    if not params:
        return "nil"
    items = []
    for param in params:
        collection = param.get("collectionFormat", "")
        typ = param.get("type", "")
        required = "true" if param.get("required") else "false"
        items.append(
            "parameterDefinition{Name: "
            + go_string(param["name"])
            + ", In: "
            + go_string(param.get("in", ""))
            + ", CollectionFormat: "
            + go_string(collection)
            + ", Type: "
            + go_string(typ)
            + ", Required: "
            + required
            + ", Enum: "
            + go_string_slice(enum_values(param))
            + "}"
        )
    return "[]parameterDefinition{" + ", ".join(items) + "}"


def definition(method, path, operation):
    params = operation.get("parameters", [])
    security = []
    for requirement in operation.get("security", []):
        security.extend(requirement.keys())
    path_params = [p["name"] for p in params if p.get("in") == "path"]
    query_params = [p for p in params if p.get("in") == "query"]
    form_params = [p for p in params if p.get("in") == "formData"]
    body_param = next((p["name"] for p in params if p.get("in") == "body"), "")
    return (
        "operationDefinition{"
        f"Method: {go_string(method.upper())}, "
        f"Path: {go_string(path)}, "
        f"PathParams: {go_string_slice(path_params)}, "
        f"QueryParams: {param_slice(query_params)}, "
        f"FormParams: {param_slice(form_params)}, "
        f"BodyParam: {go_string(body_param)}, "
        f"BodyRequired: {'true' if any(p.get('in') == 'body' and p.get('required') for p in params) else 'false'}, "
        f"Consumes: {go_string_slice(operation.get('consumes', []))}, "
        f"Produces: {go_string_slice(operation.get('produces', []))}, "
        f"Security: {go_string_slice(security)}, "
        "}"
    )


def operation_params(operation):
    return [
        p
        for p in operation.get("parameters", [])
        if p.get("in") in {"path", "query", "formData", "body"}
    ]


def response_ref(operation):
    schema = operation.get("responses", {}).get("200", {}).get("schema") or {}
    ref = schema.get("$ref", "")
    return ref.rsplit("/", 1)[-1] if ref else ""


def response_type(operation):
    schema = operation.get("responses", {}).get("200", {}).get("schema") or {}
    return go_schema_type(schema)


def model_field(param_name, schema, required, used):
    base = go_identifier(param_name)
    name = base
    i = 2
    while name in used:
        name = f"{base}{i}"
        i += 1
    used.add(name)
    tag = param_name if required else param_name + ",omitempty"
    return name, go_schema_type(schema), tag


def model_definitions(definitions):
    lines = []
    for schema_name, schema in sorted(definitions.items()):
        type_name = schema_type_name(schema_name)
        if schema.get("type") == "object" and schema.get("properties"):
            required = set(schema.get("required") or [])
            used = set()
            lines.append(f"type {type_name} struct {{")
            for prop_name, prop_schema in sorted(schema.get("properties", {}).items()):
                field_name, field_type, tag = model_field(prop_name, prop_schema, prop_name in required, used)
                lines.append(f"\t{field_name} {field_type} `json:{go_string(tag)}`")
            lines.append("}")
            lines.append("")
            continue
        lines.append(f"type {type_name} = {go_schema_type(schema)}")
        lines.append("")
    return lines


def main():
    if not SPEC_PATH.exists():
        raise SystemExit(f"public OpenAPI spec not found: {SPEC_PATH}")
    spec = json.loads(SPEC_PATH.read_text())
    (ROOT / "openapi").mkdir(exist_ok=True)
    target_spec = ROOT / "openapi" / "public.json"
    if SPEC_PATH.resolve() != target_spec.resolve():
        shutil.copyfile(SPEC_PATH, target_spec)

    operations = {}
    typed_operations = {}
    groups = {}
    used_by_group = {}
    for path, methods in sorted(spec["paths"].items()):
        for method, operation in sorted(methods.items()):
            operation_id = operation["operationId"]
            tag = (operation.get("tags") or ["Default"])[0]
            group_name = TAG_GROUP_OVERRIDES.get(tag, pascal(words(tag)))
            groups.setdefault(group_name, {})
            used_by_group.setdefault(group_name, set())
            method_name = alias(operation_id, tag, used_by_group[group_name])
            groups[group_name][method_name] = operation_id
            operations[operation_id] = definition(method, path, operation)
            typed_operations[operation_id] = {
                "type_base": group_name + method_name,
                "params": operation_params(operation),
                "response_type": response_type(operation),
            }

    lines = [
        "package crawlora",
        "",
        'import "context"',
        "",
        "// Generated by scripts/generate.py. Do not edit manually.",
        "",
        "type parameterDefinition struct {",
        "\tName string",
        "\tIn string",
        "\tCollectionFormat string",
        "\tType string",
        "\tRequired bool",
        "\tEnum []string",
        "}",
        "",
        "type operationDefinition struct {",
        "\tMethod string",
        "\tPath string",
        "\tPathParams []string",
        "\tQueryParams []parameterDefinition",
        "\tFormParams []parameterDefinition",
        "\tBodyParam string",
        "\tBodyRequired bool",
        "\tConsumes []string",
        "\tProduces []string",
        "\tSecurity []string",
        "}",
        "",
        *model_definitions(spec.get("definitions", {})),
        f"const operationCount = {sum(len(methods) for methods in spec['paths'].values())}",
        "",
        "var operations = map[string]operationDefinition{",
    ]
    for operation_id, body in operations.items():
        lines.append(f"\t{go_string(operation_id)}: {body},")
    lines.extend(["}", "", "type Services struct {"])
    for group_name in groups:
        lines.append(f"\t{group_name} *{group_name}Service")
    lines.extend(["}", "", "func initServices(c *Client) Services {", "\treturn Services{"])
    for group_name in groups:
        lines.append(f"\t\t{group_name}: &{group_name}Service{{client: c}},")
    lines.extend(["\t}", "}", ""])
    for group_name, methods in groups.items():
        lines.append(f"type {group_name}Service struct {{ client *Client }}")
        lines.append("")
        for method_name, operation_id in methods.items():
            lines.append(f"func (s *{group_name}Service) {method_name}(ctx context.Context, params Params, opts ...RequestOption) (any, error) {{")
            lines.append(f"\treturn s.client.Request(ctx, {go_string(operation_id)}, params, opts...)")
            lines.append("}")
            lines.append("")
            type_base = typed_operations[operation_id]["type_base"]
            lines.append(f"type {type_base}Params struct {{")
            used_fields = set()
            for param in typed_operations[operation_id]["params"]:
                field_name, field_type, param_name, optional = typed_field(param, used_fields)
                tag = param_name + (",omitempty" if optional else "")
                lines.append(f"\t{field_name} {field_type} `crawlora:{go_string(tag)}`")
            lines.append("}")
            lines.append("")
            lines.append(f"type {type_base}Response = {typed_operations[operation_id]['response_type']}")
            lines.append("")
            lines.append(f"func (s *{group_name}Service) {method_name}Typed(ctx context.Context, params {type_base}Params, opts ...RequestOption) ({type_base}Response, error) {{")
            lines.append(f"\treturn requestTyped[{type_base}Response](s.client, ctx, {go_string(operation_id)}, paramsFromStruct(params), opts...)")
            lines.append("}")
            lines.append("")
    (ROOT / "operations_generated.go").write_text("\n".join(lines))


if __name__ == "__main__":
    main()
