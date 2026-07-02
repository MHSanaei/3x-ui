from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import ChatIdUnion, InlineKeyboardMarkup, Message
from .base import TelegramMethod


class EditMessageLiveLocation(TelegramMethod[Message | bool]):
    """
    Use this method to edit live location messages. A location can be edited until its *live_period* expires or editing is explicitly disabled by a call to :class:`aiogram.methods.stop_message_live_location.StopMessageLiveLocation`. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.

    Source: https://core.telegram.org/bots/api#editmessagelivelocation
    """

    __returning__ = Message | bool
    __api_method__ = "editMessageLiveLocation"

    latitude: float
    """Latitude of new location"""
    longitude: float
    """Longitude of new location"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message to be edited was sent"""
    chat_id: ChatIdUnion | None = None
    """Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`"""
    message_id: int | None = None
    """Required if *inline_message_id* is not specified. Identifier of the message to edit"""
    inline_message_id: str | None = None
    """Required if *chat_id* and *message_id* are not specified. Identifier of the inline message"""
    live_period: int | None = None
    """New period in seconds during which the location can be updated, starting from the message send date. If 0x7FFFFFFF is specified, then the location can be updated forever. Otherwise, the new value must not exceed the current *live_period* by more than a day, and the live location expiration date must remain within the next 90 days. If not specified, then *live_period* remains unchanged"""
    horizontal_accuracy: float | None = None
    """The radius of uncertainty for the location, measured in meters; 0-1500"""
    heading: int | None = None
    """Direction in which the user is moving, in degrees. Must be between 1 and 360 if specified"""
    proximity_alert_radius: int | None = None
    """The maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified"""
    reply_markup: InlineKeyboardMarkup | None = None
    """A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            latitude: float,
            longitude: float,
            business_connection_id: str | None = None,
            chat_id: ChatIdUnion | None = None,
            message_id: int | None = None,
            inline_message_id: str | None = None,
            live_period: int | None = None,
            horizontal_accuracy: float | None = None,
            heading: int | None = None,
            proximity_alert_radius: int | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                latitude=latitude,
                longitude=longitude,
                business_connection_id=business_connection_id,
                chat_id=chat_id,
                message_id=message_id,
                inline_message_id=inline_message_id,
                live_period=live_period,
                horizontal_accuracy=horizontal_accuracy,
                heading=heading,
                proximity_alert_radius=proximity_alert_radius,
                reply_markup=reply_markup,
                **__pydantic_kwargs,
            )
