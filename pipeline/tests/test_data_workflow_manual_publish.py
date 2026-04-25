import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def _workflow(name: str) -> str:
    return (ROOT / ".github" / "workflows" / name).read_text()


def _workflows() -> list[Path]:
    return sorted((ROOT / ".github" / "workflows").glob("*.yml"))


def _input_block(text: str, input_name: str) -> str:
    match = re.search(rf"^      {input_name}:\n(?P<body>(?:        .*\n)+)", text, re.MULTILINE)
    assert match is not None, f"missing workflow_dispatch input {input_name}"
    return match.group("body")


def test_manual_data_runs_are_dry_by_default() -> None:
    for workflow in ("data-daily.yml", "data-publish.yml"):
        block = _input_block(_workflow(workflow), "dry_run")

        assert 'default: "true"' in block


def test_data_daily_forwards_replace_existing_release_to_publish() -> None:
    text = _workflow("data-daily.yml")

    block = _input_block(text, "replace_existing_release")
    assert 'default: "false"' in block
    assert "REPLACE_EXISTING_RELEASE:" in text
    assert (
        'gh workflow run data-publish.yml -f dry_run="$DRY_RUN" '
        '-f replace_existing_release="$REPLACE_EXISTING_RELEASE"'
    ) in text


def test_data_publish_requires_explicit_replace_for_existing_catalog_release() -> None:
    text = _workflow("data-publish.yml")

    block = _input_block(text, "replace_existing_release")
    assert 'default: "false"' in block
    assert 'if gh release view "data-${CATALOG_VERSION}" >/dev/null 2>&1; then' in text
    assert "REPLACE_EXISTING_RELEASE" in text
    assert "release already exists: data-${CATALOG_VERSION}" in text
    assert 'gh release delete "data-${CATALOG_VERSION}" --yes' in text
    assert (
        'gh api --method DELETE "/repos/${GITHUB_REPOSITORY}/git/refs/tags/data-${CATALOG_VERSION}"'
    ) in text


def test_workflows_do_not_pin_known_node20_actions() -> None:
    download_artifact = "actions/download-artifact"
    deprecated_patterns = (
        f"{download_artifact}@v5",
        f"{download_artifact}@v6",
    )
    offenders: list[str] = []

    for workflow in _workflows():
        text = workflow.read_text()
        for pattern in deprecated_patterns:
            if pattern in text:
                offenders.append(f"{workflow.name}: {pattern}")

    assert offenders == []
