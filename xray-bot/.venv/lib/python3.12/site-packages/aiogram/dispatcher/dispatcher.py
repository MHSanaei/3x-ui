from __future__ import annotations

import asyncio
import contextvars
import signal
import sys
import warnings
from asyncio import CancelledError, Event, Future, Lock
from collections.abc import AsyncGenerator, Awaitable
from contextlib import suppress
from typing import TYPE_CHECKING, Any

from aiogram import loggers
from aiogram.exceptions import TelegramAPIError
from aiogram.fsm.middleware import FSMContextMiddleware
from aiogram.fsm.storage.base import BaseEventIsolation, BaseStorage
from aiogram.fsm.storage.memory import DisabledEventIsolation, MemoryStorage
from aiogram.fsm.strategy import FSMStrategy
from aiogram.methods import GetUpdates, TelegramMethod
from aiogram.types import Update, User
from aiogram.types.base import UNSET, UNSET_TYPE
from aiogram.types.update import UpdateTypeLookupError
from aiogram.utils.backoff import Backoff, BackoffConfig

from .event.bases import UNHANDLED, SkipHandler
from .event.telegram import TelegramEventObserver
from .middlewares.error import ErrorsMiddleware
from .middlewares.user_context import UserContextMiddleware
from .router import Router

if TYPE_CHECKING:
    from aiogram.client.bot import Bot
    from aiogram.methods.base import TelegramType

DEFAULT_BACKOFF_CONFIG = BackoffConfig(min_delay=1.0, max_delay=5.0, factor=1.3, jitter=0.1)


