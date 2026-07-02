from typing import Any, Callable

from ..exceptions import RejectOperations
from .base import BaseOperation


class CastOperation(BaseOperation):
    __slots__ = ("func",)

    def __init__(self, func: Callable[[Any], Any]) -> None:
        self.func = func

    def resolve(self, value: Any, initial_value: Any) -> Any:
        try:
            return self.func(value)
        except Exception as e:
            raise RejectOperations(e) from e
