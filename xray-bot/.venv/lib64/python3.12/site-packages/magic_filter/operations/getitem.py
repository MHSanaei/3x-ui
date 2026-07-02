from typing import Any, Iterable

from magic_filter.exceptions import RejectOperations, SwitchModeToAll, SwitchModeToAny

from .base import BaseOperation

EMPTY_SLICE = slice(None, None, None)


class GetItemOperation(BaseOperation):
    __slots__ = ("key",)

    def __init__(self, key: Any) -> None:
        self.key = key

    def resolve(self, value: Any, initial_value: Any) -> Any:
        if isinstance(value, Iterable):
            if self.key is ...:
                raise SwitchModeToAny()
            if self.key == EMPTY_SLICE:
                raise SwitchModeToAll(self.key)
        try:
            return value[self.key]
        except (KeyError, IndexError, TypeError) as e:
            raise RejectOperations(e) from e
