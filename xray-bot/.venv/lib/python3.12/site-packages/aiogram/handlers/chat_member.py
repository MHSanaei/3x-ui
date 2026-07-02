from abc import ABC

from aiogram.handlers import BaseHandler
from aiogram.types import ChatMemberUpdated, User


class ChatMemberHandler(BaseHandler[ChatMemberUpdated], ABC):
    """
    Base class for chat member updated events
    """

    @property
    def from_user(self) -> User:
        return self.event.from_user
