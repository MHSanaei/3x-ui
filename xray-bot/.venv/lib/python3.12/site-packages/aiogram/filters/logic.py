from abc import ABC
from typing import TYPE_CHECKING, Any

from aiogram.filters import Filter

if TYPE_CHECKING:
    from aiogram.dispatcher.event.handler import CallbackType, FilterObject


class _LogicFilter(Filter, ABC):
    pass


class _InvertFilter(_LogicFilter):
    __slots__ = ("target",)

    def __init__(self, target: "FilterObject") -> None:
        self.target = target

    async def __call__(self, *args: Any, **kwargs: Any) -> bool | dict[str, Any]:
        return not bool(await self.target.call(*args, **kwargs))


class _AndFilter(_LogicFilter):
    __slots__ = ("targets",)

    def __init__(self, *targets: "FilterObject") -> None:
        self.targets = targets

    async def __call__(self, *args: Any, **kwargs: Any) -> bool | dict[str, Any]:
        final_result = {}

        for target in self.targets:
            result = await target.call(*args, **kwargs)
            if not result:
                return False
            if isinstance(result, dict):
                final_result.update(result)

        if final_result:
            return final_result
        return True


class _OrFilter(_LogicFilter):
    __slots__ = ("targets",)

    def __init__(self, *targets: "FilterObject") -> None:
        self.targets = targets

    async def __call__(self, *args: Any, **kwargs: Any) -> bool | dict[str, Any]:
        for target in self.targets:
            result = await target.call(*args, **kwargs)
            if not result:
                continue
            if isinstance(result, dict):
                return result
            return bool(result)
        return False


def and_f(*targets: "CallbackType") -> _AndFilter:
    from aiogram.dispatcher.event.handler import FilterObject

    return _AndFilter(*(FilterObject(target) for target in targets))


def or_f(*targets: "CallbackType") -> _OrFilter:
    from aiogram.dispatcher.event.handler import FilterObject

    return _OrFilter(*(FilterObject(target) for target in targets))


def invert_f(target: "CallbackType") -> _InvertFilter:
    from aiogram.dispatcher.event.handler import FilterObject

    return _InvertFilter(FilterObject(target))
