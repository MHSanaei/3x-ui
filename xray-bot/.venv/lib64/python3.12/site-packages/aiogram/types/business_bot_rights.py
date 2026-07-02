from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject


class BusinessBotRights(TelegramObject):
    """
    Represents the rights of a business bot.

    Source: https://core.telegram.org/bots/api#businessbotrights
    """

    can_reply: bool | None = None
    """*Optional*. :code:`True`, if the bot can send and edit messages in the private chats that had incoming messages in the last 24 hours"""
    can_read_messages: bool | None = None
    """*Optional*. :code:`True`, if the bot can mark incoming private messages as read"""
    can_delete_sent_messages: bool | None = None
    """*Optional*. :code:`True`, if the bot can delete messages sent by the bot"""
    can_delete_all_messages: bool | None = None
    """*Optional*. :code:`True`, if the bot can delete all private messages in managed chats"""
    can_edit_name: bool | None = None
    """*Optional*. :code:`True`, if the bot can edit the first and last name of the business account"""
    can_edit_bio: bool | None = None
    """*Optional*. :code:`True`, if the bot can edit the bio of the business account"""
    can_edit_profile_photo: bool | None = None
    """*Optional*. :code:`True`, if the bot can edit the profile photo of the business account"""
    can_edit_username: bool | None = None
    """*Optional*. :code:`True`, if the bot can edit the username of the business account"""
    can_change_gift_settings: bool | None = None
    """*Optional*. :code:`True`, if the bot can change the privacy settings pertaining to gifts for the business account"""
    can_view_gifts_and_stars: bool | None = None
    """*Optional*. :code:`True`, if the bot can view gifts and the amount of Telegram Stars owned by the business account"""
    can_convert_gifts_to_stars: bool | None = None
    """*Optional*. :code:`True`, if the bot can convert regular gifts owned by the business account to Telegram Stars"""
    can_transfer_and_upgrade_gifts: bool | None = None
    """*Optional*. :code:`True`, if the bot can transfer and upgrade gifts owned by the business account"""
    can_transfer_stars: bool | None = None
    """*Optional*. :code:`True`, if the bot can transfer Telegram Stars received by the business account to its own account, or use them to upgrade and transfer gifts"""
    can_manage_stories: bool | None = None
    """*Optional*. :code:`True`, if the bot can post, edit and delete stories on behalf of the business account"""
    can_delete_outgoing_messages: bool | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. True, if the bot can delete messages sent by the bot

.. deprecated:: API:9.1
   https://core.telegram.org/bots/api-changelog#july-3-2025"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            can_reply: bool | None = None,
            can_read_messages: bool | None = None,
            can_delete_sent_messages: bool | None = None,
            can_delete_all_messages: bool | None = None,
            can_edit_name: bool | None = None,
            can_edit_bio: bool | None = None,
            can_edit_profile_photo: bool | None = None,
            can_edit_username: bool | None = None,
            can_change_gift_settings: bool | None = None,
            can_view_gifts_and_stars: bool | None = None,
            can_convert_gifts_to_stars: bool | None = None,
            can_transfer_and_upgrade_gifts: bool | None = None,
            can_transfer_stars: bool | None = None,
            can_manage_stories: bool | None = None,
            can_delete_outgoing_messages: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                can_reply=can_reply,
                can_read_messages=can_read_messages,
                can_delete_sent_messages=can_delete_sent_messages,
                can_delete_all_messages=can_delete_all_messages,
                can_edit_name=can_edit_name,
                can_edit_bio=can_edit_bio,
                can_edit_profile_photo=can_edit_profile_photo,
                can_edit_username=can_edit_username,
                can_change_gift_settings=can_change_gift_settings,
                can_view_gifts_and_stars=can_view_gifts_and_stars,
                can_convert_gifts_to_stars=can_convert_gifts_to_stars,
                can_transfer_and_upgrade_gifts=can_transfer_and_upgrade_gifts,
                can_transfer_stars=can_transfer_stars,
                can_manage_stories=can_manage_stories,
                can_delete_outgoing_messages=can_delete_outgoing_messages,
                **__pydantic_kwargs,
            )
