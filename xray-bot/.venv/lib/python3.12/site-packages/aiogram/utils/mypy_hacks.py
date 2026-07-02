from __future__ import annotations

import functools
from typing import TYPE_CHECKING, TypeVar

if TYPE_CHECKING:
    from collections.abc import Callable

T = TypeVar("T")


def lru_cache(maxsize: int = 128, typed: bool = False) -> Callable[[T], T]:
    """
    fix: lru_cache annotation doesn't work with a property
    this hack is only needed for the property, so type annotations are as they are
    """

    def wrapper(func: T) -> T:
        return functools.lru_cache(maxsize, typed)(func)  # type: ignore

    return wrapper
