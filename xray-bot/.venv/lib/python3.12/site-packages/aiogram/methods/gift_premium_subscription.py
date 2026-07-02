from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import MessageEntity
from .base import TelegramMethod


class GiftPremiumSubscription(TelegramMethod[bool]):
    """
    Gifts a Telegram Premium subscription to the given user. Returns :code:`True` on success.

    Source: https://core.telegram.org/bots/api#giftpremiumsubscription
    """

    __returning__ = bool
    __api_method__ = "giftPremiumSubscription"

    user_id: int
    """Unique identifier of the target user who will receive a Telegram Premium subscription"""
    month_count: int
    """Number of months the Telegram Premium subscription will be active for the user; must be one of 3, 6, or 12"""
    star_count: int
    """Number of Telegram Stars to pay for the Telegram Premium subscription; must be 1000 for 3 months, 1500 for 6 months, and 2500 for 12 months"""
    text: str | None = None
    """Text that will be shown along with the service message about the subscription; 0-128 characters"""
    text_parse_mode: str | None = None
    """Mode for parsing entities in the text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored"""
    text_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the gift text. It can be specified instead of *text_parse_mode*. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            user_id: int,
            month_count: int,
            star_count: int,
            text: str | None = None,
            text_parse_mode: str | None = None,
            text_entities: list[MessageEntity] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                user_id=user_id,
                month_count=month_count,
                star_count=star_count,
                text=text,
                text_parse_mode=text_parse_mode,
                text_entities=text_entities,
                **__pydantic_kwargs,
            )
