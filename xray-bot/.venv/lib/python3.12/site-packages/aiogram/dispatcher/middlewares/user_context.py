from collections.abc import Awaitable, Callable
from dataclasses import dataclass
from typing import Any

from aiogram.dispatcher.middlewares.base import BaseMiddleware
from aiogram.types import (
    Chat,
    ChatBoostSourcePremium,
    InaccessibleMessage,
    TelegramObject,
    Update,
    User,
)

EVENT_CONTEXT_KEY = "event_context"

EVENT_FROM_USER_KEY = "event_from_user"
EVENT_CHAT_KEY = "event_chat"
EVENT_THREAD_ID_KEY = "event_thread_id"


@dataclass(frozen=True)
class EventContext:
    chat: Chat | None = None
    user: User | None = None
    thread_id: int | None = None
    business_connection_id: str | None = None

    @property
    def user_id(self) -> int | None:
        return self.user.id if self.user else None

    @property
    def chat_id(self) -> int | None:
        return self.chat.id if self.chat else None


class UserContextMiddleware(BaseMiddleware):
    async def __call__(
        self,
        handler: Callable[[TelegramObject, dict[str, Any]], Awaitable[Any]],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        if not isinstance(event, Update):
            msg = "UserContextMiddleware got an unexpected event type!"
            raise RuntimeError(msg)
        event_context = data[EVENT_CONTEXT_KEY] = self.resolve_event_context(event=event)

        # Backward compatibility
        if event_context.user is not None:
            data[EVENT_FROM_USER_KEY] = event_context.user
        if event_context.chat is not None:
            data[EVENT_CHAT_KEY] = event_context.chat
        if event_context.thread_id is not None:
            data[EVENT_THREAD_ID_KEY] = event_context.thread_id

        return await handler(event, data)

    @classmethod
    def resolve_event_context(cls, event: Update) -> EventContext:
        """
        Resolve chat and user instance from Update object
        """
        if event.message:
            return EventContext(
                chat=event.message.chat,
                user=event.message.from_user,
                thread_id=(
                    event.message.message_thread_id if event.message.is_topic_message else None
                ),
            )
        if event.edited_message:
            return EventContext(
                chat=event.edited_message.chat,
                user=event.edited_message.from_user,
                thread_id=(
                    event.edited_message.message_thread_id
                    if event.edited_message.is_topic_message
                    else None
                ),
            )
        if event.channel_post:
            return EventContext(chat=event.channel_post.chat)
        if event.edited_channel_post:
            return EventContext(chat=event.edited_channel_post.chat)
        if event.inline_query:
            return EventContext(user=event.inline_query.from_user)
        if event.chosen_inline_result:
            return EventContext(user=event.chosen_inline_result.from_user)
        if event.callback_query:
            callback_query_message = event.callback_query.message
            if callback_query_message:
                return EventContext(
                    chat=callback_query_message.chat,
                    user=event.callback_query.from_user,
                    thread_id=(
                        callback_query_message.message_thread_id
                        if not isinstance(callback_query_message, InaccessibleMessage)
                        and callback_query_message.is_topic_message
                        else None
                    ),
                    business_connection_id=(
                        callback_query_message.business_connection_id
                        if not isinstance(callback_query_message, InaccessibleMessage)
                        else None
                    ),
                )
            return EventContext(user=event.callback_query.from_user)
        if event.shipping_query:
            return EventContext(user=event.shipping_query.from_user)
        if event.pre_checkout_query:
            return EventContext(user=event.pre_checkout_query.from_user)
        if event.poll_answer:
            return EventContext(
                chat=event.poll_answer.voter_chat,
                user=event.poll_answer.user,
            )
        if event.my_chat_member:
            return EventContext(
                chat=event.my_chat_member.chat,
                user=event.my_chat_member.from_user,
            )
        if event.chat_member:
            return EventContext(chat=event.chat_member.chat, user=event.chat_member.from_user)
        if event.chat_join_request:
            return EventContext(
                chat=event.chat_join_request.chat,
                user=event.chat_join_request.from_user,
            )
        if event.message_reaction:
            return EventContext(
                chat=event.message_reaction.chat,
                user=event.message_reaction.user,
            )
        if event.message_reaction_count:
            return EventContext(chat=event.message_reaction_count.chat)
        if event.chat_boost:
            # We only check the premium source, because only it has a sender user,
            # other sources have a user, but it is not the sender, but the recipient
            if isinstance(event.chat_boost.boost.source, ChatBoostSourcePremium):
                return EventContext(
                    chat=event.chat_boost.chat,
                    user=event.chat_boost.boost.source.user,
                )

            return EventContext(chat=event.chat_boost.chat)
        if event.removed_chat_boost:
            return EventContext(chat=event.removed_chat_boost.chat)
        if event.deleted_business_messages:
            return EventContext(
                chat=event.deleted_business_messages.chat,
                business_connection_id=event.deleted_business_messages.business_connection_id,
            )
        if event.business_connection:
            return EventContext(
                user=event.business_connection.user,
                business_connection_id=event.business_connection.id,
            )
        if event.business_message:
            return EventContext(
                chat=event.business_message.chat,
                user=event.business_message.from_user,
                thread_id=(
                    event.business_message.message_thread_id
                    if event.business_message.is_topic_message
                    else None
                ),
                business_connection_id=event.business_message.business_connection_id,
            )
        if event.edited_business_message:
            return EventContext(
                chat=event.edited_business_message.chat,
                user=event.edited_business_message.from_user,
                thread_id=(
                    event.edited_business_message.message_thread_id
                    if event.edited_business_message.is_topic_message
                    else None
                ),
                business_connection_id=event.edited_business_message.business_connection_id,
            )
        if event.purchased_paid_media:
            return EventContext(
                user=event.purchased_paid_media.from_user,
            )
        if event.managed_bot:
            return EventContext(user=event.managed_bot.user)
        if event.guest_message:
            return EventContext(
                chat=event.guest_message.chat,
                user=event.guest_message.from_user,
                thread_id=(
                    event.guest_message.message_thread_id
                    if event.guest_message.is_topic_message
                    else None
                ),
            )
        return EventContext()
