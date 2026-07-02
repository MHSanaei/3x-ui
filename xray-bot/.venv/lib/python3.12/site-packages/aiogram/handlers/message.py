from abc import ABC
from typing import cast

from aiogram.filters import CommandObject
from aiogram.handlers.base import BaseHandler, BaseHandlerMixin
from aiogram.types import Chat, Message, User


class MessageHandler(BaseHandler[Message], ABC):
    """
    Base class for message handlers
    """

    @property
    def from_user(self) -> User | None:
        return self.event.from_user

    @property
    def chat(self) -> Chat:
        return self.event.chat


class MessageHandlerCommandMixin(BaseHandlerMixin[Message]):
    @property
    def command(self) -> CommandObject | None:
        if "command" in self.data:
            return cast(CommandObject, self.data["command"])
        return None
