#!/usr/bin/env python3
"""Go SDK emitter.

Language-neutral spec parsing, grouping, aliasing, and the operations docs table
live in the vendored `scripts/_sdkgen/core.py` (synced from the API repo). This
file only maps OpenAPI schemas to Go types and renders the Go artifacts:
`operations_generated.go` (runtime metadata + typed service methods).
"""
import json
import os
import shutil
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from _sdkgen import core  # noqa: E402

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SPEC = ROOT / "openapi" / "public.json"
SPEC_PATH = Path(os.environ.get("CRAWLORA_OPENAPI_SPEC", DEFAULT_SPEC))

POLICY = core.NamingPolicy(
    case_fn=lambda parts: "".join(p[:1].upper() + p[1:] for p in parts) or "Call",
    dedup_sep="",
    type_base_fn=lambda group, method: group + method,
    tag_group_overrides={
        "AppStore": "AppStore",
        "CoinGecko": "CoinGecko",
        "GooglePlay": "GooglePlay",
        "ProductHunt": "ProductHunt",
        "SimilarWeb": "SimilarWeb",
        "SpotifyPodcasts": "SpotifyPodcasts",
        "TikTok": "TikTok",
        "YouTube": "YouTube",
    },
)


def go_string(value):
    return json.dumps(value)


def go_string_slice(values):
    return "[]string{" + ", ".join(go_string(v) for v in values) + "}"


def go_identifier(value):
    name = "".join(part[:1].upper() + part[1:] for part in core.words(value))
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
            + go_string_slice(core.enum_values(param))
            + "}"
        )
    return "[]parameterDefinition{" + ", ".join(items) + "}"


def go_definition(op):
    """Render a core operation_definition dict into a Go operationDefinition literal."""
    fields = (
        "operationDefinition{"
        f"Method: {go_string(op['method'])}, "
        f"Path: {go_string(op['path'])}, "
        f"PathParams: {go_string_slice(op['pathParams'])}, "
        f"QueryParams: {param_slice(op['queryParams'])}, "
        f"FormParams: {param_slice(op['formParams'])}, "
        f"BodyParam: {go_string(op['bodyParam'] or '')}, "
        f"BodyRequired: {'true' if op['bodyRequired'] else 'false'}, "
        f"Consumes: {go_string_slice(op['consumes'])}, "
        f"Produces: {go_string_slice(op['produces'])}, "
        f"Security: {go_string_slice(op['security'])}, "
    )
    if op.get("paginatable"):
        fields += "Paginatable: true, "
    if op.get("cursorParams"):
        fields += f"CursorParams: {go_string_slice(op['cursorParams'])}, "
    return fields + "}"


def operation_const_name(type_base):
    return "Operation" + type_base


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

    model = core.build_model(spec, POLICY)

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
        "\tPaginatable bool",
        "\tCursorParams []string",
        "}",
        "",
        *model_definitions(model.definitions),
        f"const operationCount = {model.operation_count}",
        "",
        "const (",
    ]
    for operation_id, meta in sorted(model.meta.items(), key=lambda item: item[1]["type_base"]):
        lines.append(f"\t{operation_const_name(meta['type_base'])} = {go_string(operation_id)}")
    lines.extend([
        ")",
        "",
        "var operations = map[string]operationDefinition{",
    ])
    for operation_id, op in model.operations.items():
        lines.append(f"\t{go_string(operation_id)}: {go_definition(op)},")
    lines.extend(["}", "", "type Services struct {"])
    for group_name in model.groups:
        lines.append(f"\t{group_name} *{group_name}Service")
    lines.extend(["}", "", "func initServices(c *Client) Services {", "\treturn Services{"])
    for group_name in model.groups:
        lines.append(f"\t\t{group_name}: &{group_name}Service{{client: c}},")
    lines.extend(["\t}", "}", ""])
    for group_name, methods in model.groups.items():
        lines.append(f"type {group_name}Service struct {{ client *Client }}")
        lines.append("")
        for method_name, operation_id in methods.items():
            meta = model.meta[operation_id]
            type_base = meta["type_base"]
            lines.append(f"func (s *{group_name}Service) {method_name}(ctx context.Context, params Params, opts ...RequestOption) (any, error) {{")
            lines.append(f"\treturn s.client.Request(ctx, {go_string(operation_id)}, params, opts...)")
            lines.append("}")
            lines.append("")
            lines.append(f"type {type_base}Params struct {{")
            used_fields = set()
            for param in meta["params"]:
                field_name, field_type, param_name, optional = typed_field(param, used_fields)
                tag = param_name + (",omitempty" if optional else "")
                lines.append(f"\t{field_name} {field_type} `crawlora:{go_string(tag)}`")
            lines.append("}")
            lines.append("")
            lines.append(f"type {type_base}Response = {go_schema_type(meta['response_schema'])}")
            lines.append("")
            lines.append(f"func (s *{group_name}Service) {method_name}Typed(ctx context.Context, params {type_base}Params, opts ...RequestOption) ({type_base}Response, error) {{")
            lines.append(f"\treturn requestTyped[{type_base}Response](s.client, ctx, {go_string(operation_id)}, paramsFromStruct(params), opts...)")
            lines.append("}")
            lines.append("")
    (ROOT / "operations_generated.go").write_text("\n".join(lines))
    (ROOT / "docs").mkdir(exist_ok=True)
    (ROOT / "docs" / "operations.md").write_text(
        core.operation_docs(model, title="Crawlora Go SDK Operations", type_render=go_type)
    )


if __name__ == "__main__":
    main()
