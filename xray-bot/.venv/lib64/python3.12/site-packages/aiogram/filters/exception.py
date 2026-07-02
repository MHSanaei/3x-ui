import re
from re import Pattern
from typing import Any, cast

from aiogram.filters.base import Filter
from aiogram.types import TelegramObject
from aiogram.types.error_event import ErrorEvent


class ExceptionTypeFilter(Filter):
    """
    Allows to match exception by type
    """

    __slots__ = ("exceptions",)

    def __init__(self, *exceptions: type[Exception]):
        """
        :param exceptions: Exception type(s)
        """
        if not exceptions:
            msg = "At least one exception type is required"
            raise ValueError(msg)
        self.exceptions = exceptions

    async def __call__(self, obj: TelegramObject) -> bool | dict[str, Any]:
        return isinstance(cast(ErrorEvent, obj).exception, self.exceptions)


class ExceptionMessageFilter(Filter):
    """
    Allow to match exception by message
    """

    __slots__ = ("pattern",)

    def __init__(self, pattern: str | Pattern[str]):
        """
        :param pattern: Regexp pattern
        """
        if isinstance(pattern, str):
            pattern = re.compile(pattern)
        self.pattern = pattern

    def __str__(self) -> str:
        return self._signature_to_string(
            pattern=self.pattern,
        )

    async def __call__(
        self,
        obj: TelegramObject,
    ) -> bool | dict[str, Any]:
        result = self.pattern.match(str(cast(ErrorEvent, obj).exception))
        if not result:
            return False
        return {"match_exception": result}
