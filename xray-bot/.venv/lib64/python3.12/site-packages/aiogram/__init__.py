from aiogram.dispatcher.flags import FlagGenerator

from . import enums, methods, types
from .__meta__ import __api_version__, __version__
from .client import session
from .client.bot import Bot
from .dispatcher.dispatcher import Dispatcher
from .dispatcher.middlewares.base import BaseMiddleware
from .dispatcher.router import Router
from .utils.magic_filter import MagicFilter
from .utils.text_decorations import html_decoration as html
from .utils.text_decorations import markdown_decoration as md

F = MagicFilter()
flags = FlagGenerator()

__all__ = (
    "BaseMiddleware",
    "Bot",
    "Dispatcher",
    "F",
    "Router",
    "__api_version__",
    "__version__",
    "enums",
    "flags",
    "html",
    "md",
    "methods",
    "session",
    "types",
)
