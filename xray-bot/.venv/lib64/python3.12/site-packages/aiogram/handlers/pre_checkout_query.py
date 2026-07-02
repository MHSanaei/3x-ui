from abc import ABC

from aiogram.handlers import BaseHandler
from aiogram.types import PreCheckoutQuery, User


class PreCheckoutQueryHandler(BaseHandler[PreCheckoutQuery], ABC):
    """
    Base class for pre-checkout handlers
    """

    @property
    def from_user(self) -> User:
        return self.event.from_user
