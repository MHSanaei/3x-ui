from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject
from .custom import DateTime


class WebhookInfo(TelegramObject):
    """
    Describes the current status of a webhook.

    Source: https://core.telegram.org/bots/api#webhookinfo
    """

    url: str
    """Webhook URL, may be empty if webhook is not set up"""
    has_custom_certificate: bool
    """:code:`True`, if a custom certificate was provided for webhook certificate checks"""
    pending_update_count: int
    """Number of updates awaiting delivery"""
    ip_address: str | None = None
    """*Optional*. Currently used webhook IP address"""
    last_error_date: DateTime | None = None
    """*Optional*. Unix time for the most recent error that happened when trying to deliver an update via webhook"""
    last_error_message: str | None = None
    """*Optional*. Error message in human-readable format for the most recent error that happened when trying to deliver an update via webhook"""
    last_synchronization_error_date: DateTime | None = None
    """*Optional*. Unix time of the most recent error that happened when trying to synchronize available updates with Telegram datacenters"""
    max_connections: int | None = None
    """*Optional*. The maximum allowed number of simultaneous HTTPS connections to the webhook for update delivery"""
    allowed_updates: list[str] | None = None
    """*Optional*. A list of update types the bot is subscribed to. Defaults to all update types except *chat_member*, *message_reaction*, and *message_reaction_count*"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            url: str,
            has_custom_certificate: bool,
            pending_update_count: int,
            ip_address: str | None = None,
            last_error_date: DateTime | None = None,
            last_error_message: str | None = None,
            last_synchronization_error_date: DateTime | None = None,
            max_connections: int | None = None,
            allowed_updates: list[str] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                url=url,
                has_custom_certificate=has_custom_certificate,
                pending_update_count=pending_update_count,
                ip_address=ip_address,
                last_error_date=last_error_date,
                last_error_message=last_error_message,
                last_synchronization_error_date=last_synchronization_error_date,
                max_connections=max_connections,
                allowed_updates=allowed_updates,
                **__pydantic_kwargs,
            )
