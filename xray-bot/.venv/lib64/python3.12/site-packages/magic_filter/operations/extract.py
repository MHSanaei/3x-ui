from typing import TYPE_CHECKING, Any, Iterable

from magic_filter.operations import BaseOperation

if TYPE_CHECKING:
    from magic_filter.magic import MagicFilter


class ExtractOperation(BaseOperation):
    __slots__ = ("extractor",)

    def __init__(self, extractor: "MagicFilter") -> None:
        self.extractor = extractor

    def resolve(self, value: Any, initial_value: Any) -> Any:
        if not isinstance(value, Iterable):
            return None

        result = []
        for item in value:
            if self.extractor.resolve(item):
                result.append(item)
        return result
