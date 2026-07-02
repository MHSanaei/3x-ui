from __future__ import annotations

from typing import TypeAlias

from .force_reply import ForceReply
from .inline_keyboard_markup import InlineKeyboardMarkup
from .reply_keyboard_markup import ReplyKeyboardMarkup
from .reply_keyboard_remove import ReplyKeyboardRemove

ReplyMarkupUnion: TypeAlias = (
    InlineKeyboardMarkup | ReplyKeyboardMarkup | ReplyKeyboardRemove | ForceReply
)
