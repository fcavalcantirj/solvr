"""Builds the SDK's dataclasses from the API's JSON, by each field's declared type."""

from dataclasses import fields, is_dataclass
from functools import lru_cache
from typing import Any, Dict, Mapping, Type, TypeVar, Union, cast, get_args, get_origin, get_type_hints

T = TypeVar("T")


@lru_cache(maxsize=None)
def _hints(cls: Any) -> Dict[str, Any]:
    return get_type_hints(cls)


def decode(cls: Type[T], data: Mapping[str, Any]) -> T:
    """The cls dataclass of an API object. A field the API omits keeps its default; a field the
    dataclass does not declare is ignored."""
    dataclass = cast(Any, cls)
    hints = _hints(dataclass)
    values = {
        f.name: _decode_value(hints[f.name], data[f.name])
        for f in fields(dataclass)
        if f.init and f.name in data
    }
    return cls(**values)


def _decode_value(hint: Any, value: Any) -> Any:
    if value is None:
        return None
    origin = get_origin(hint)
    if origin is Union:
        args = [arg for arg in get_args(hint) if arg is not type(None)]
        return _decode_value(args[0], value) if len(args) == 1 else value
    if origin is list and isinstance(value, list):
        (item,) = get_args(hint)
        return [_decode_value(item, v) for v in value]
    if isinstance(hint, type) and is_dataclass(hint) and isinstance(value, Mapping):
        return decode(hint, value)
    return value
