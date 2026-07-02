import asyncio
import secrets
from abc import ABC, abstractmethod
from asyncio import Transport
from collections.abc import Awaitable, Callable
from typing import TYPE_CHECKING, Any, cast

from aiohttp import JsonPayload, MultipartWriter, Payload, web
from aiohttp.typedefs import Handler
from aiohttp.web_app import Application
from aiohttp.web_middlewares import middleware

from aiogram import Bot, Dispatcher, loggers
from aiogram.methods import TelegramMethod
from aiogram.methods.base import TelegramType
from aiogram.webhook.security import IPFilter

if TYPE_CHECKING:
    from aiogram.types import InputFile


def setup_application(app: Application, dispatcher: Dispatcher, /, **kwargs: Any) -> None:
    """
    This function helps to configure a startup-shutdown process

    :param app: aiohttp application
    :param dispatcher: aiogram dispatcher
    :param kwargs: additional data
    :return:
    """
    workflow_data = {
        "app": app,
        "dispatcher": dispatcher,
        **dispatcher.workflow_data,
        **kwargs,
    }

    async def on_startup(*a: Any, **kw: Any) -> None:  # pragma: no cover
        await dispatcher.emit_startup(**workflow_data)

    async def on_shutdown(*a: Any, **kw: Any) -> None:  # pragma: no cover
        await dispatcher.emit_shutdown(**workflow_data)

    app.on_startup.append(on_startup)
    app.on_shutdown.append(on_shutdown)


def check_ip(ip_filter: IPFilter, request: web.Request) -> tuple[str, bool]:
    # Try to resolve client IP over reverse proxy
    if forwarded_for := request.headers.get("X-Forwarded-For", ""):
        # Get the left-most ip when there is multiple ips
        # (request got through multiple proxy/load balancers)
        # https://github.com/aiogram/aiogram/issues/672
        forwarded_for, *_ = forwarded_for.split(",", maxsplit=1)
        return forwarded_for, forwarded_for in ip_filter

    # When reverse proxy is not configured IP address can be resolved from incoming connection
    if peer_name := cast(Transport, request.transport).get_extra_info("peername"):
        host, _ = peer_name
        return host, host in ip_filter

    # Potentially impossible case
    return "", False  # pragma: no cover


def ip_filter_middleware(
    ip_filter: IPFilter,
) -> Callable[[web.Request, Handler], Awaitable[Any]]:
    """

    :param ip_filter:
    :return:
    """

    @middleware
    async def _ip_filter_middleware(request: web.Request, handler: Handler) -> Any:
        ip_address, accept = check_ip(ip_filter=ip_filter, request=request)
        if not accept:
            loggers.webhook.warning("Blocking request from an unauthorized IP: %s", ip_address)
            raise web.HTTPUnauthorized()
        return await handler(request)

    return _ip_filter_middleware


class BaseRequestHandler(ABC):
    def __init__(
        self,
        dispatcher: Dispatcher,
        handle_in_background: bool = False,
        **data: Any,
    ) -> None:
        """
        Base handler that helps to handle incoming request from aiohttp
        and propagate it to the Dispatcher

        :param dispatcher: instance of :class:`aiogram.dispatcher.dispatcher.Dispatcher`
        :param handle_in_background: immediately responds to the Telegram instead of
            a waiting end of a handler process
        """
        self.dispatcher = dispatcher
        self.handle_in_background = handle_in_background
        self.data = data
        self._background_feed_update_tasks: set[asyncio.Task[Any]] = set()

    def register(self, app: Application, /, path: str, **kwargs: Any) -> None:
        """
        Register route and shutdown callback

        :param app: instance of aiohttp Application
        :param path: route path
        :param kwargs:
        """
        app.on_shutdown.append(self._handle_close)
        app.router.add_route("POST", path, self.handle, **kwargs)

    async def _handle_close(self, *a: Any, **kw: Any) -> None:
        await self.close()

    @abstractmethod
    async def close(self) -> None:
        pass

    @abstractmethod
    async def resolve_bot(self, request: web.Request) -> Bot:
        """
        This method should be implemented in subclasses of this class.

        Resolve Bot instance from request.

        :param request:
        :return: Bot instance
        """

    @abstractmethod
    def verify_secret(self, telegram_secret_token: str, bot: Bot) -> bool:
        pass

    async def _background_feed_update(self, bot: Bot, update: dict[str, Any]) -> None:
        result = await self.dispatcher.feed_raw_update(bot=bot, update=update, **self.data)
        if isinstance(result, TelegramMethod):
            await self.dispatcher.silent_call_request(bot=bot, result=result)

    async def _handle_request_background(self, bot: Bot, request: web.Request) -> web.Response:
        feed_update_task = asyncio.create_task(
            self._background_feed_update(
                bot=bot,
                update=await request.json(loads=bot.session.json_loads),
            ),
        )
        self._background_feed_update_tasks.add(feed_update_task)
        feed_update_task.add_done_callback(self._background_feed_update_tasks.discard)
        return web.json_response({}, dumps=bot.session.json_dumps)

    def _build_response_writer(
        self,
        bot: Bot,
        result: TelegramMethod[TelegramType] | None,
    ) -> Payload:
        if not result:
            # we need to return something "empty"
            # and "empty" form doesn't work
            # since it's sending only "end" boundary w/o "start"
            return JsonPayload({})

        writer = MultipartWriter(
            "form-data",
            boundary=f"webhookBoundary{secrets.token_urlsafe(16)}",
        )

        payload = writer.append(result.__api_method__)
        payload.set_content_disposition("form-data", name="method")

        files: dict[str, InputFile] = {}
        for key, value in result.model_dump(warnings=False).items():
            value = bot.session.prepare_value(value, bot=bot, files=files)
            if not value:
                continue
            payload = writer.append(value)
            payload.set_content_disposition("form-data", name=key)

        for key, value in files.items():
            payload = writer.append(value.read(bot))
            payload.set_content_disposition(
                "form-data",
                name=key,
                filename=value.filename or key,
            )

        return writer

    async def _handle_request(self, bot: Bot, request: web.Request) -> web.Response:
        result: TelegramMethod[Any] | None = await self.dispatcher.feed_webhook_update(
            bot,
            await request.json(loads=bot.session.json_loads),
            **self.data,
        )
        return web.Response(body=self._build_response_writer(bot=bot, result=result))

    async def handle(self, request: web.Request) -> web.Response:
        bot = await self.resolve_bot(request)
        if not self.verify_secret(request.headers.get("X-Telegram-Bot-Api-Secret-Token", ""), bot):
            return web.Response(body="Unauthorized", status=401)
        if self.handle_in_background:
            return await self._handle_request_background(bot=bot, request=request)
        return await self._handle_request(bot=bot, request=request)

    __call__ = handle


