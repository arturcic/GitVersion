#!/usr/bin/env python3
"""Validate pinned behavior mappings and render the reviewable checklist."""
import argparse
from collections import Counter
import json
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]


def read_json(path):
    return json.loads(path.read_text())


def require(condition, message):
    if not condition:
        raise ValueError(message)


def git(*args):
    return subprocess.check_output(["git", "-C", ROOT, *args], text=True)


def cli_entries(command, parent=""):
    path = (parent + " " + command["name"]).strip()
    yield "command", path, path, ()
    for kind, key in [("argument", "arguments"), ("option", "options")]:
        for item in command.get(key, []):
            aliases = tuple(item.get("aliases", [])) if kind == "option" else ()
            yield kind, path, item["name"], aliases
    for child in command.get("commands", []):
        yield from cli_entries(child, path)


def configuration_paths(schema, node, prefix=""):
    for key, child in node.get("properties", {}).items():
        path = prefix + key
        if child.get("$ref", "").endswith("/preventIncrementConfiguration"):
            child = schema["$defs"]["preventIncrementConfiguration"]
        additional = child.get("additionalProperties")
        if child.get("properties"):
            yield from configuration_paths(schema, child, path + ".")
        elif isinstance(additional, dict) and additional.get("properties"):
            yield from configuration_paths(schema, additional, path + ".*.")
        else:
            yield path


def validate(data, cli_schema):
    pin = data["baseline"]
    require(pin == read_json(ROOT / "baseline.json")["revision"], "Pinned baseline differs from baseline.json")
    source = set(git("ls-tree", "-r", "--name-only", "--full-tree", pin).splitlines())
    tests = {}
    for base in ("cmd", "internal"):
        for path in (ROOT / base).rglob("*_test.go"):
            for lineno, line in enumerate(path.read_text().splitlines(), 1):
                match = re.match(r"func (Test\w+)\(", line)
                if match:
                    tests.setdefault(match[1], []).append((path.relative_to(ROOT), lineno))
    rows = {}
    for row in data["behaviors"]:
        identifier = row["id"]
        require(identifier not in rows, f"Duplicate row {identifier}")
        rows[identifier] = row
        require(row["status"] in data["statuses"], f"Unknown status in {identifier}")
        require(row["source"] and set(row["source"]) <= source, f"Missing pinned source in {identifier}")
        if row["status"] in {"partial", "matched"}:
            require(row["tests"] and row["evidence"], f"Missing evidence in {identifier}")
        if row["status"] in {"partial", "open", "decision", "deferred"}:
            require(row["gap"], f"Missing acceptance work in {identifier}")
        for test in row["tests"]:
            name = test.split("/")[0]
            require(len(tests.get(name, [])) == 1, f"Unknown or ambiguous test {test} in {identifier}")
        for name in row["evidence"]:
            evidence = read_json(ROOT / name)
            require(pin in (evidence.get("baseline"), evidence.get("baseline_revision")), f"Wrong evidence baseline in {name}")
    surface = data["surface"]
    for group in (surface["cli"], surface["configuration"], [surface["output_variables"]]):
        for item in group:
            require(item["behaviors"] and set(item["behaviors"]) <= rows.keys(), f"Unmapped surface item {item}")
    mapped = [(e["kind"], e["command"], e["name"], tuple(e.get("aliases", []))) for e in surface["cli"]]
    require(len({entry[:3] for entry in mapped}) == len(mapped), "Duplicate CLI surface item")
    if cli_schema is not None:
        require(list(cli_entries(read_json(cli_schema)["command"])) == mapped, "CLI surface differs from the pinned export")
    schema = json.loads(git("show", f'{pin}:{surface["configuration_schema"]}'))
    paths = [item["path"] for item in surface["configuration"]]
    require(paths == list(configuration_paths(schema, schema)), "Configuration properties differ from the pinned schema")
    output = surface["output_variables"]
    model = git("show", f'{pin}:{output["source"]}')
    variables = sorted(re.findall(r"public\s+\w[\w?.<>]*\s+(\w+)\s*\{\s*get;", model))
    require(output["names"] == variables, "Output fields differ from the pinned JSON model")
    inventories = {
        "strategies": r"src/GitVersion.Core/VersionCalculation/VersionSearchStrategies/[^/]+VersionStrategy\.cs",
        "providers": r"src/GitVersion.BuildAgents/Agents/(?!AppVeyorJsonContext\.cs)[^/]+\.cs",
    }
    for key, pattern in inventories.items():
        mapped = surface[key]
        require(len({item["source"] for item in mapped}) == len(mapped), f"Duplicate {key}")
        require({item["source"] for item in mapped} == {path for path in source if re.fullmatch(pattern, path)}, f"Pinned {key} inventory differs")
        for item in mapped:
            require(item["behavior"] in rows and item["source"] in rows[item["behavior"]]["source"], f"Unmapped {key} entry {item}")
    return rows, tests


