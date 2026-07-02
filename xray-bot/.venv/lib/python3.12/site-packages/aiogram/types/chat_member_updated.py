from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from ..methods import (
        SendAnimation,
        SendAudio,
        SendContact,
        SendDice,
        SendDocument,
        SendGame,
        SendInvoice,
        SendLocation,
        SendMediaGroup,
        SendMessage,
        SendPhoto,
        SendPoll,
        SendSticker,
        SendVenue,
        SendVideo,
        SendVideoNote,
        SendVoice,
    )
    from .chat import Chat
    from .chat_invite_link import ChatInviteLink
    from .chat_member_union import ChatMemberUnion
    from .date_time_union import DateTimeUnion
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .input_file import InputFile
    from .input_file_union import InputFileUnion
    from .input_poll_media_union import InputPollMediaUnion
    from .input_poll_option_union import InputPollOptionUnion
    from .labeled_price import LabeledPrice
    from .link_preview_options import LinkPreviewOptions
    from .media_union import MediaUnion
    from .message_entity import MessageEntity
    from .reply_markup_union import ReplyMarkupUnion
    from .reply_parameters import ReplyParameters
    from .suggested_post_parameters import SuggestedPostParameters
    from .user import User


class ChatMemberUpdated(TelegramObject):
    """
    This object represents changes in the status of a chat member.

    Source: https://core.telegram.org/bots/api#chatmemberupdated
    """

    chat: Chat
    """Chat the user belongs to"""
    from_user: User = Field(..., alias="from")
    """Performer of the action, which resulted in the change"""
    date: DateTime
    """Date the change was done in Unix time"""
    old_chat_member: ChatMemberUnion
    """Previous information about the chat member"""
    new_chat_member: ChatMemberUnion
    """New information about the chat member"""
    invite_link: ChatInviteLink | None = None
    """*Optional*. Chat invite link, which was used by the user to join the chat; for joining by invite link events only"""
    via_join_request: bool | None = None
    """*Optional*. :code:`True`, if the user joined the chat after sending a direct join request without using an invite link and being approved by an administrator"""
    via_chat_folder_invite_link: bool | None = None
    """*Optional*. :code:`True`, if the user joined the chat via a chat folder invite link"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat: Chat,
            from_user: User,
            date: DateTime,
            old_chat_member: ChatMemberUnion,
            new_chat_member: ChatMemberUnion,
            invite_link: ChatInviteLink | None = None,
            via_join_request: bool | None = None,
            via_chat_folder_invite_link: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat=chat,
                from_user=from_user,
                date=date,
                old_chat_member=old_chat_member,
                new_chat_member=new_chat_member,
                invite_link=invite_link,
                via_join_request=via_join_request,
                via_chat_folder_invite_link=via_chat_folder_invite_link,
                **__pydantic_kwargs,
            )

    def answer(
        self,
        text: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        entities: list[MessageEntity] | None = None,
        link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview"),
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        disable_web_page_preview: bool | Default | None = Default("link_preview_is_disabled"),
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendMessage:
        """
        Shortcut for method :class:`aiogram.methods.send_message.SendMessage`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send text messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendmessage

        :param text: Text of the message to be sent, 1-4096 characters after entities parsing
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param link_preview_options: Link preview generation options for the message
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param disable_web_page_preview: Disables link previews for links in this message
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_message.SendMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendMessage

        return SendMessage(
            chat_id=self.chat.id,
            text=text,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            parse_mode=parse_mode,
            entities=entities,
            link_preview_options=link_preview_options,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            disable_web_page_preview=disable_web_page_preview,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_animation(
        self,
        animation: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        width: int | None = None,
        height: int | None = None,
        thumbnail: InputFile | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendAnimation:
        """
        Shortcut for method :class:`aiogram.methods.send_animation.SendAnimation`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send animation files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendanimation

        :param animation: Animation to send. Pass a file_id as String to send an animation that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an animation from the Internet, or upload a new animation using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent animation in seconds
        :param width: Animation width
        :param height: Animation height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Animation caption (may also be used when resending animation by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the animation caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the animation needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_animation.SendAnimation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendAnimation

        return SendAnimation(
            chat_id=self.chat.id,
            animation=animation,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            width=width,
            height=height,
            thumbnail=thumbnail,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_audio(
        self,
        audio: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        duration: int | None = None,
        performer: str | None = None,
        title: str | None = None,
        thumbnail: InputFile | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendAudio:
        """
        Shortcut for method :class:`aiogram.methods.send_audio.SendAudio`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio must be in the .MP3 or .M4A format. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send audio files of up to 50 MB in size, this limit may be changed in the future.
        For sending voice messages, use the :class:`aiogram.methods.send_voice.SendVoice` method instead.

        Source: https://core.telegram.org/bots/api#sendaudio

        :param audio: Audio file to send. Pass a file_id as String to send an audio file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an audio file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Audio caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the audio caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the audio in seconds
        :param performer: Performer
        :param title: Track name
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_audio.SendAudio`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendAudio

        return SendAudio(
            chat_id=self.chat.id,
            audio=audio,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            duration=duration,
            performer=performer,
            title=title,
            thumbnail=thumbnail,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_contact(
        self,
        phone_number: str,
        first_name: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        last_name: str | None = None,
        vcard: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendContact:
        """
        Shortcut for method :class:`aiogram.methods.send_contact.SendContact`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send phone contacts. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendcontact

        :param phone_number: Contact's phone number
        :param first_name: Contact's first name
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param last_name: Contact's last name
        :param vcard: Additional data about the contact in the form of a `vCard <https://en.wikipedia.org/wiki/VCard>`_, 0-2048 bytes
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_contact.SendContact`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendContact

        return SendContact(
            chat_id=self.chat.id,
            phone_number=phone_number,
            first_name=first_name,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            last_name=last_name,
            vcard=vcard,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_document(
        self,
        document: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        thumbnail: InputFile | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        disable_content_type_detection: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendDocument:
        """
        Shortcut for method :class:`aiogram.methods.send_document.SendDocument`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send general files. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send files of any type of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#senddocument

        :param document: File to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Document caption (may also be used when resending documents by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the document caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param disable_content_type_detection: Disables automatic server-side content type detection for files uploaded using multipart/form-data
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_document.SendDocument`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendDocument

        return SendDocument(
            chat_id=self.chat.id,
            document=document,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            thumbnail=thumbnail,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            disable_content_type_detection=disable_content_type_detection,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_game(
        self,
        game_short_name: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendGame:
        """
        Shortcut for method :class:`aiogram.methods.send_game.SendGame`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send a game. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendgame

        :param game_short_name: Short name of the game, serves as the unique identifier for the game. Set up your games via `@BotFather <https://t.me/botfather>`_
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Play game_title' button will be shown. If not empty, the first button must launch the game
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_game.SendGame`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendGame

        return SendGame(
            chat_id=self.chat.id,
            game_short_name=game_short_name,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_invoice(
        self,
        title: str,
        description: str,
        payload: str,
        currency: str,
        prices: list[LabeledPrice],
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        provider_token: str | None = None,
        max_tip_amount: int | None = None,
        suggested_tip_amounts: list[int] | None = None,
        start_parameter: str | None = None,
        provider_data: str | None = None,
        photo_url: str | None = None,
        photo_size: int | None = None,
        photo_width: int | None = None,
        photo_height: int | None = None,
        need_name: bool | None = None,
        need_phone_number: bool | None = None,
        need_email: bool | None = None,
        need_shipping_address: bool | None = None,
        send_phone_number_to_provider: bool | None = None,
        send_email_to_provider: bool | None = None,
        is_flexible: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendInvoice:
        """
        Shortcut for method :class:`aiogram.methods.send_invoice.SendInvoice`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send invoices. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendinvoice

        :param title: Product name, 1-32 characters
        :param description: Product description, 1-255 characters
        :param payload: Bot-defined invoice payload, 1-128 bytes. This will not be displayed to the user, use it for your internal processes
        :param currency: Three-letter ISO 4217 currency code, see `more on currencies <https://core.telegram.org/bots/payments#supported-currencies>`_. Pass 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param prices: Price breakdown, a JSON-serialized list of components (e.g. product price, tax, discount, delivery cost, delivery tax, bonus, etc.). Must contain exactly one item for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param provider_token: Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param max_tip_amount: The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param suggested_tip_amounts: A JSON-serialized array of suggested amounts of tips in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*
        :param start_parameter: Unique deep-linking parameter. If left empty, **forwarded copies** of the sent message will have a *Pay* button, allowing multiple users to pay directly from the forwarded message, using the same invoice. If non-empty, forwarded copies of the sent message will have a *URL* button with a deep link to the bot (instead of a *Pay* button), with the value used as the start parameter
        :param provider_data: JSON-serialized data about the invoice, which will be shared with the payment provider. A detailed description of required fields should be provided by the payment provider
        :param photo_url: URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service. People like it better when they see what they are paying for
        :param photo_size: Photo size in bytes
        :param photo_width: Photo width
        :param photo_height: Photo height
        :param need_name: Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_phone_number: Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_email: Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_shipping_address: Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param send_phone_number_to_provider: Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param send_email_to_provider: Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param is_flexible: Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Pay :code:`total price`' button will be shown. If not empty, the first button must be a Pay button
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_invoice.SendInvoice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendInvoice

        return SendInvoice(
            chat_id=self.chat.id,
            title=title,
            description=description,
            payload=payload,
            currency=currency,
            prices=prices,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            provider_token=provider_token,
            max_tip_amount=max_tip_amount,
            suggested_tip_amounts=suggested_tip_amounts,
            start_parameter=start_parameter,
            provider_data=provider_data,
            photo_url=photo_url,
            photo_size=photo_size,
            photo_width=photo_width,
            photo_height=photo_height,
            need_name=need_name,
            need_phone_number=need_phone_number,
            need_email=need_email,
            need_shipping_address=need_shipping_address,
            send_phone_number_to_provider=send_phone_number_to_provider,
            send_email_to_provider=send_email_to_provider,
            is_flexible=is_flexible,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_location(
        self,
        latitude: float,
        longitude: float,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        horizontal_accuracy: float | None = None,
        live_period: int | None = None,
        heading: int | None = None,
        proximity_alert_radius: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendLocation:
        """
        Shortcut for method :class:`aiogram.methods.send_location.SendLocation`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send point on the map. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendlocation

        :param latitude: Latitude of the location
        :param longitude: Longitude of the location
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param horizontal_accuracy: The radius of uncertainty for the location, measured in meters; 0-1500
        :param live_period: Period in seconds during which the location will be updated (see `Live Locations <https://telegram.org/blog/live-locations>`_, should be between 60 and 86400, or 0x7FFFFFFF for live locations that can be edited indefinitely
        :param heading: For live locations, a direction in which the user is moving, in degrees. Must be between 1 and 360 if specified
        :param proximity_alert_radius: For live locations, a maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_location.SendLocation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendLocation

        return SendLocation(
            chat_id=self.chat.id,
            latitude=latitude,
            longitude=longitude,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            horizontal_accuracy=horizontal_accuracy,
            live_period=live_period,
            heading=heading,
            proximity_alert_radius=proximity_alert_radius,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_media_group(
        self,
        media: list[MediaUnion],
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendMediaGroup:
        """
        Shortcut for method :class:`aiogram.methods.send_media_group.SendMediaGroup`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send a group of photos, live photos, videos, documents or audios as an album. Documents and audio files can be only grouped in an album with messages of the same type. On success, an array of :class:`aiogram.types.message.Message` objects that were sent is returned.

        Source: https://core.telegram.org/bots/api#sendmediagroup

        :param media: A JSON-serialized array describing messages to be sent, must include 2-10 items
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the messages will be sent; required if the messages are sent to a direct messages chat
        :param disable_notification: Sends messages `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent messages from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the messages are a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_media_group.SendMediaGroup`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendMediaGroup

        return SendMediaGroup(
            chat_id=self.chat.id,
            media=media,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_photo(
        self,
        photo: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendPhoto:
        """
        Shortcut for method :class:`aiogram.methods.send_photo.SendPhoto`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send photos. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendphoto

        :param photo: Photo to send. Pass a file_id as String to send a photo that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a photo from the Internet, or upload a new photo using multipart/form-data. The photo must be at most 10 MB in size. The photo's width and height must not exceed 10000 in total. Width and height ratio must be at most 20. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Photo caption (may also be used when resending photos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the photo caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the photo needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_photo.SendPhoto`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPhoto

        return SendPhoto(
            chat_id=self.chat.id,
            photo=photo,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_poll(
        self,
        question: str,
        options: list[InputPollOptionUnion],
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        question_parse_mode: str | Default | None = Default("parse_mode"),
        question_entities: list[MessageEntity] | None = None,
        is_anonymous: bool | None = None,
        type: str | None = None,
        allows_multiple_answers: bool | None = None,
        allows_revoting: bool | None = None,
        shuffle_options: bool | None = None,
        allow_adding_options: bool | None = None,
        hide_results_until_closes: bool | None = None,
        members_only: bool | None = None,
        country_codes: list[str] | None = None,
        correct_option_ids: list[int] | None = None,
        explanation: str | None = None,
        explanation_parse_mode: str | Default | None = Default("parse_mode"),
        explanation_entities: list[MessageEntity] | None = None,
        explanation_media: InputPollMediaUnion | None = None,
        open_period: int | None = None,
        close_date: DateTimeUnion | None = None,
        is_closed: bool | None = None,
        description: str | None = None,
        description_parse_mode: str | Default | None = Default("parse_mode"),
        description_entities: list[MessageEntity] | None = None,
        media: InputPollMediaUnion | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        correct_option_id: int | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendPoll:
        """
        Shortcut for method :class:`aiogram.methods.send_poll.SendPoll`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send a native poll. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpoll

        :param question: Poll question, 1-300 characters
        :param options: A JSON-serialized list of 1-12 answer options
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param question_parse_mode: Mode for parsing entities in the question. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Currently, only custom emoji entities are allowed
        :param question_entities: A JSON-serialized list of special entities that appear in the poll question. It can be specified instead of *question_parse_mode*
        :param is_anonymous: :code:`True`, if the poll needs to be anonymous, defaults to :code:`True`
        :param type: Poll type, 'quiz' or 'regular', defaults to 'regular'
        :param allows_multiple_answers: Pass :code:`True`, if the poll allows multiple answers, defaults to :code:`False`
        :param allows_revoting: Pass :code:`True`, if the poll allows to change chosen answer options, defaults to :code:`False` for quizzes and to :code:`True` for regular polls
        :param shuffle_options: Pass :code:`True`, if the poll options must be shown in random order
        :param allow_adding_options: Pass :code:`True`, if answer options can be added to the poll after creation; not supported for anonymous polls and quizzes
        :param hide_results_until_closes: Pass :code:`True`, if poll results must be shown only after the poll closes
        :param members_only: Pass :code:`True`, if voting is limited to users who have been members of the chat where the poll is being sent for more than 24 hours; for channel chats only
        :param country_codes: A JSON-serialized list of 0-12 two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which users can vote in the poll; for channel chats only. Use 'FT' as a country code to allow users with anonymous numbers to vote. If omitted or empty, then users from any country can participate in the poll
        :param correct_option_ids: A JSON-serialized list of monotonically increasing 0-based identifiers of the correct answer options, required for polls in quiz mode
        :param explanation: Text that is shown when a user chooses an incorrect answer or taps on the lamp icon in a quiz-style poll, 0-200 characters with at most 2 line feeds after entities parsing
        :param explanation_parse_mode: Mode for parsing entities in the explanation. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param explanation_entities: A JSON-serialized list of special entities that appear in the poll explanation. It can be specified instead of *explanation_parse_mode*
        :param explanation_media: Media added to the quiz explanation
        :param open_period: Amount of time in seconds the poll will be active after creation, 5-2628000. Can't be used together with *close_date*
        :param close_date: Point in time (Unix timestamp) when the poll will be automatically closed. Must be at least 5 and no more than 2628000 seconds in the future. Can't be used together with *open_period*
        :param is_closed: Pass :code:`True` if the poll needs to be immediately closed. This can be useful for poll preview
        :param description: Description of the poll to be sent, 0-1024 characters after entities parsing
        :param description_parse_mode: Mode for parsing entities in the poll description. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param description_entities: A JSON-serialized list of special entities that appear in the poll description, which can be specified instead of *description_parse_mode*
        :param media: Media added to the poll description
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param correct_option_id: 0-based identifier of the correct answer option, required for polls in quiz mode
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_poll.SendPoll`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPoll

        return SendPoll(
            chat_id=self.chat.id,
            question=question,
            options=options,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            question_parse_mode=question_parse_mode,
            question_entities=question_entities,
            is_anonymous=is_anonymous,
            type=type,
            allows_multiple_answers=allows_multiple_answers,
            allows_revoting=allows_revoting,
            shuffle_options=shuffle_options,
            allow_adding_options=allow_adding_options,
            hide_results_until_closes=hide_results_until_closes,
            members_only=members_only,
            country_codes=country_codes,
            correct_option_ids=correct_option_ids,
            explanation=explanation,
            explanation_parse_mode=explanation_parse_mode,
            explanation_entities=explanation_entities,
            explanation_media=explanation_media,
            open_period=open_period,
            close_date=close_date,
            is_closed=is_closed,
            description=description,
            description_parse_mode=description_parse_mode,
            description_entities=description_entities,
            media=media,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            correct_option_id=correct_option_id,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_dice(
        self,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        emoji: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendDice:
        """
        Shortcut for method :class:`aiogram.methods.send_dice.SendDice`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send an animated emoji that will display a random value. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#senddice

        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji on which the dice throw animation is based. Currently, must be one of '🎲', '🎯', '🏀', '⚽', '🎳', or '🎰'. Dice can have values 1-6 for '🎲', '🎯' and '🎳', values 1-5 for '🏀' and '⚽', and values 1-64 for '🎰'. Defaults to '🎲'
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_dice.SendDice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendDice

        return SendDice(
            chat_id=self.chat.id,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            emoji=emoji,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_sticker(
        self,
        sticker: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        emoji: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendSticker:
        """
        Shortcut for method :class:`aiogram.methods.send_sticker.SendSticker`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send static .WEBP, `animated <https://telegram.org/blog/animated-stickers>`_ .TGS, or `video <https://telegram.org/blog/video-stickers-better-reactions>`_ .WEBM stickers. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendsticker

        :param sticker: Sticker to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a .WEBP sticker from the Internet, or upload a new .WEBP, .TGS, or .WEBM sticker using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Video and animated stickers can't be sent via an HTTP URL
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji associated with the sticker; only for just uploaded stickers
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_sticker.SendSticker`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendSticker

        return SendSticker(
            chat_id=self.chat.id,
            sticker=sticker,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            emoji=emoji,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_venue(
        self,
        latitude: float,
        longitude: float,
        title: str,
        address: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        foursquare_id: str | None = None,
        foursquare_type: str | None = None,
        google_place_id: str | None = None,
        google_place_type: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVenue:
        """
        Shortcut for method :class:`aiogram.methods.send_venue.SendVenue`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send information about a venue. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvenue

        :param latitude: Latitude of the venue
        :param longitude: Longitude of the venue
        :param title: Name of the venue
        :param address: Address of the venue
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param foursquare_id: Foursquare identifier of the venue
        :param foursquare_type: Foursquare type of the venue, if known. (For example, 'arts_entertainment/default', 'arts_entertainment/aquarium' or 'food/icecream'.)
        :param google_place_id: Google Places identifier of the venue
        :param google_place_type: Google Places type of the venue. (See `supported types <https://developers.google.com/places/web-service/supported_types>`_.)
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_venue.SendVenue`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVenue

        return SendVenue(
            chat_id=self.chat.id,
            latitude=latitude,
            longitude=longitude,
            title=title,
            address=address,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            foursquare_id=foursquare_id,
            foursquare_type=foursquare_type,
            google_place_id=google_place_id,
            google_place_type=google_place_type,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_video(
        self,
        video: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        width: int | None = None,
        height: int | None = None,
        thumbnail: InputFile | None = None,
        cover: InputFileUnion | None = None,
        start_timestamp: DateTimeUnion | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        supports_streaming: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVideo:
        """
        Shortcut for method :class:`aiogram.methods.send_video.SendVideo`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send video files, Telegram clients support MPEG4 videos (other formats may be sent as :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvideo

        :param video: Video to send. Pass a file_id as String to send a video that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a video from the Internet, or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param width: Video width
        :param height: Video height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param cover: Cover for the video in the message. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`
        :param start_timestamp: Start timestamp for the video in the message
        :param caption: Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the video needs to be covered with a spoiler animation
        :param supports_streaming: Pass :code:`True` if the uploaded video is suitable for streaming
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_video.SendVideo`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVideo

        return SendVideo(
            chat_id=self.chat.id,
            video=video,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            width=width,
            height=height,
            thumbnail=thumbnail,
            cover=cover,
            start_timestamp=start_timestamp,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            supports_streaming=supports_streaming,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_video_note(
        self,
        video_note: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        length: int | None = None,
        thumbnail: InputFile | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVideoNote:
        """
        Shortcut for method :class:`aiogram.methods.send_video_note.SendVideoNote`
        will automatically fill method attributes:

        - :code:`chat_id`

        As of `v.4.0 <https://telegram.org/blog/video-messages-and-telescope>`_, Telegram clients support rounded square MPEG4 videos of up to 1 minute long. Use this method to send video messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvideonote

        :param video_note: Video note to send. Pass a file_id as String to send a video note that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending video notes by a URL is currently unsupported
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param length: Video width and height, i.e. diameter of the video message
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_video_note.SendVideoNote`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVideoNote

        return SendVideoNote(
            chat_id=self.chat.id,
            video_note=video_note,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            length=length,
            thumbnail=thumbnail,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def answer_voice(
        self,
        voice: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        duration: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVoice:
        """
        Shortcut for method :class:`aiogram.methods.send_voice.SendVoice`
        will automatically fill method attributes:

        - :code:`chat_id`

        Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message. For this to work, your audio must be in an .OGG file encoded with OPUS, or in .MP3 format, or in .M4A format (other formats may be sent as :class:`aiogram.types.audio.Audio` or :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send voice messages of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvoice

        :param voice: Audio file to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Voice message caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the voice message caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the voice message in seconds
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_voice.SendVoice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVoice

        return SendVoice(
            chat_id=self.chat.id,
            voice=voice,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            duration=duration,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)
