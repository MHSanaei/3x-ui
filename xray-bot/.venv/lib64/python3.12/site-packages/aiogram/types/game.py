from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .animation import Animation
    from .message_entity import MessageEntity
    from .photo_size import PhotoSize


class Game(TelegramObject):
    """
    This object represents a game. Use BotFather to create and edit games, their short names will act as unique identifiers.

    Source: https://core.telegram.org/bots/api#game
    """

    title: str
    """Title of the game"""
    description: str
    """Description of the game"""
    photo: list[PhotoSize]
    """Photo that will be displayed in the game message in chats"""
    text: str | None = None
    """*Optional*. Brief description of the game or high scores included in the game message. Can be automatically edited to include current high scores for the game when the bot calls :class:`aiogram.methods.set_game_score.SetGameScore`, or manually edited using :class:`aiogram.methods.edit_message_text.EditMessageText`. 0-4096 characters"""
    text_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in *text*, such as usernames, URLs, bot commands, etc"""
    animation: Animation | None = None
    """*Optional*. Animation that will be displayed in the game message in chats. Upload via `BotFather <https://t.me/botfather>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            title: str,
            description: str,
            photo: list[PhotoSize],
            text: str | None = None,
            text_entities: list[MessageEntity] | None = None,
            animation: Animation | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                title=title,
                description=description,
                photo=photo,
                text=text,
                text_entities=text_entities,
                animation=animation,
                **__pydantic_kwargs,
            )
