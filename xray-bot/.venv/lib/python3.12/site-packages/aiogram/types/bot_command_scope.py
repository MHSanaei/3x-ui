from __future__ import annotations

from .base import TelegramObject


class BotCommandScope(TelegramObject):
    """
    This object represents the scope to which bot commands are applied. Currently, the following 7 scopes are supported:

     - :class:`aiogram.types.bot_command_scope_default.BotCommandScopeDefault`
     - :class:`aiogram.types.bot_command_scope_all_private_chats.BotCommandScopeAllPrivateChats`
     - :class:`aiogram.types.bot_command_scope_all_group_chats.BotCommandScopeAllGroupChats`
     - :class:`aiogram.types.bot_command_scope_all_chat_administrators.BotCommandScopeAllChatAdministrators`
     - :class:`aiogram.types.bot_command_scope_chat.BotCommandScopeChat`
     - :class:`aiogram.types.bot_command_scope_chat_administrators.BotCommandScopeChatAdministrators`
     - :class:`aiogram.types.bot_command_scope_chat_member.BotCommandScopeChatMember`

    Source: https://core.telegram.org/bots/api#botcommandscope
    """
