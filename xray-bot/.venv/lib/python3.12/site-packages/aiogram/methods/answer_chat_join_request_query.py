from typing import TYPE_CHECKING, Any

from .base import TelegramMethod


class AnswerChatJoinRequestQuery(TelegramMethod[bool]):
    """
    Use this method to process a received chat join request query. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#answerchatjoinrequestquery
    """

    __returning__ = bool
    __api_method__ = "answerChatJoinRequestQuery"

    chat_join_request_query_id: str
    """Unique identifier of the join request query"""
    result: str
    """Result of the query. Must be either 'approve' to allow the user to join the chat, 'decline' to disallow the user to join the chat, or 'queue' to leave the decision to other administrators"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_join_request_query_id: str,
            result: str,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_join_request_query_id=chat_join_request_query_id,
                result=result,
                **__pydantic_kwargs,
            )
