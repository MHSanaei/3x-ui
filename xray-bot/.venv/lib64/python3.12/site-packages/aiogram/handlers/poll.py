from abc import ABC

from aiogram.handlers import BaseHandler
from aiogram.types import Poll, PollOption


class PollHandler(BaseHandler[Poll], ABC):
    """
    Base class for poll handlers
    """

    @property
    def question(self) -> str:
        return self.event.question

    @property
    def options(self) -> list[PollOption]:
        return self.event.options
