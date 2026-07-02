from typing import Any, Callable

from ..helper import resolve_if_needed
from .base import BaseOperation


class CombinationOperation(BaseOperation):
    __slots__ = (
        "right",
        "combinator",
    )

    def __init__(self, right: Any, combinator: Callable[[Any, Any], bool]) -> None:
        self.right = right
        self.combinator = combinator

    def resolve(self, value: Any, initial_value: Any) -> Any:
        return self.combinator(value, resolve_if_needed(self.right, initial_value=initial_value))


class ImportantCombinationOperation(CombinationOperation):
    important = True


class RCombinationOperation(BaseOperation):
    __slots__ = (
        "left",
        "combinator",
    )

    def __init__(self, left: Any, combinator: Callable[[Any, Any], bool]) -> None:
        self.left = left
        self.combinator = combinator

    def resolve(self, value: Any, initial_value: Any) -> Any:
        return self.combinator(resolve_if_needed(self.left, initial_value), value)
