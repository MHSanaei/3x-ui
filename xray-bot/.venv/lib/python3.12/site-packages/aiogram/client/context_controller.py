from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import BaseModel, PrivateAttr
from typing_extensions import Self

if TYPE_CHECKING:
    from aiogram.client.bot import Bot


class BotContextController(BaseModel):
    _bot: Bot | None = PrivateAttr()

    def model_post_init(self, __context: Any) -> None:  # noqa: PYI063
        self._bot = __context.get("bot") if __context else None

    def as_(self, bot: Bot | None) -> Self:
        """
        Bind object to a bot instance.

        :param bot: Bot instance
        :return: self
        """
        self._bot = bot
        return self

    @property
    def bot(self) -> Bot | None:
        """
        Get bot instance.

        :return: Bot instance
        """
        return self._bot
