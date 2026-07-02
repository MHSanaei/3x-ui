from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..types import InputFile
from .base import TelegramMethod


class SetWebhook(TelegramMethod[bool]):
    """
    Use this method to specify a URL and receive incoming updates via an outgoing webhook. Whenever there is an update for the bot, we will send an HTTPS POST request to the specified URL, containing a JSON-serialized :class:`aiogram.types.update.Update`. In case of an unsuccessful request (a request with response `HTTP status code <https://en.wikipedia.org/wiki/List_of_HTTP_status_codes>`_ different from :code:`2XY`), we will repeat the request and give up after a reasonable amount of attempts. Returns :code:`True` on success.
    If you'd like to make sure that the webhook was set by you, you can specify secret data in the parameter *secret_token*. If specified, the request will contain a header 'X-Telegram-Bot-Api-Secret-Token' with the secret token as content.

     **Notes**

     **1.** You will not be able to receive updates using :class:`aiogram.methods.get_updates.GetUpdates` for as long as an outgoing webhook is set up.

     **2.** To use a self-signed certificate, you need to upload your `public key certificate <https://core.telegram.org/bots/self-signed>`_ using *certificate* parameter. Please upload as InputFile, sending a String will not work.

     **3.** Ports currently supported *for webhooks*: **443, 80, 88, 8443**.
     If you're having any trouble setting up webhooks, please check out this `amazing guide to webhooks <https://core.telegram.org/bots/webhooks>`_.

    Source: https://core.telegram.org/bots/api#setwebhook
    """

    __returning__ = bool
    __api_method__ = "setWebhook"

    url: str
    """HTTPS URL to send updates to. Use an empty string to remove webhook integration"""
    certificate: InputFile | None = None
    """Upload your public key certificate so that the root certificate in use can be checked. See our `self-signed guide <https://core.telegram.org/bots/self-signed>`_ for details"""
    ip_address: str | None = None
    """The fixed IP address which will be used to send webhook requests instead of the IP address resolved through DNS"""
    max_connections: int | None = None
    """The maximum allowed number of simultaneous HTTPS connections to the webhook for update delivery, 1-100. Defaults to *40*. Use lower values to limit the load on your bot's server, and higher values to increase your bot's throughput"""
    allowed_updates: list[str] | None = None
    """A JSON-serialized list of the update types you want your bot to receive. For example, specify :code:`["message", "edited_channel_post", "callback_query"]` to only receive updates of these types. See :class:`aiogram.types.update.Update` for a complete list of available update types. Specify an empty list to receive all update types except *chat_member*, *message_reaction*, and *message_reaction_count* (default). If not specified, the previous setting will be used"""
    drop_pending_updates: bool | None = None
    """Pass :code:`True` to drop all pending updates"""
    secret_token: str | None = None
    """A secret token to be sent in a header 'X-Telegram-Bot-Api-Secret-Token' in every webhook request, 1-256 characters. Only characters :code:`A-Z`, :code:`a-z`, :code:`0-9`, :code:`_` and :code:`-` are allowed. The header is useful to ensure that the request comes from a webhook set by you"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            url: str,
            certificate: InputFile | None = None,
            ip_address: str | None = None,
            max_connections: int | None = None,
            allowed_updates: list[str] | None = None,
            drop_pending_updates: bool | None = None,
            secret_token: str | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                url=url,
                certificate=certificate,
                ip_address=ip_address,
                max_connections=max_connections,
                allowed_updates=allowed_updates,
                drop_pending_updates=drop_pending_updates,
                secret_token=secret_token,
                **__pydantic_kwargs,
            )
