from typing import Any

from aiogram.methods import TelegramMethod
from aiogram.methods.base import TelegramType
from aiogram.utils.link import docs_url


class AiogramError(Exception):
    """
    Base exception for all aiogram errors.
    """


class DetailedAiogramError(AiogramError):
    """
    Base exception for all aiogram errors with detailed message.
    """

    url: str | None = None

    def __init__(self, message: str) -> None:
        self.message = message

    def __str__(self) -> str:
        message = self.message
        if self.url:
            message += f"\n(background on this error at: {self.url})"
        return message

    def __repr__(self) -> str:
        return f"{type(self).__name__}('{self}')"


class CallbackAnswerException(AiogramError):
    """
    Exception for callback answer.
    """


class SceneException(AiogramError):
    """
    Exception for scenes.
    """


class UnsupportedKeywordArgument(DetailedAiogramError):
    """
    Exception raised when a keyword argument is passed as filter.
    """

    url = docs_url("migration_2_to_3.html", fragment_="filtering-events")


class TelegramAPIError(DetailedAiogramError):
    """
    Base exception for all Telegram API errors.
    """

    label: str = "Telegram server says"

    def __init__(
        self,
        method: TelegramMethod[TelegramType],
        message: str,
    ) -> None:
        super().__init__(message=message)
        self.method = method

    def __str__(self) -> str:
        original_message = super().__str__()
        return f"{self.label} - {original_message}"


class TelegramNetworkError(TelegramAPIError):
    """
    Base exception for all Telegram network errors.
    """

    label = "HTTP Client says"


class TelegramRetryAfter(TelegramAPIError):
    """
    Exception raised when flood control exceeds.
    """

    url = "https://core.telegram.org/bots/faq#my-bot-is-hitting-limits-how-do-i-avoid-this"

    def __init__(
        self,
        method: TelegramMethod[TelegramType],
        message: str,
        retry_after: int,
    ) -> None:
        description = f"Flood control exceeded on method {type(method).__name__!r}"
        if chat_id := getattr(method, "chat_id", None):
            description += f" in chat {chat_id}"
        description += f". Retry in {retry_after} seconds."
        description += f"\nOriginal description: {message}"

        super().__init__(method=method, message=description)
        self.retry_after = retry_after


class TelegramMigrateToChat(TelegramAPIError):
    """
    Exception raised when chat has been migrated to a supergroup.
    """

    url = "https://core.telegram.org/bots/api#responseparameters"

    def __init__(
        self,
        method: TelegramMethod[TelegramType],
        message: str,
        migrate_to_chat_id: int,
    ) -> None:
        description = f"The group has been migrated to a supergroup with id {migrate_to_chat_id}"
        if chat_id := getattr(method, "chat_id", None):
            description += f" from {chat_id}"
        description += f"\nOriginal description: {message}"
        super().__init__(method=method, message=description)
        self.migrate_to_chat_id = migrate_to_chat_id


class TelegramBadRequest(TelegramAPIError):
    """
    Exception raised when request is malformed.
    """


class TelegramNotFound(TelegramAPIError):
    """
    Exception raised when chat, message, user, etc. not found.
    """


class TelegramConflictError(TelegramAPIError):
    """
    Exception raised when bot token is already used by another application in polling mode.
    """


class TelegramUnauthorizedError(TelegramAPIError):
    """
    Exception raised when bot token is invalid.
    """


class TelegramForbiddenError(TelegramAPIError):
    """
    Exception raised when bot is kicked from chat or etc.
    """


class TelegramServerError(TelegramAPIError):
    """
    Exception raised when Telegram server returns 5xx error.
    """


class RestartingTelegram(TelegramServerError):
    """
    Exception raised when Telegram server is restarting.

    It seems like this error is not used by Telegram anymore,
    but it's still here for backward compatibility.

    Currently, you should expect that Telegram can raise RetryAfter (with timeout 5 seconds)
     error instead of this one.
    """


class TelegramEntityTooLarge(TelegramNetworkError):
    """
    Exception raised when you are trying to send a file that is too large.
    """

    url = "https://core.telegram.org/bots/api#sending-files"


class ClientDecodeError(AiogramError):
    """
    Exception raised when client can't decode response. (Malformed response, etc.)
    """

    def __init__(self, message: str, original: Exception, data: Any) -> None:
        self.message = message
        self.original = original
        self.data = data

    def __str__(self) -> str:
        original_type = type(self.original)
        return (
            f"{self.message}\n"
            f"Caused from error: "
            f"{original_type.__module__}.{original_type.__name__}: {self.original}\n"
            f"Content: {self.data}"
        )


class DataNotDictLikeError(DetailedAiogramError):
    """
    Exception raised when data is not dict-like.
    """
