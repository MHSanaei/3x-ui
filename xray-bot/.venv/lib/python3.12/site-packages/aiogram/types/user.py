from __future__ import annotations

from typing import TYPE_CHECKING, Any

from ..utils import markdown
from ..utils.link import create_tg_link
from .base import TelegramObject

if TYPE_CHECKING:
    from ..methods import GetUserProfileAudios, GetUserProfilePhotos


class User(TelegramObject):
    """
    This object represents a Telegram user or bot.

    Source: https://core.telegram.org/bots/api#user
    """

    id: int
    """Unique identifier for this user or bot. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a 64-bit integer or double-precision float type are safe for storing this identifier"""
    is_bot: bool
    """:code:`True`, if this user is a bot"""
    first_name: str
    """User's or bot's first name"""
    last_name: str | None = None
    """*Optional*. User's or bot's last name"""
    username: str | None = None
    """*Optional*. User's or bot's username"""
    language_code: str | None = None
    """*Optional*. `IETF language tag <https://en.wikipedia.org/wiki/IETF_language_tag>`_ of the user's language"""
    is_premium: bool | None = None
    """*Optional*. :code:`True`, if this user is a Telegram Premium user"""
    added_to_attachment_menu: bool | None = None
    """*Optional*. :code:`True`, if this user added the bot to the attachment menu"""
    can_join_groups: bool | None = None
    """*Optional*. :code:`True`, if the bot can be invited to groups. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    can_read_all_group_messages: bool | None = None
    """*Optional*. :code:`True`, if `privacy mode <https://core.telegram.org/bots/features#privacy-mode>`_ is disabled for the bot. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    supports_guest_queries: bool | None = None
    """*Optional*. :code:`True`, if the bot supports guest queries from chats it is not a member of. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    supports_inline_queries: bool | None = None
    """*Optional*. :code:`True`, if the bot supports inline queries. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    can_connect_to_business: bool | None = None
    """*Optional*. :code:`True`, if the bot can be connected to a user account to manage it. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    has_main_web_app: bool | None = None
    """*Optional*. :code:`True`, if the bot has a main Web App. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    has_topics_enabled: bool | None = None
    """*Optional*. :code:`True`, if the bot has forum topic mode enabled in private chats. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    allows_users_to_create_topics: bool | None = None
    """*Optional*. :code:`True`, if the bot allows users to create and delete topics in private chats. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    can_manage_bots: bool | None = None
    """*Optional*. :code:`True`, if other bots can be created to be controlled by the bot. Returned only in :class:`aiogram.methods.get_me.GetMe`"""
    supports_join_request_queries: bool | None = None
    """*Optional*. :code:`True`, if the bot supports join request queries and can be assigned to process them. Returned only in :class:`aiogram.methods.get_me.GetMe`"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: int,
            is_bot: bool,
            first_name: str,
            last_name: str | None = None,
            username: str | None = None,
            language_code: str | None = None,
            is_premium: bool | None = None,
            added_to_attachment_menu: bool | None = None,
            can_join_groups: bool | None = None,
            can_read_all_group_messages: bool | None = None,
            supports_guest_queries: bool | None = None,
            supports_inline_queries: bool | None = None,
            can_connect_to_business: bool | None = None,
            has_main_web_app: bool | None = None,
            has_topics_enabled: bool | None = None,
            allows_users_to_create_topics: bool | None = None,
            can_manage_bots: bool | None = None,
            supports_join_request_queries: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                is_bot=is_bot,
                first_name=first_name,
                last_name=last_name,
                username=username,
                language_code=language_code,
                is_premium=is_premium,
                added_to_attachment_menu=added_to_attachment_menu,
                can_join_groups=can_join_groups,
                can_read_all_group_messages=can_read_all_group_messages,
                supports_guest_queries=supports_guest_queries,
                supports_inline_queries=supports_inline_queries,
                can_connect_to_business=can_connect_to_business,
                has_main_web_app=has_main_web_app,
                has_topics_enabled=has_topics_enabled,
                allows_users_to_create_topics=allows_users_to_create_topics,
                can_manage_bots=can_manage_bots,
                supports_join_request_queries=supports_join_request_queries,
                **__pydantic_kwargs,
            )

    @property
    def full_name(self) -> str:
        if self.last_name:
            return f"{self.first_name} {self.last_name}"
        return self.first_name

    @property
    def url(self) -> str:
        return create_tg_link("user", id=self.id)

    def mention_markdown(self, name: str | None = None) -> str:
        if name is None:
            name = self.full_name
        return markdown.link(name, self.url)

    def mention_html(self, name: str | None = None) -> str:
        if name is None:
            name = self.full_name
        return markdown.hlink(name, self.url)

    def get_profile_photos(
        self,
        offset: int | None = None,
        limit: int | None = None,
        **kwargs: Any,
    ) -> GetUserProfilePhotos:
        """
        Shortcut for method :class:`aiogram.methods.get_user_profile_photos.GetUserProfilePhotos`
        will automatically fill method attributes:

        - :code:`user_id`

        Use this method to get a list of profile pictures for a user. Returns a :class:`aiogram.types.user_profile_photos.UserProfilePhotos` object.

        Source: https://core.telegram.org/bots/api#getuserprofilephotos

        :param offset: Sequential number of the first photo to be returned. By default, all photos are returned
        :param limit: Limits the number of photos to be retrieved. Values between 1-100 are accepted. Defaults to 100
        :return: instance of method :class:`aiogram.methods.get_user_profile_photos.GetUserProfilePhotos`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import GetUserProfilePhotos

        return GetUserProfilePhotos(
            user_id=self.id,
            offset=offset,
            limit=limit,
            **kwargs,
        ).as_(self._bot)

    def get_profile_audios(
        self,
        offset: int | None = None,
        limit: int | None = None,
        **kwargs: Any,
    ) -> GetUserProfileAudios:
        """
        Shortcut for method :class:`aiogram.methods.get_user_profile_audios.GetUserProfileAudios`
        will automatically fill method attributes:

        - :code:`user_id`

        Use this method to get a list of profile audios for a user. Returns a :class:`aiogram.types.user_profile_audios.UserProfileAudios` object.

        Source: https://core.telegram.org/bots/api#getuserprofileaudios

        :param offset: Sequential number of the first audio to be returned. By default, all audios are returned
        :param limit: Limits the number of audios to be retrieved. Values between 1-100 are accepted. Defaults to 100
        :return: instance of method :class:`aiogram.methods.get_user_profile_audios.GetUserProfileAudios`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import GetUserProfileAudios

        return GetUserProfileAudios(
            user_id=self.id,
            offset=offset,
            limit=limit,
            **kwargs,
        ).as_(self._bot)
