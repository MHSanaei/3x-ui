from abc import ABC

from aiogram.handlers.base import BaseHandler


class ErrorHandler(BaseHandler[Exception], ABC):
    """
    Base class for errors handlers
    """

    @property
    def exception_name(self) -> str:
        return self.event.__class__.__name__

    @property
    def exception_message(self) -> str:
        return str(self.event)
