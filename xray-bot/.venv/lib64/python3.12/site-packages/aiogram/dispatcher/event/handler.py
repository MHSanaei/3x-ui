import asyncio
import inspect
import sys
import warnings
from collections.abc import Callable
from dataclasses import dataclass, field
from functools import partial
from typing import Any

from magic_filter.magic import MagicFilter as OriginalMagicFilter

from aiogram.dispatcher.flags import extract_flags_from_object
from aiogram.filters.base import Filter
from aiogram.handlers import BaseHandler
from aiogram.utils.magic_filter import MagicFilter
from aiogram.utils.warnings import Recommendation

CallbackType = Callable[..., Any]

_ACCEPTED_PARAM_KINDS = {
    inspect.Parameter.POSITIONAL_ONLY,
    inspect.Parameter.POSITIONAL_OR_KEYWORD,
    inspect.Parameter.KEYWORD_ONLY,
}


@dataclass
class CallableObject:
    callback: CallbackType
    awaitable: bool = field(init=False)
    params: set[str] = field(init=False)
    varkw: bool = field(init=False)

    def __post_init__(self) -> None:
        callback = inspect.unwrap(self.callback)
        self.awaitable = inspect.isawaitable(callback) or inspect.iscoroutinefunction(callback)

        kwargs: dict[str, Any] = {}
        if sys.version_info >= (3, 14):
            import annotationlib

            kwargs["annotation_format"] = annotationlib.Format.FORWARDREF

        try:
            signature = inspect.signature(callback, **kwargs)
        except (ValueError, TypeError):  # pragma: no cover
            self.params = set()
            self.varkw = False
            return

        params: set[str] = set()
        varkw: bool = False

        for p in signature.parameters.values():
            if p.kind in _ACCEPTED_PARAM_KINDS:
                params.add(p.name)
            elif p.kind == inspect.Parameter.VAR_KEYWORD:
                varkw = True
        self.params = params
        self.varkw = varkw

    def _prepare_kwargs(self, kwargs: dict[str, Any]) -> dict[str, Any]:
        if self.varkw:
            return kwargs

        return {k: kwargs[k] for k in self.params if k in kwargs}

    async def call(self, *args: Any, **kwargs: Any) -> Any:
        wrapped = partial(self.callback, *args, **self._prepare_kwargs(kwargs))
        if self.awaitable:
            return await wrapped()
        return await asyncio.to_thread(wrapped)


@dataclass
class FilterObject(CallableObject):
    magic: OriginalMagicFilter | None = None

    def __post_init__(self) -> None:
        if isinstance(self.callback, OriginalMagicFilter):
            # MagicFilter instance is callable but generates
            # only "CallOperation" instead of applying the filter
            self.magic = self.callback
            self.callback = self.callback.resolve
            if not isinstance(self.magic, MagicFilter):
                # Issue: https://github.com/aiogram/aiogram/issues/990
                warnings.warn(
                    category=Recommendation,
                    message="You are using F provided by magic_filter package directly, "
                    "but it lacks `.as_()` extension."
                    "\n Please change the import statement: from `from magic_filter import F` "
                    "to `from aiogram import F` to silence this warning.",
                    stacklevel=6,
                )

        super().__post_init__()

        if isinstance(self.callback, Filter):
            self.awaitable = True


@dataclass
class HandlerObject(CallableObject):
    filters: list[FilterObject] | None = None
    flags: dict[str, Any] = field(default_factory=dict)

    def __post_init__(self) -> None:
        super().__post_init__()
        callback = inspect.unwrap(self.callback)
        if inspect.isclass(callback) and issubclass(callback, BaseHandler):
            self.awaitable = True
        self.flags.update(extract_flags_from_object(callback))

    async def check(self, *args: Any, **kwargs: Any) -> tuple[bool, dict[str, Any]]:
        if not self.filters:
            return True, kwargs
        for event_filter in self.filters:
            check = await event_filter.call(*args, **kwargs)
            if not check:
                return False, kwargs
            if isinstance(check, dict):
                kwargs.update(check)
        return True, kwargs
