import asyncio
import logging
import time
from asyncio import Event, Lock
from collections.abc import Awaitable, Callable
from contextlib import suppress
from types import TracebackType
from typing import Any

from aiogram import BaseMiddleware, Bot
from aiogram.dispatcher.flags import get_flag
from aiogram.types import Message, TelegramObject

logger = logging.getLogger(__name__)
DEFAULT_INTERVAL = 5.0
DEFAULT_INITIAL_SLEEP = 0.0


class ChatActionSender:
    """
    This utility helps to automatically send chat action until long actions is done
    to take acknowledge bot users the bot is doing something and not crashed.

    Provides simply to use context manager.

    Technically sender start background task with infinity loop which works
    until action will be finished and sends the
    `chat action <https://core.telegram.org/bots/api#sendchataction>`_
    every 5 seconds.
    """

    def __init__(
        self,
        *,
        bot: Bot,
        chat_id: str | int,
        message_thread_id: int | None = None,
        action: str = "typing",
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> None:
        """
        :param bot: instance of the bot
        :param chat_id: target chat id
        :param message_thread_id: unique identifier for the target message thread; supergroups only
        :param action: chat action type
        :param interval: interval between iterations
        :param initial_sleep: sleep before first sending of the action
        """
        self.chat_id = chat_id
        self.message_thread_id = message_thread_id
        self.action = action
        self.interval = interval
        self.initial_sleep = initial_sleep
        self.bot = bot

        self._lock = Lock()
        self._close_event = Event()
        self._closed_event = Event()
        self._task: asyncio.Task[Any] | None = None

    @property
    def running(self) -> bool:
        return bool(self._task)

    async def _wait(self, interval: float) -> None:
        with suppress(asyncio.TimeoutError):
            await asyncio.wait_for(self._close_event.wait(), interval)

    async def _worker(self) -> None:
        logger.debug(
            "Started chat action %r sender in chat_id=%s via bot id=%d",
            self.action,
            self.chat_id,
            self.bot.id,
        )
        try:
            counter = 0
            await self._wait(self.initial_sleep)
            while not self._close_event.is_set():
                start = time.monotonic()
                logger.debug(
                    "Sent chat action %r to chat_id=%s via bot %d (already sent actions %d)",
                    self.action,
                    self.chat_id,
                    self.bot.id,
                    counter,
                )
                await self.bot.send_chat_action(
                    chat_id=self.chat_id,
                    action=self.action,
                    message_thread_id=self.message_thread_id,
                )
                counter += 1

                interval = self.interval - (time.monotonic() - start)
                await self._wait(interval)
        finally:
            logger.debug(
                "Finished chat action %r sender in chat_id=%s via bot id=%d",
                self.action,
                self.chat_id,
                self.bot.id,
            )
            self._closed_event.set()

    async def _run(self) -> None:
        async with self._lock:
            self._close_event.clear()
            self._closed_event.clear()
            if self.running:
                msg = "Already running"
                raise RuntimeError(msg)
            self._task = asyncio.create_task(self._worker())

    async def _stop(self) -> None:
        async with self._lock:
            if not self.running:
                return
            if not self._close_event.is_set():  # pragma: no branches
                self._close_event.set()
                await self._closed_event.wait()
            self._task = None

    async def __aenter__(self) -> "ChatActionSender":
        await self._run()
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: TracebackType | None,
    ) -> Any:
        await self._stop()

    @classmethod
    def typing(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `typing` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="typing",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def upload_photo(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `upload_photo` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="upload_photo",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def record_video(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `record_video` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="record_video",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def upload_video(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `upload_video` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="upload_video",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def record_voice(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `record_voice` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="record_voice",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def upload_voice(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `upload_voice` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="upload_voice",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def upload_document(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `upload_document` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="upload_document",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def choose_sticker(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `choose_sticker` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="choose_sticker",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def find_location(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `find_location` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="find_location",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def record_video_note(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `record_video_note` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="record_video_note",
            interval=interval,
            initial_sleep=initial_sleep,
        )

    @classmethod
    def upload_video_note(
        cls,
        chat_id: int | str,
        bot: Bot,
        message_thread_id: int | None = None,
        interval: float = DEFAULT_INTERVAL,
        initial_sleep: float = DEFAULT_INITIAL_SLEEP,
    ) -> "ChatActionSender":
        """Create instance of the sender with `upload_video_note` action"""
        return cls(
            bot=bot,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            action="upload_video_note",
            interval=interval,
            initial_sleep=initial_sleep,
        )


class ChatActionMiddleware(BaseMiddleware):
    """
    Helps to automatically use chat action sender for all message handlers
    """

    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        if not isinstance(event, Message):
            return await handler(event, data)
        bot = data["bot"]

        chat_action = get_flag(data, "chat_action") or "typing"
        kwargs = {}
        if isinstance(chat_action, dict):
            if initial_sleep := chat_action.get("initial_sleep"):
                kwargs["initial_sleep"] = initial_sleep
            if interval := chat_action.get("interval"):
                kwargs["interval"] = interval
            if action := chat_action.get("action"):
                kwargs["action"] = action
        elif isinstance(chat_action, bool):
            kwargs["action"] = "typing"
        else:
            kwargs["action"] = chat_action
        kwargs["message_thread_id"] = (
            event.message_thread_id
            if isinstance(event, Message) and event.is_topic_message
            else None
        )
        async with ChatActionSender(bot=bot, chat_id=event.chat.id, **kwargs):
            return await handler(event, data)
