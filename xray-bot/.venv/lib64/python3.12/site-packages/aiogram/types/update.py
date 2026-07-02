from __future__ import annotations

from typing import TYPE_CHECKING, Any, cast

from ..utils.mypy_hacks import lru_cache
from .base import TelegramObject

if TYPE_CHECKING:
    from .business_connection import BusinessConnection
    from .business_messages_deleted import BusinessMessagesDeleted
    from .callback_query import CallbackQuery
    from .chat_boost_removed import ChatBoostRemoved
    from .chat_boost_updated import ChatBoostUpdated
    from .chat_join_request import ChatJoinRequest
    from .chat_member_updated import ChatMemberUpdated
    from .chosen_inline_result import ChosenInlineResult
    from .inline_query import InlineQuery
    from .managed_bot_updated import ManagedBotUpdated
    from .message import Message
    from .message_reaction_count_updated import MessageReactionCountUpdated
    from .message_reaction_updated import MessageReactionUpdated
    from .paid_media_purchased import PaidMediaPurchased
    from .poll import Poll
    from .poll_answer import PollAnswer
    from .pre_checkout_query import PreCheckoutQuery
    from .shipping_query import ShippingQuery


class Update(TelegramObject):
    """
    This `object <https://core.telegram.org/bots/api#available-types>`_ represents an incoming update.

    At most **one** of the optional fields can be present in any given update.

    Source: https://core.telegram.org/bots/api#update
    """

    update_id: int
    """The update's unique identifier. Update identifiers start from a certain positive number and increase sequentially. This identifier becomes especially handy if you're using `webhooks <https://core.telegram.org/bots/api#setwebhook>`_, since it allows you to ignore repeated updates or to restore the correct update sequence, should they get out of order. If there are no new updates for at least a week, then identifier of the next update will be chosen randomly instead of sequentially"""
    message: Message | None = None
    """*Optional*. New incoming message of any kind - text, photo, sticker, etc"""
    edited_message: Message | None = None
    """*Optional*. New version of a message that is known to the bot and was edited. This update may at times be triggered by changes to message fields that are either unavailable or not actively used by your bot"""
    channel_post: Message | None = None
    """*Optional*. New incoming channel post of any kind - text, photo, sticker, etc"""
    edited_channel_post: Message | None = None
    """*Optional*. New version of a channel post that is known to the bot and was edited. This update may at times be triggered by changes to message fields that are either unavailable or not actively used by your bot"""
    business_connection: BusinessConnection | None = None
    """*Optional*. The bot was connected to or disconnected from a business account, or a user edited an existing connection with the bot"""
    business_message: Message | None = None
    """*Optional*. New message from a connected business account"""
    edited_business_message: Message | None = None
    """*Optional*. New version of a message from a connected business account"""
    deleted_business_messages: BusinessMessagesDeleted | None = None
    """*Optional*. Messages were deleted from a connected business account"""
    guest_message: Message | None = None
    """*Optional*. New guest message. The bot can use the field *Message.guest_query_id* and the method :class:`aiogram.methods.answer_guest_query.AnswerGuestQuery` to send a message in response"""
    message_reaction: MessageReactionUpdated | None = None
    """*Optional*. A reaction to a message was changed by a user. The bot must be an administrator in the chat and must explicitly specify :code:`"message_reaction"` in the list of *allowed_updates* to receive these updates. The update isn't received for reactions set by bots"""
    message_reaction_count: MessageReactionCountUpdated | None = None
    """*Optional*. Reactions to a message with anonymous reactions were changed. The bot must be an administrator in the chat and must explicitly specify :code:`"message_reaction_count"` in the list of *allowed_updates* to receive these updates. The updates are grouped and can be sent with delay up to a few minutes"""
    inline_query: InlineQuery | None = None
    """*Optional*. New incoming `inline <https://core.telegram.org/bots/api#inline-mode>`_ query"""
    chosen_inline_result: ChosenInlineResult | None = None
    """*Optional*. The result of an `inline <https://core.telegram.org/bots/api#inline-mode>`_ query that was chosen by a user and sent to their chat partner. Please see our documentation on the `feedback collecting <https://core.telegram.org/bots/inline#collecting-feedback>`_ for details on how to enable these updates for your bot"""
    callback_query: CallbackQuery | None = None
    """*Optional*. New incoming callback query"""
    shipping_query: ShippingQuery | None = None
    """*Optional*. New incoming shipping query. Only for invoices with flexible price"""
    pre_checkout_query: PreCheckoutQuery | None = None
    """*Optional*. New incoming pre-checkout query. Contains full information about checkout"""
    purchased_paid_media: PaidMediaPurchased | None = None
    """*Optional*. A user purchased paid media with a non-empty payload sent by the bot in a non-channel chat"""
    poll: Poll | None = None
    """*Optional*. New poll state. Bots receive only updates about manually stopped polls and polls, which are sent by the bot"""
    poll_answer: PollAnswer | None = None
    """*Optional*. A user changed their answer in a non-anonymous poll. Bots receive new votes only in polls that were sent by the bot itself"""
    my_chat_member: ChatMemberUpdated | None = None
    """*Optional*. The bot's chat member status was updated in a chat. For private chats, this update is received only when the bot is blocked or unblocked by the user"""
    chat_member: ChatMemberUpdated | None = None
    """*Optional*. A chat member's status was updated in a chat. The bot must be an administrator in the chat and must explicitly specify :code:`"chat_member"` in the list of *allowed_updates* to receive these updates"""
    chat_join_request: ChatJoinRequest | None = None
    """*Optional*. A request to join the chat has been sent. The bot must have the *can_invite_users* administrator right in the chat to receive these updates"""
    chat_boost: ChatBoostUpdated | None = None
    """*Optional*. A chat boost was added or changed. The bot must be an administrator in the chat to receive these updates"""
    removed_chat_boost: ChatBoostRemoved | None = None
    """*Optional*. A boost was removed from a chat. The bot must be an administrator in the chat to receive these updates"""
    managed_bot: ManagedBotUpdated | None = None
    """*Optional*. A new bot was created to be managed by the bot, or token or owner of a managed bot was changed"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            update_id: int,
            message: Message | None = None,
            edited_message: Message | None = None,
            channel_post: Message | None = None,
            edited_channel_post: Message | None = None,
            business_connection: BusinessConnection | None = None,
            business_message: Message | None = None,
            edited_business_message: Message | None = None,
            deleted_business_messages: BusinessMessagesDeleted | None = None,
            guest_message: Message | None = None,
            message_reaction: MessageReactionUpdated | None = None,
            message_reaction_count: MessageReactionCountUpdated | None = None,
            inline_query: InlineQuery | None = None,
            chosen_inline_result: ChosenInlineResult | None = None,
            callback_query: CallbackQuery | None = None,
            shipping_query: ShippingQuery | None = None,
            pre_checkout_query: PreCheckoutQuery | None = None,
            purchased_paid_media: PaidMediaPurchased | None = None,
            poll: Poll | None = None,
            poll_answer: PollAnswer | None = None,
            my_chat_member: ChatMemberUpdated | None = None,
            chat_member: ChatMemberUpdated | None = None,
            chat_join_request: ChatJoinRequest | None = None,
            chat_boost: ChatBoostUpdated | None = None,
            removed_chat_boost: ChatBoostRemoved | None = None,
            managed_bot: ManagedBotUpdated | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                update_id=update_id,
                message=message,
                edited_message=edited_message,
                channel_post=channel_post,
                edited_channel_post=edited_channel_post,
                business_connection=business_connection,
                business_message=business_message,
                edited_business_message=edited_business_message,
                deleted_business_messages=deleted_business_messages,
                guest_message=guest_message,
                message_reaction=message_reaction,
                message_reaction_count=message_reaction_count,
                inline_query=inline_query,
                chosen_inline_result=chosen_inline_result,
                callback_query=callback_query,
                shipping_query=shipping_query,
                pre_checkout_query=pre_checkout_query,
                purchased_paid_media=purchased_paid_media,
                poll=poll,
                poll_answer=poll_answer,
                my_chat_member=my_chat_member,
                chat_member=chat_member,
                chat_join_request=chat_join_request,
                chat_boost=chat_boost,
                removed_chat_boost=removed_chat_boost,
                managed_bot=managed_bot,
                **__pydantic_kwargs,
            )

    def __hash__(self) -> int:
        return hash((type(self), self.update_id))

    @property
    @lru_cache()
    def event_type(self) -> str:
        """
        Detect update type
        If update type is unknown, raise UpdateTypeLookupError

        :return:
        """
        if self.message:
            return "message"
        if self.edited_message:
            return "edited_message"
        if self.channel_post:
            return "channel_post"
        if self.edited_channel_post:
            return "edited_channel_post"
        if self.inline_query:
            return "inline_query"
        if self.chosen_inline_result:
            return "chosen_inline_result"
        if self.callback_query:
            return "callback_query"
        if self.shipping_query:
            return "shipping_query"
        if self.pre_checkout_query:
            return "pre_checkout_query"
        if self.poll:
            return "poll"
        if self.poll_answer:
            return "poll_answer"
        if self.my_chat_member:
            return "my_chat_member"
        if self.chat_member:
            return "chat_member"
        if self.chat_join_request:
            return "chat_join_request"
        if self.message_reaction:
            return "message_reaction"
        if self.message_reaction_count:
            return "message_reaction_count"
        if self.chat_boost:
            return "chat_boost"
        if self.removed_chat_boost:
            return "removed_chat_boost"
        if self.deleted_business_messages:
            return "deleted_business_messages"
        if self.business_connection:
            return "business_connection"
        if self.edited_business_message:
            return "edited_business_message"
        if self.business_message:
            return "business_message"
        if self.purchased_paid_media:
            return "purchased_paid_media"
        if self.guest_message:
            return "guest_message"
        if self.managed_bot:
            return "managed_bot"

        raise UpdateTypeLookupError("Update does not contain any known event type.")

    @property
    def event(self) -> TelegramObject:
        return cast(TelegramObject, getattr(self, self.event_type))


class UpdateTypeLookupError(LookupError):
    """Update does not contain any known event type."""