class SimpleRequestHandler(BaseRequestHandler):
    def __init__(
        self,
        dispatcher: Dispatcher,
        bot: Bot,
        handle_in_background: bool = True,
        secret_token: str | None = None,
        **data: Any,
    ) -> None:
        """
        Handler for single Bot instance

        :param dispatcher: instance of :class:`aiogram.dispatcher.dispatcher.Dispatcher`
        :param handle_in_background: immediately responds to the Telegram instead of
            a waiting end of handler process
        :param bot: instance of :class:`aiogram.client.bot.Bot`
        """
        super().__init__(dispatcher=dispatcher, handle_in_background=handle_in_background, **data)
        self.bot = bot
        self.secret_token = secret_token

    def verify_secret(self, telegram_secret_token: str, bot: Bot) -> bool:
        if self.secret_token:
            return secrets.compare_digest(telegram_secret_token, self.secret_token)
        return True

    async def close(self) -> None:
        """
        Close bot session
        """
        await self.bot.session.close()

    async def resolve_bot(self, request: web.Request) -> Bot:
        return self.bot


class TokenBasedRequestHandler(BaseRequestHandler):
    def __init__(
        self,
        dispatcher: Dispatcher,
        handle_in_background: bool = True,
        bot_settings: dict[str, Any] | None = None,
        **data: Any,
    ) -> None:
        """
        Handler that supports multiple bots the context will be resolved
        from path variable 'bot_token'

        .. note::

            This handler is not recommended in due to token is available in URL
            and can be logged by reverse proxy server or other middleware.

        :param dispatcher: instance of :class:`aiogram.dispatcher.dispatcher.Dispatcher`
        :param handle_in_background: immediately responds to the Telegram instead of
            a waiting end of handler process
        :param bot_settings: kwargs that will be passed to new Bot instance
        """
        super().__init__(dispatcher=dispatcher, handle_in_background=handle_in_background, **data)
        if bot_settings is None:
            bot_settings = {}
        self.bot_settings = bot_settings
        self.bots: dict[str, Bot] = {}

    def verify_secret(self, telegram_secret_token: str, bot: Bot) -> bool:
        return True

    async def close(self) -> None:
        for bot in self.bots.values():
            await bot.session.close()

    def register(self, app: Application, /, path: str, **kwargs: Any) -> None:
        """
        Validate path, register route and shutdown callback

        :param app: instance of aiohttp Application
        :param path: route path
        :param kwargs:
        """
        if "{bot_token}" not in path:
            msg = "Path should contains '{bot_token}' substring"
            raise ValueError(msg)
        super().register(app, path=path, **kwargs)

    async def resolve_bot(self, request: web.Request) -> Bot:
        """
        Get bot token from a path and create or get from cache Bot instance

        :param request:
        :return:
        """
        token = request.match_info["bot_token"]
        if token not in self.bots:
            self.bots[token] = Bot(token=token, **self.bot_settings)
        return self.bots[token]
