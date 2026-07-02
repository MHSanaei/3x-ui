from collections.abc import Iterable
from typing import Any

from magic_filter import MagicFilter as _MagicFilter
from magic_filter import MagicT as _MagicT
from magic_filter.operations import BaseOperation


class AsFilterResultOperation(BaseOperation):
    __slots__ = ("name",)

    def __init__(self, name: str) -> None:
        self.name = name

    def resolve(self, value: Any, initial_value: Any) -> Any:
        if value is None or (isinstance(value, Iterable) and not value):
            return None
        return {self.name: value}


class MagicFilter(_MagicFilter):
    def as_(self: _MagicT, name: str) -> _MagicT:
        return self._extend(AsFilterResultOperation(name=name))
