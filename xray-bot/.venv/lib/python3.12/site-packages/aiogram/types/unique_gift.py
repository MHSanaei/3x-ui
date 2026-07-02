from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .chat import Chat
    from .unique_gift_backdrop import UniqueGiftBackdrop
    from .unique_gift_colors import UniqueGiftColors
    from .unique_gift_model import UniqueGiftModel
    from .unique_gift_symbol import UniqueGiftSymbol


class UniqueGift(TelegramObject):
    """
    This object describes a unique gift that was upgraded from a regular gift.

    Source: https://core.telegram.org/bots/api#uniquegift
    """

    gift_id: str
    """Identifier of the regular gift from which the gift was upgraded"""
    base_name: str
    """Human-readable name of the regular gift from which this unique gift was upgraded"""
    name: str
    """Unique name of the gift. This name can be used in :code:`https://t.me/nft/...` links and story areas"""
    number: int
    """Unique number of the upgraded gift among gifts upgraded from the same regular gift"""
    model: UniqueGiftModel
    """Model of the gift"""
    symbol: UniqueGiftSymbol
    """Symbol of the gift"""
    backdrop: UniqueGiftBackdrop
    """Backdrop of the gift"""
    is_premium: bool | None = None
    """*Optional*. :code:`True`, if the original regular gift was exclusively purchaseable by Telegram Premium subscribers"""
    is_burned: bool | None = None
    """*Optional*. :code:`True`, if the gift was used to craft another gift and isn't available anymore"""
    is_from_blockchain: bool | None = None
    """*Optional*. :code:`True`, if the gift is assigned from the TON blockchain and can't be resold or transferred in Telegram"""
    colors: UniqueGiftColors | None = None
    """*Optional*. The color scheme that can be used by the gift's owner for the chat's name, replies to messages and link previews; for business account gifts and gifts that are currently on sale only"""
    publisher_chat: Chat | None = None
    """*Optional*. Information about the chat that published the gift"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            gift_id: str,
            base_name: str,
            name: str,
            number: int,
            model: UniqueGiftModel,
            symbol: UniqueGiftSymbol,
            backdrop: UniqueGiftBackdrop,
            is_premium: bool | None = None,
            is_burned: bool | None = None,
            is_from_blockchain: bool | None = None,
            colors: UniqueGiftColors | None = None,
            publisher_chat: Chat | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                gift_id=gift_id,
                base_name=base_name,
                name=name,
                number=number,
                model=model,
                symbol=symbol,
                backdrop=backdrop,
                is_premium=is_premium,
                is_burned=is_burned,
                is_from_blockchain=is_from_blockchain,
                colors=colors,
                publisher_chat=publisher_chat,
                **__pydantic_kwargs,
            )
