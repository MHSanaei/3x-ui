from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InlineQueryResultUnion, SentWebAppMessage
from .base import TelegramMethod


class AnswerWebAppQuery(TelegramMethod[SentWebAppMessage]):
    """
    Use this method to set the result of an interaction with a `Web App <https://core.telegram.org/bots/webapps>`_ and send a corresponding message on behalf of the user to the chat from which the query originated. On success, a :class:`aiogram.types.sent_web_app_message.SentWebAppMessage` object is returned.

    Source: https://core.telegram.org/bots/api#answerwebappquery
    """

    __returning__ = SentWebAppMessage
    __api_method__ = "answerWebAppQuery"

    web_app_query_id: str
    """Unique identifier for the query to be answered"""
    result: InlineQueryResultUnion
    """A JSON-serialized object describing the message to be sent"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            web_app_query_id: str,
            result: InlineQueryResultUnion,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(web_app_query_id=web_app_query_id, result=result, **__pydantic_kwargs)
