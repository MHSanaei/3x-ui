from abc import ABC

from aiogram.handlers import BaseHandler
from aiogram.types import ChosenInlineResult, User


class ChosenInlineResultHandler(BaseHandler[ChosenInlineResult], ABC):
    """
    Base class for chosen inline result handlers
    """

    @property
    def from_user(self) -> User:
        return self.event.from_user

    @property
    def query(self) -> str:
        return self.event.query
