from abc import ABC, abstractmethod
from collections.abc import Awaitable, Callable
from typing import TYPE_CHECKING, Any

if TYPE_CHECKING:
    from aiogram.filters.logic import _InvertFilter


class Filter(ABC):  # noqa: B024
    """
    If you want to register own filters like builtin filters you will need to write subclass
    of this class with overriding the :code:`__call__`
    method and adding filter attributes.
    """

    if TYPE_CHECKING:
        # This checking type-hint is needed because mypy checks validity of overrides and raises:
        # error: Signature of "__call__" incompatible with supertype "BaseFilter"  [override]
        # https://mypy.readthedocs.io/en/latest/error_code_list.html#check-validity-of-overrides-override
        __call__: Callable[..., Awaitable[bool | dict[str, Any]]]
    else:  # pragma: no cover

        @abstractmethod
        async def __call__(self, *args: Any, **kwargs: Any) -> bool | dict[str, Any]:
            """
            This method should be overridden.

            Accepts incoming event and should return boolean or dict.

            :return: :class:`bool` or :class:`dict[str, Any]`
            """

    def __invert__(self) -> "_InvertFilter":
        from aiogram.filters.logic import invert_f

        return invert_f(self)

    def update_handler_flags(self, flags: dict[str, Any]) -> None:  # noqa: B027
        """
        Also if you want to extend handler flags with using this filter
        you should implement this method

        :param flags: existing flags, can be updated directly
        """

    def _signature_to_string(self, *args: Any, **kwargs: Any) -> str:
        items = [repr(arg) for arg in args]
        items.extend([f"{k}={v!r}" for k, v in kwargs.items() if v is not None])

        return f"{type(self).__name__}({', '.join(items)})"

    def __await__(self):  # type: ignore # pragma: no cover
        # Is needed only for inspection and this method is never be called
        return self.__call__
