from __future__ import annotations

from typing import TYPE_CHECKING, Any

from aiogram.types import TelegramObject

if TYPE_CHECKING:
    from .chat_administrator_rights import ChatAdministratorRights


class KeyboardButtonRequestChat(TelegramObject):
    """
    This object defines the criteria used to request a suitable chat. Information about the selected chat will be shared with the bot when the corresponding button is pressed. The bot will be granted requested rights in the chat if appropriate. `More about requesting chats » <https://core.telegram.org/bots/features#chat-and-user-selection>`_.

    Source: https://core.telegram.org/bots/api#keyboardbuttonrequestchat
    """

    request_id: int
    """Signed 32-bit identifier of the request, which will be received back in the :class:`aiogram.types.chat_shared.ChatShared` object. Must be unique within the message"""
    chat_is_channel: bool
    """Pass :code:`True` to request a channel chat, pass :code:`False` to request a group or a supergroup chat"""
    chat_is_forum: bool | None = None
    """*Optional*. Pass :code:`True` to request a forum supergroup, pass :code:`False` to request a non-forum chat. If not specified, no additional restrictions are applied"""
    chat_has_username: bool | None = None
    """*Optional*. Pass :code:`True` to request a supergroup or a channel with a username, pass :code:`False` to request a chat without a username. If not specified, no additional restrictions are applied"""
    chat_is_created: bool | None = None
    """*Optional*. Pass :code:`True` to request a chat owned by the user. Otherwise, no additional restrictions are applied"""
    user_administrator_rights: ChatAdministratorRights | None = None
    """*Optional*. A JSON-serialized object listing the required administrator rights of the user in the chat. The rights must be a superset of *bot_administrator_rights*. If not specified, no additional restrictions are applied"""
    bot_administrator_rights: ChatAdministratorRights | None = None
    """*Optional*. A JSON-serialized object listing the required administrator rights of the bot in the chat. The rights must be a subset of *user_administrator_rights*. If not specified, no additional restrictions are applied"""
    bot_is_member: bool | None = None
    """*Optional*. Pass :code:`True` to request a chat with the bot as a member. Otherwise, no additional restrictions are applied"""
    request_title: bool | None = None
    """*Optional*. Pass :code:`True` to request the chat's title"""
    request_username: bool | None = None
    """*Optional*. Pass :code:`True` to request the chat's username"""
    request_photo: bool | None = None
    """*Optional*. Pass :code:`True` to request the chat's photo"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            request_id: int,
            chat_is_channel: bool,
            chat_is_forum: bool | None = None,
            chat_has_username: bool | None = None,
            chat_is_created: bool | None = None,
            user_administrator_rights: ChatAdministratorRights | None = None,
            bot_administrator_rights: ChatAdministratorRights | None = None,
            bot_is_member: bool | None = None,
            request_title: bool | None = None,
            request_username: bool | None = None,
            request_photo: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                request_id=request_id,
                chat_is_channel=chat_is_channel,
                chat_is_forum=chat_is_forum,
                chat_has_username=chat_has_username,
                chat_is_created=chat_is_created,
                user_administrator_rights=user_administrator_rights,
                bot_administrator_rights=bot_administrator_rights,
                bot_is_member=bot_is_member,
                request_title=request_title,
                request_username=request_username,
                request_photo=request_photo,
                **__pydantic_kwargs,
            )
