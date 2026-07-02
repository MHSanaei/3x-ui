from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .gift_background import GiftBackground
    from .sticker import Sticker


class Gift(TelegramObject):
    """
    This object represents a gift that can be sent by the bot.

    Source: https://core.telegram.org/bots/api#gift
    """

    id: str
    """Unique identifier of the gift"""
    sticker: Sticker
    """The sticker that represents the gift"""
    star_count: int
    """The number of Telegram Stars that must be paid to send the sticker"""
    upgrade_star_count: int | None = None
    """*Optional*. The number of Telegram Stars that must be paid to upgrade the gift to a unique one"""
    is_premium: bool | None = None
    """*Optional*. :code:`True`, if the gift can only be purchased by Telegram Premium subscribers"""
    has_colors: bool | None = None
    """*Optional*. :code:`True`, if the gift can be used (after being upgraded) to customize a user's appearance"""
    total_count: int | None = None
    """*Optional*. The total number of gifts of this type that can be sent by all users; for limited gifts only"""
    remaining_count: int | None = None
    """*Optional*. The number of remaining gifts of this type that can be sent by all users; for limited gifts only"""
    personal_total_count: int | None = None
    """*Optional*. The total number of gifts of this type that can be sent by the bot; for limited gifts only"""
    personal_remaining_count: int | None = None
    """*Optional*. The number of remaining gifts of this type that can be sent by the bot; for limited gifts only"""
    background: GiftBackground | None = None
    """*Optional*. Background of the gift"""
    unique_gift_variant_count: int | None = None
    """*Optional*. The total number of different unique gifts that can be obtained by upgrading the gift"""
    publisher_chat: Chat | None = None
    """*Optional*. Information about the chat that published the gift"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            sticker: Sticker,
            star_count: int,
            upgrade_star_count: int | None = None,
            is_premium: bool | None = None,
            has_colors: bool | None = None,
            total_count: int | None = None,
            remaining_count: int | None = None,
            personal_total_count: int | None = None,
            personal_remaining_count: int | None = None,
            background: GiftBackground | None = None,
            unique_gift_variant_count: int | None = None,
            publisher_chat: Chat | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                sticker=sticker,
                star_count=star_count,
                upgrade_star_count=upgrade_star_count,
                is_premium=is_premium,
                has_colors=has_colors,
                total_count=total_count,
                remaining_count=remaining_count,
                personal_total_count=personal_total_count,
                personal_remaining_count=personal_remaining_count,
                background=background,
                unique_gift_variant_count=unique_gift_variant_count,
                publisher_chat=publisher_chat,
                **__pydantic_kwargs,
            )