class Dispatcher(Router):
    """
    Root router
    """

    def __init__(
        self,
        *,  # * - Preventing to pass instance of Bot to the FSM storage
        storage: BaseStorage | None = None,
        fsm_strategy: FSMStrategy = FSMStrategy.USER_IN_CHAT,
        events_isolation: BaseEventIsolation | None = None,
        disable_fsm: bool = False,
        name: str | None = None,
        **kwargs: Any,
    ) -> None:
        """
        Root router

        :param storage: Storage for FSM
        :param fsm_strategy: FSM strategy
        :param events_isolation: Events isolation
        :param disable_fsm: Disable FSM, note that if you disable FSM
            then you should not use storage and events isolation
        :param kwargs: Other arguments, will be passed as keyword arguments to handlers
        """
        super().__init__(name=name)

        if storage and not isinstance(storage, BaseStorage):
            msg = f"FSM storage should be instance of 'BaseStorage' not {type(storage).__name__}"
            raise TypeError(msg)

        # Telegram API provides originally only one event type - Update
        # For making easily interactions with events here is registered handler which helps
        # to separate Update to different event types like Message, CallbackQuery etc.
        self.update = self.observers["update"] = TelegramEventObserver(
            router=self,
            event_name="update",
        )
        self.update.register(self._listen_update)

        # Error handlers should work is out of all other functions
        # and should be registered before all others middlewares
        self.update.outer_middleware(ErrorsMiddleware(self))

        # User context middleware makes small optimization for all other builtin
        # middlewares via caching the user and chat instances in the event context
        self.update.outer_middleware(UserContextMiddleware())

        # FSM middleware should always be registered after User context middleware
        # because here is used context from previous step
        self.fsm = FSMContextMiddleware(
            storage=storage or MemoryStorage(),
            strategy=fsm_strategy,
            events_isolation=events_isolation or DisabledEventIsolation(),
        )
        if not disable_fsm:
            # Note that when FSM middleware is disabled, the event isolation is also disabled
            # Because the isolation mechanism is a part of the FSM
            self.update.outer_middleware(self.fsm)
        self.shutdown.register(self.fsm.close)

        self.workflow_data: dict[str, Any] = kwargs
        self._running_lock = Lock()
        self._stop_signal: Event | None = None
        self._stopped_signal: Event | None = None
        self._handle_update_tasks: set[asyncio.Task[Any]] = set()

    def __getitem__(self, item: str) -> Any:
        return self.workflow_data[item]

    def __setitem__(self, key: str, value: Any) -> None:
        self.workflow_data[key] = value

    def __delitem__(self, key: str) -> None:
        del self.workflow_data[key]

    def get(self, key: str, /, default: Any | None = None) -> Any | None:
        return self.workflow_data.get(key, default)

    @property
    def storage(self) -> BaseStorage:
        return self.fsm.storage

    @property
    def parent_router(self) -> Router | None:
        """
        Dispatcher has no parent router and can't be included to any other routers or dispatchers

        :return:
        """
        return None

    @parent_router.setter
    def parent_router(self, value: Router) -> None:
        """
        Dispatcher is root Router then configuring parent router is not allowed

        :param value:
        :return:
        """
        msg = "Dispatcher can not be attached to another Router."
        raise RuntimeError(msg)

    async def feed_update(self, bot: Bot, update: Update, **kwargs: Any) -> Any:
        """
        Main entry point for incoming updates
        Response of this method can be used as Webhook response

        :param bot:
        :param update:
        """
        loop = asyncio.get_running_loop()
        handled = False
        start_time = loop.time()

        if update.bot != bot:
            # Re-mounting update to the current bot instance for making possible to
            # use it in shortcuts.
            # Here is update is re-created because we need to propagate context to
            # all nested objects and attributes of the Update, but it
            # is impossible without roundtrip to JSON :(
            # The preferred way is that pass already mounted Bot instance to this update
            # before call feed_update method
            update = Update.model_validate(update.model_dump(), context={"bot": bot})

        try:
            response = await self.update.wrap_outer_middleware(
                self.update.trigger,
                update,
                {
                    **self.workflow_data,
                    **kwargs,
                    "bot": bot,
                },
            )
            handled = response is not UNHANDLED
            return response
        finally:
            finish_time = loop.time()
            duration = (finish_time - start_time) * 1000
            loggers.event.info(
                "Update id=%s is %s. Duration %d ms by bot id=%d",
                update.update_id,
                "handled" if handled else "not handled",
                duration,
                bot.id,
            )

    async def feed_raw_update(self, bot: Bot, update: dict[str, Any], **kwargs: Any) -> Any:
        """
        Main entry point for incoming updates with automatic Dict->Update serializer

        :param bot:
        :param update:
        :param kwargs:
        """
        parsed_update = Update.model_validate(update, context={"bot": bot})
        return await self._feed_webhook_update(bot=bot, update=parsed_update, **kwargs)

    @classmethod
    async def _listen_updates(
        cls,
        bot: Bot,
        polling_timeout: int = 30,
        backoff_config: BackoffConfig = DEFAULT_BACKOFF_CONFIG,
        allowed_updates: list[str] | None = None,
    ) -> AsyncGenerator[Update, None]:
        """
        Endless updates reader with correctly handling any server-side or connection errors.

        So you may not worry that the polling will stop working.
        """
        backoff = Backoff(config=backoff_config)
        get_updates = GetUpdates(timeout=polling_timeout, allowed_updates=allowed_updates)
        kwargs = {}
        if bot.session.timeout:
            # Request timeout can be lower than session timeout and that's OK.
            # To prevent false-positive TimeoutError we should wait longer than polling timeout
            kwargs["request_timeout"] = int(bot.session.timeout + polling_timeout)
        failed = False
        while True:
            try:
                updates = await bot(get_updates, **kwargs)
            except Exception as e:  # noqa: BLE001
                failed = True
                # In cases when Telegram Bot API was inaccessible don't need to stop polling
                # process because some developers can't make auto-restarting of the script
                loggers.dispatcher.error("Failed to fetch updates - %s: %s", type(e).__name__, e)
                # And also backoff timeout is best practice to retry any network activity
                loggers.dispatcher.warning(
                    "Sleep for %f seconds and try again... (tryings = %d, bot id = %d)",
                    backoff.next_delay,
                    backoff.counter,
                    bot.id,
                )
                await backoff.asleep()
                continue

            # In case when network connection was fixed let's reset the backoff
            # to initial value and then process updates
            if failed:
                loggers.dispatcher.info(
                    "Connection established (tryings = %d, bot id = %d)",
                    backoff.counter,
                    bot.id,
                )
                backoff.reset()
                failed = False

            for update in updates:
                yield update
                # The getUpdates method returns the earliest 100 unconfirmed updates.
                # To confirm an update, use the offset parameter when calling getUpdates
                # All updates with update_id less than or equal to offset will be marked
                # as confirmed on the server and will no longer be returned.
                get_updates.offset = update.update_id + 1

    async def _listen_update(self, update: Update, **kwargs: Any) -> Any:
        """
        Main updates listener

        Workflow:
        - Detect content type and propagate to observers in current router
        - If no one filter is pass - propagate update to child routers as Update

        :param update:
        :param kwargs:
        :return:
        """
        try:
            update_type = update.event_type
            event = update.event
        except UpdateTypeLookupError as e:
            warnings.warn(
                "Detected unknown update type.\n"
                "Seems like Telegram Bot API was updated and you have "
                "installed not latest version of aiogram framework"
                f"\nUpdate: {update.model_dump_json(exclude_unset=True)}",
                RuntimeWarning,
                stacklevel=2,
            )
            raise SkipHandler() from e

        kwargs.update(event_update=update)

        return await self.propagate_event(update_type=update_type, event=event, **kwargs)

    @classmethod
    async def silent_call_request(cls, bot: Bot, result: TelegramMethod[Any]) -> None:
        """
        Simulate answer into WebHook

        :param bot:
        :param result:
        :return:
        """
        try:
            await bot(result)
        except TelegramAPIError as e:
            # In due to WebHook mechanism doesn't allow getting response for
            # requests called in answer to WebHook request.
            # Need to skip unsuccessful responses.
            # For debugging here is added logging.
            loggers.event.error("Failed to make answer: %s: %s", e.__class__.__name__, e)

    async def _process_update(
        self,
        bot: Bot,
        update: Update,
        call_answer: bool = True,
        **kwargs: Any,
    ) -> bool:
        """
        Propagate update to event listeners

        :param bot: instance of Bot
        :param update: instance of Update
        :param call_answer: need to execute response as Telegram method (like answer into webhook)
        :param kwargs: contextual data for middlewares, filters and handlers
        :return: status
        """
        try:
            response = await self.feed_update(bot, update, **kwargs)
            if call_answer and isinstance(response, TelegramMethod):
                await self.silent_call_request(bot=bot, result=response)

        except Exception as e:  # noqa: BLE001
            loggers.event.exception(
                "Cause exception while process update id=%d by bot id=%d\n%s: %s",
                update.update_id,
                bot.id,
                e.__class__.__name__,
                e,
            )
            return True  # because update was processed but unsuccessful

        else:
            return response is not UNHANDLED

    async def _process_with_semaphore(
        self,
        handle_update: Awaitable[bool],
        semaphore: asyncio.Semaphore,
    ) -> bool:
        """
        Process update with semaphore to limit concurrent tasks

        :param handle_update: Coroutine that processes the update
        :param semaphore: Semaphore to limit concurrent tasks
        :return: bool indicating the result of the update processing
        """
        try:
            return await handle_update
        finally:
            semaphore.release()

    async def _polling(
        self,
        bot: Bot,
        polling_timeout: int = 30,
        handle_as_tasks: bool = True,
        backoff_config: BackoffConfig = DEFAULT_BACKOFF_CONFIG,
        allowed_updates: list[str] | None = None,
        tasks_concurrency_limit: int | None = None,
        **kwargs: Any,
    ) -> None:
        """
        Internal polling process

        :param bot:
        :param polling_timeout: Long-polling wait time
        :param handle_as_tasks: Run task for each event and no wait result
        :param backoff_config: backoff-retry config
        :param allowed_updates: List of the update types you want your bot to receive
        :param tasks_concurrency_limit: Maximum number of concurrent updates to process
            (None = no limit), used only if handle_as_tasks is True
        :param kwargs:
        :return:
        """
        user: User = await bot.me()
        loggers.dispatcher.info(
            "Run polling for bot @%s id=%d - %r",
            user.username,
            bot.id,
            user.full_name,
        )

        # Create semaphore if tasks_concurrency_limit is specified
        semaphore = None
        if tasks_concurrency_limit is not None and handle_as_tasks:
            semaphore = asyncio.Semaphore(tasks_concurrency_limit)

        try:
            async for update in self._listen_updates(
                bot,
                polling_timeout=polling_timeout,
                backoff_config=backoff_config,
                allowed_updates=allowed_updates,
            ):
                handle_update = self._process_update(bot=bot, update=update, **kwargs)
                if handle_as_tasks:
                    if semaphore:
                        # Use semaphore to limit concurrent tasks
                        await semaphore.acquire()
                        handle_update_task = asyncio.create_task(
                            self._process_with_semaphore(handle_update, semaphore),
                        )
                    else:
                        handle_update_task = asyncio.create_task(handle_update)

                    self._handle_update_tasks.add(handle_update_task)
                    handle_update_task.add_done_callback(self._handle_update_tasks.discard)
                else:
                    await handle_update
        finally:
            loggers.dispatcher.info(
                "Polling stopped for bot @%s id=%d - %r",
                user.username,
                bot.id,
                user.full_name,
            )

    async def _feed_webhook_update(self, bot: Bot, update: Update, **kwargs: Any) -> Any:
        """
        The same with `Dispatcher.process_update()` but returns real response instead of bool
        """
        try:
            return await self.feed_update(bot, update, **kwargs)
        except Exception as e:
            loggers.event.exception(
                "Cause exception while process update id=%d by bot id=%d\n%s: %s",
                update.update_id,
                bot.id,
                e.__class__.__name__,
                e,
            )
            raise

    async def feed_webhook_update(
        self,
        bot: Bot,
        update: Update | dict[str, Any],
        _timeout: float = 55,
        **kwargs: Any,
    ) -> TelegramMethod[TelegramType] | None:
        if not isinstance(update, Update):  # Allow to use raw updates
            update = Update.model_validate(update, context={"bot": bot})

        ctx = contextvars.copy_context()
        loop = asyncio.get_running_loop()
        waiter = loop.create_future()

        def release_waiter(*_: Any) -> None:
            if not waiter.done():
                waiter.set_result(None)

        timeout_handle = loop.call_later(_timeout, release_waiter)

        process_updates: Future[Any] = asyncio.ensure_future(
            self._feed_webhook_update(bot=bot, update=update, **kwargs),
        )
        process_updates.add_done_callback(release_waiter, context=ctx)

        def process_response(task: Future[Any]) -> None:
            warnings.warn(
                "Detected slow response into webhook.\n"
                "Telegram is waiting for response only first 60 seconds and then re-send update.\n"
                "For preventing this situation response into webhook returned immediately "
                "and handler is moved to background and still processing update.",
                RuntimeWarning,
                stacklevel=2,
            )
            result = task.result()
            if isinstance(result, TelegramMethod):
                asyncio.ensure_future(self.silent_call_request(bot=bot, result=result))

        try:
            try:
                await waiter
            except CancelledError:  # pragma: no cover
                process_updates.remove_done_callback(release_waiter)
                process_updates.cancel()
                raise

            if process_updates.done():
                # TODO: handle exceptions
                response: Any = process_updates.result()
                if isinstance(response, TelegramMethod):
                    return response

            else:
                process_updates.remove_done_callback(release_waiter)
                process_updates.add_done_callback(process_response, context=ctx)

        finally:
            timeout_handle.cancel()

        return None

    async def stop_polling(self) -> None:
        """
        Execute this method if you want to stop polling programmatically

        :return:
        """
        if not self._running_lock.locked():
            msg = "Polling is not started"
            raise RuntimeError(msg)
        if not self._stop_signal or not self._stopped_signal:
            return
        self._stop_signal.set()
        await self._stopped_signal.wait()

    def _signal_stop_polling(self, sig: signal.Signals) -> None:
        if not self._running_lock.locked():
            return

        loggers.dispatcher.warning("Received %s signal", sig.name)
        if not self._stop_signal:
            return
        self._stop_signal.set()

    async def start_polling(
        self,
        *bots: Bot,
        polling_timeout: int = 10,
        handle_as_tasks: bool = True,
        backoff_config: BackoffConfig = DEFAULT_BACKOFF_CONFIG,
        allowed_updates: list[str] | UNSET_TYPE | None = UNSET,
        handle_signals: bool = True,
        close_bot_session: bool = True,
        tasks_concurrency_limit: int | None = None,
        **kwargs: Any,
    ) -> None:
        """
        Polling runner

        :param bots: Bot instances (one or more)
        :param polling_timeout: Long-polling wait time
        :param handle_as_tasks: Run task for each event and no wait result
        :param backoff_config: backoff-retry config
        :param allowed_updates: List of the update types you want your bot to receive
               By default, all used update types are enabled (resolved from handlers)
        :param handle_signals: handle signals (SIGINT/SIGTERM)
        :param close_bot_session: close bot sessions on shutdown
        :param tasks_concurrency_limit: Maximum number of concurrent updates to process
            (None = no limit), used only if handle_as_tasks is True
        :param kwargs: contextual data
        :return:
        """
        if not bots:
            msg = "At least one bot instance is required to start polling"
            raise ValueError(msg)
        if "bot" in kwargs:
            msg = (
                "Keyword argument 'bot' is not acceptable, "
                "the bot instance should be passed as positional argument"
            )
            raise ValueError(msg)

        async with self._running_lock:  # Prevent to run this method twice at a once
            if self._stop_signal is None:
                self._stop_signal = Event()
            if self._stopped_signal is None:
                self._stopped_signal = Event()

            if allowed_updates is UNSET:
                allowed_updates = self.resolve_used_update_types()

            self._stop_signal.clear()
            self._stopped_signal.clear()

            if handle_signals:
                loop = asyncio.get_running_loop()
                with suppress(NotImplementedError):  # pragma: no cover
                    # Signals handling is not supported on Windows
                    # It also can't be covered on Windows
                    loop.add_signal_handler(
                        signal.SIGTERM,
                        self._signal_stop_polling,
                        signal.SIGTERM,
                    )
                    loop.add_signal_handler(
                        signal.SIGINT,
                        self._signal_stop_polling,
                        signal.SIGINT,
                    )

            workflow_data = {
                "dispatcher": self,
                "bots": bots,
                **self.workflow_data,
                **kwargs,
            }
            if "bot" in workflow_data:
                workflow_data.pop("bot")

            await self.emit_startup(bot=bots[-1], **workflow_data)
            loggers.dispatcher.info("Start polling")
            try:
                tasks: list[asyncio.Task[Any]] = [
                    asyncio.create_task(
                        self._polling(
                            bot=bot,
                            handle_as_tasks=handle_as_tasks,
                            polling_timeout=polling_timeout,
                            backoff_config=backoff_config,
                            allowed_updates=allowed_updates,
                            tasks_concurrency_limit=tasks_concurrency_limit,
                            **workflow_data,
                        ),
                    )
                    for bot in bots
                ]
                tasks.append(asyncio.create_task(self._stop_signal.wait()))
                done, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)

                for task in pending:
                    # (mostly) Graceful shutdown unfinished tasks
                    task.cancel()
                    with suppress(CancelledError):
                        await task
                # Wait finished tasks to propagate unhandled exceptions
                await asyncio.gather(*done)

            finally:
                loggers.dispatcher.info("Polling stopped")
                try:
                    await self.emit_shutdown(bot=bots[-1], **workflow_data)
                finally:
                    if close_bot_session:
                        await asyncio.gather(*(bot.session.close() for bot in bots))
                self._stopped_signal.set()

    def run_polling(
        self,
        *bots: Bot,
        polling_timeout: int = 10,
        handle_as_tasks: bool = True,
        backoff_config: BackoffConfig = DEFAULT_BACKOFF_CONFIG,
        allowed_updates: list[str] | UNSET_TYPE | None = UNSET,
        handle_signals: bool = True,
        close_bot_session: bool = True,
        tasks_concurrency_limit: int | None = None,
        **kwargs: Any,
    ) -> None:
        """
        Run many bots with polling

        :param bots: Bot instances (one or more)
        :param polling_timeout: Long-polling wait time
        :param handle_as_tasks: Run task for each event and no wait result
        :param backoff_config: backoff-retry config
        :param allowed_updates: List of the update types you want your bot to receive
        :param handle_signals: handle signals (SIGINT/SIGTERM)
        :param close_bot_session: close bot sessions on shutdown
        :param tasks_concurrency_limit: Maximum number of concurrent updates to process
            (None = no limit), used only if handle_as_tasks is True
        :param kwargs: contextual data
        :return:
        """
        with suppress(KeyboardInterrupt):
            coro = self.start_polling(
                *bots,
                **kwargs,
                polling_timeout=polling_timeout,
                handle_as_tasks=handle_as_tasks,
                backoff_config=backoff_config,
                allowed_updates=allowed_updates,
                handle_signals=handle_signals,
                close_bot_session=close_bot_session,
                tasks_concurrency_limit=tasks_concurrency_limit,
            )

            try:
                import uvloop

            except ImportError:
                return asyncio.run(coro)

            else:
                if sys.version_info >= (3, 11):
                    with asyncio.Runner(loop_factory=uvloop.new_event_loop) as runner:
                        return runner.run(coro)
                else:  # pragma: no cover
                    uvloop.install()
                    return asyncio.run(coro)
