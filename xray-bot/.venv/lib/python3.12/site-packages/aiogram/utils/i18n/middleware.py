from __future__ import annotations

from abc import ABC, abstractmethod
from typing import TYPE_CHECKING, Any

try:
    from babel import Locale, UnknownLocaleError
except ImportError:  # pragma: no cover
    Locale = None  # type: ignore

    class UnknownLocaleError(Exception):  # type: ignore
        pass


from aiogram import BaseMiddleware, Router

if TYPE_CHECKING:
    from collections.abc import Awaitable, Callable

    from aiogram.fsm.context import FSMContext
    from aiogram.types import TelegramObject, User
    from aiogram.utils.i18n.core import I18n


class I18nMiddleware(BaseMiddleware, ABC):
    """
    Abstract I18n middleware.
    """

    def __init__(
        self,
        i18n: I18n,
        i18n_key: str | None = "i18n",
        middleware_key: str = "i18n_middleware",
    ) -> None:
        """
        Create an instance of middleware

        :param i18n: instance of I18n
        :param i18n_key: context key for I18n instance
        :param middleware_key: context key for this middleware
        """
        self.i18n = i18n
        self.i18n_key = i18n_key
        self.middleware_key = middleware_key

    def setup(
        self: BaseMiddleware,
        router: Router,
        exclude: set[str] | None = None,
    ) -> BaseMiddleware:
        """
        Register middleware for all events in the Router

        :param router:
        :param exclude:
        :return:
        """
        if exclude is None:
            exclude = set()
        exclude_events = {"update", *exclude}
        for event_name, observer in router.observers.items():
            if event_name in exclude_events:
                continue
            observer.outer_middleware(self)
        return self

    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        current_locale = await self.get_locale(event=event, data=data) or self.i18n.default_locale

        if self.i18n_key:
            data[self.i18n_key] = self.i18n
        if self.middleware_key:
            data[self.middleware_key] = self

        with self.i18n.context(), self.i18n.use_locale(current_locale):
            return await handler(event, data)

    @abstractmethod
    async def get_locale(self, event: TelegramObject, data: dict[str, Any]) -> str:
        """
        Detect current user locale based on event and context.

        **This method must be defined in child classes**

        :param event:
        :param data:
        :return:
        """


class SimpleI18nMiddleware(I18nMiddleware):
    """
    Simple I18n middleware.

    Chooses language code from the User object received in event
    """

    def __init__(
        self,
        i18n: I18n,
        i18n_key: str | None = "i18n",
        middleware_key: str = "i18n_middleware",
    ) -> None:
        super().__init__(i18n=i18n, i18n_key=i18n_key, middleware_key=middleware_key)

        if Locale is None:  # pragma: no cover
            msg = (
                f"{type(self).__name__} can be used only when Babel installed\n"
                "Just install Babel (`pip install Babel`) "
                "or aiogram with i18n support (`pip install aiogram[i18n]`)"
            )
            raise RuntimeError(msg)

    async def get_locale(self, event: TelegramObject, data: dict[str, Any]) -> str:
        if Locale is None:  # pragma: no cover
            msg = (
                f"{type(self).__name__} can be used only when Babel installed\n"
                "Just install Babel (`pip install Babel`) "
                "or aiogram with i18n support (`pip install aiogram[i18n]`)"
            )
            raise RuntimeError(msg)

        event_from_user: User | None = data.get("event_from_user")
        if event_from_user is None or event_from_user.language_code is None:
            return self.i18n.default_locale
        try:
            locale = Locale.parse(event_from_user.language_code, sep="-")
        except UnknownLocaleError:
            return self.i18n.default_locale

        if locale.language not in self.i18n.available_locales:
            return self.i18n.default_locale
        return locale.language


class ConstI18nMiddleware(I18nMiddleware):
    """
    Const middleware chooses statically defined locale
    """

    def __init__(
        self,
        locale: str,
        i18n: I18n,
        i18n_key: str | None = "i18n",
        middleware_key: str = "i18n_middleware",
    ) -> None:
        super().__init__(i18n=i18n, i18n_key=i18n_key, middleware_key=middleware_key)
        self.locale = locale

    async def get_locale(self, event: TelegramObject, data: dict[str, Any]) -> str:
        return self.locale


class FSMI18nMiddleware(SimpleI18nMiddleware):
    """
    This middleware stores locale in the FSM storage
    """

    def __init__(
        self,
        i18n: I18n,
        key: str = "locale",
        i18n_key: str | None = "i18n",
        middleware_key: str = "i18n_middleware",
    ) -> None:
        super().__init__(i18n=i18n, i18n_key=i18n_key, middleware_key=middleware_key)
        self.key = key

    async def get_locale(self, event: TelegramObject, data: dict[str, Any]) -> str:
        fsm_context: FSMContext | None = data.get("state")
        locale = None
        if fsm_context:
            fsm_data = await fsm_context.get_data()
            locale = fsm_data.get(self.key, None)
        if not locale:
            locale = await super().get_locale(event=event, data=data)
            if fsm_context:
                await fsm_context.update_data(data={self.key: locale})
        return locale

    async def set_locale(self, state: FSMContext, locale: str) -> None:
        """
        Write new locale to the storage

        :param state: instance of FSMContext
        :param locale: new locale
        """
        await state.update_data(data={self.key: locale})
        self.i18n.current_locale = locale