def render_checklist(data, tests):
    counts = Counter(row["status"] for row in data["behaviors"])
    surface = data["surface"]
    lines = [
        "# GitVersion behavior-by-behavior parity checklist",
        "",
        f'C# oracle {data["baseline"]} · Go evidence audited at {data["audit_of"][:9]}',
        "",
        "Scope covers the established C# src CLI with the v7 argument parser and managed Git backend. "
        "The priority platforms are macOS arm64 and Ubuntu 26.04 arm64. Windows is the final platform gate.",
        "",
        f'**{len(data["behaviors"])} acceptance rows** · '
        + " · ".join(f"{counts[name]} {name}" for name in data["statuses"]),
        "",
        f'The [machine-readable checklist](PARITY-CHECKLIST.json) maps '
        f'{sum(e["kind"] == "option" for e in surface["cli"])} CLI option entries, '
        f'{len(surface["configuration"])} configuration property paths, '
        f'{len(surface["output_variables"]["names"])} JSON variables, '
        f'{len(surface["strategies"])} strategies and {len(surface["providers"])} CI providers. '
        "A surface entry can map to several behaviors.",
        "",
        "## How to close a row",
        "",
        "Run the relevant live tests against the pinned oracle on both priority platforms and record the candidate revision. "
        "Compare version values and types, streams, exit status, generated files and repository state as applicable. "
        "Include failure, boundary and interaction cases appropriate to the row. The linked results come from the checkpoints named in the evidence. "
        "Ported unit tests support those comparisons.",
        "",
        "A decision row stays unresolved until the expected behavior is recorded and tested. "
        "The release gate requires closure of all in-scope rows and a complete run of the agreed matrix. "
        "See the [CLI option index](CLI-COMPATIBILITY.md) for lookup by command and flag.",
        "",
    ]
    lines += [f"- **{name.capitalize()}** — {meaning}" for name, meaning in data["statuses"].items()]
    area = None
    for row in data["behaviors"]:
        if row["area"] != area:
            area = row["area"]
            lines += ["", f"## {area}", "", "| Behavior | Status | Pinned source | Existing tests and evidence | Remaining acceptance work |",
                      "| --- | --- | --- | --- | --- |"]
        refs = []
        for test in row["tests"]:
            path, lineno = tests[test.split("/")[0]][0]
            refs.append(f"[{test}]({path.as_posix()}#L{lineno})")
        refs += [f"[Results {index}]({path})" for index, path in enumerate(row["evidence"], 1)]
        pinned = "<br>".join(Path(name).name for name in row["source"])
        lines.append(f'| <a id="{row["id"]}"></a>{row["behavior"]} | {row["status"].capitalize()} | '
                     f'{pinned} | {"<br>".join(refs) or "—"} | {row["gap"] or "—"} |')
    return "\n".join(lines) + "\n"


def render_cli(data, rows):
    lines = [
        "# Pinned CLI option index",
        "",
        f'Reference SHA {data["baseline"]}. The surface comes from the pinned --cli-schema export. '
        "POSIX aliases are in scope. Slash aliases are shown for reference.",
        "",
        "Use the [behavior checklist](PARITY-CHECKLIST.md) for acceptance criteria, exact test names and results.",
        "",
        "| Command | Entry | Aliases | Mapped behavior and status |",
        "| --- | --- | --- | --- |",
    ]
    for item in data["surface"]["cli"]:
        name = item["name"] if item["kind"] != "command" else "command"
        aliases = ", ".join(item.get("aliases", [])) or "—"
        links = "<br>".join(f'[{rows[id]["status"].capitalize()} · {id}](PARITY-CHECKLIST.md#{id})' for id in item["behaviors"])
        lines.append(f'| {item["command"]} | {name} | {aliases} | {links} |')
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli-schema", type=Path, help="Compare with a local --cli-schema export from the pinned executable")
    parser.add_argument("--check", action="store_true", help="Validate that the tracked Markdown is current")
    args = parser.parse_args()
    data = read_json(ROOT / "PARITY-CHECKLIST.json")
    rows, tests = validate(data, args.cli_schema)
    for name, rendered in [
        ("PARITY-CHECKLIST.md", render_checklist(data, tests)),
        ("CLI-COMPATIBILITY.md", render_cli(data, rows)),
    ]:
        path = ROOT / name
        if args.check:
            require(path.read_text() == rendered, f"Regenerate {name} with {Path(__file__).name}")
        else:
            path.write_text(rendered)
    print(f"Validated {len(rows)} behavior rows against {data['baseline'][:9]}.")


if __name__ == "__main__":
    main()
