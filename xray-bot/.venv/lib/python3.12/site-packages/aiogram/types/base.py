from typing import Any
from unittest.mock import sentinel

from pydantic import BaseModel, ConfigDict, model_validator

from aiogram.client.context_controller import BotContextController
from aiogram.client.default import Default


class TelegramObject(BotContextController, BaseModel):
    model_config = ConfigDict(
        use_enum_values=True,
        extra="allow",
        validate_assignment=True,
        frozen=True,
        populate_by_name=True,
        arbitrary_types_allowed=True,
        defer_build=True,
        protected_namespaces=(),
    )

    @model_validator(mode="before")
    @classmethod
    def remove_unset(cls, values: dict[str, Any]) -> dict[str, Any]:
        """
        Remove UNSET before fields validation.

        We use UNSET as a sentinel value for `parse_mode` and replace it to real value later.
        It isn't a problem when it's just default value for a model field,
        but UNSET might be passed to a model initialization from `Bot.method_name`,
        so we must take care of it and remove it before fields validation.
        """
        if not isinstance(values, dict):
            return values
        return {k: v for k, v in values.items() if not isinstance(v, UNSET_TYPE)}


class MutableTelegramObject(TelegramObject):
    model_config = ConfigDict(
        frozen=False,
    )


# special sentinel object which used in a situation when None might be a useful value
UNSET: Any = sentinel.UNSET
UNSET_TYPE: Any = type(UNSET)

# Unused constants are needed only for backward compatibility with external
# libraries that a working with framework internals
UNSET_PARSE_MODE: Any = Default("parse_mode")
UNSET_DISABLE_WEB_PAGE_PREVIEW: Any = Default("link_preview_is_disabled")
UNSET_PROTECT_CONTENT: Any = Default("protect_content")
