"""solvr 2.0.0 removed the legacy 1.x choices: a post has no type, every
contribution is a reply, and search no longer filters by the legacy type or
status. A 1.x call that still uses one fails with a TypeError naming it."""

import inspect
import re
from pathlib import Path

import pytest
import responses

import solvr
from solvr import Solvr

PACKAGE = Path(__file__).resolve().parent.parent
API_KEY = "solvr_sk_test_key"

# Every 1.x member 2.0.0 removed: a Solvr method, or a method's keyword argument.
REMOVED_METHODS = ["approach", "answer"]
REMOVED_ARGUMENTS = {
    "search": ["type", "status"],
    "post": ["type", "success_criteria"],
    "create_post": ["type", "success_criteria"],
    "get": ["include"],
    "get_post": ["include"],
}


def migration_notes() -> str:
    """The "Migrating from 1.x to <__version__>" section of the README."""
    heading = f"## Migrating from 1.x to {solvr.__version__}"
    readme = (PACKAGE / "README.md").read_text()
    if heading not in readme:
        pytest.fail(f'the README has no "{heading}" section')
    notes = readme.split(heading, 1)[1]
    return re.split(r"\n## ", notes, maxsplit=1)[0]


@responses.activate
@pytest.mark.parametrize("argument", [{"type": "problem"}, {"type": "all"}, {"status": "open"}])
def test_search_rejects_the_removed_filter_without_calling_the_api(argument):
    client = Solvr(api_key=API_KEY)
    (name,) = argument
    with pytest.raises(TypeError, match=f"'{name}'"):
        client.search("postgres", **argument)
    assert len(responses.calls) == 0


@responses.activate
def test_a_positional_1x_type_is_not_read_as_another_option():
    """1.x search(query, type, status, limit, page): the options after the
    query are keyword-only now, so search("q", "problem") cannot become a limit."""
    client = Solvr(api_key=API_KEY)
    with pytest.raises(TypeError):
        client.search("postgres", "problem")  # type: ignore[misc]
    assert len(responses.calls) == 0


def test_no_removed_method_or_argument_remains():
    for method in REMOVED_METHODS:
        assert not hasattr(Solvr, method), f"Solvr.{method} is named as removed but still exists"
    for method, arguments in REMOVED_ARGUMENTS.items():
        parameters = inspect.signature(getattr(Solvr, method)).parameters
        for argument in arguments:
            assert argument not in parameters, f"{method}({argument}=...) is named as removed but still accepted"
        assert not any(p.kind is p.VAR_KEYWORD for p in parameters.values()), f"{method} accepts any keyword"


def test_version_is_2_0_0_and_is_the_published_package_version():
    assert solvr.__version__ == "2.0.0"
    pyproject = (PACKAGE / "pyproject.toml").read_text()
    assert re.search(r'^version = "([^"]+)"', pyproject, re.M).group(1) == solvr.__version__


def test_the_readme_migration_notes_name_every_removed_member_and_what_replaces_it():
    notes = migration_notes()
    for method in REMOVED_METHODS:
        assert f"`{method}()`" in notes, f"the notes do not name {method}()"
    for argument in {a for arguments in REMOVED_ARGUMENTS.values() for a in arguments}:
        assert f"`{argument}`" in notes, f"the notes do not name {argument}"
    for use in ["`reply()`", "`replies()`", "ENDPOINT_RETIRED", 'details["replacement"]']:
        assert use in notes, f"the notes do not name {use}"
