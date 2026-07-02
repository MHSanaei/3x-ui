from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from ..client.default import Default
from ..types import (
    ChatIdUnion,
    DateTimeUnion,
    InputPollMediaUnion,
    InputPollOptionUnion,
    Message,
    MessageEntity,
    ReplyMarkupUnion,
    ReplyParameters,
)
from .base import TelegramMethod


class SendPoll(TelegramMethod[Message]):
    """
    Use this method to send a native poll. On success, the sent :class:`aiogram.types.message.Message` is returned.

    Source: https://core.telegram.org/bots/api#sendpoll
    """

    __returning__ = Message
    __api_method__ = "sendPoll"

    chat_id: ChatIdUnion
    """Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`. Polls can't be sent to channel direct messages chats"""
    question: str
    """Poll question, 1-300 characters"""
    options: list[InputPollOptionUnion]
    """A JSON-serialized list of 1-12 answer options"""
    business_connection_id: str | None = None
    """Unique identifier of the business connection on behalf of which the message will be sent"""
    message_thread_id: int | None = None
    """Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only"""
    question_parse_mode: str | Default | None = Default("parse_mode")
    """Mode for parsing entities in the question. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Currently, only custom emoji entities are allowed"""
    question_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the poll question. It can be specified instead of *question_parse_mode*"""
    is_anonymous: bool | None = None
    """:code:`True`, if the poll needs to be anonymous, defaults to :code:`True`"""
    type: str | None = None
    """Poll type, 'quiz' or 'regular', defaults to 'regular'"""
    allows_multiple_answers: bool | None = None
    """Pass :code:`True`, if the poll allows multiple answers, defaults to :code:`False`"""
    allows_revoting: bool | None = None
    """Pass :code:`True`, if the poll allows to change chosen answer options, defaults to :code:`False` for quizzes and to :code:`True` for regular polls"""
    shuffle_options: bool | None = None
    """Pass :code:`True`, if the poll options must be shown in random order"""
    allow_adding_options: bool | None = None
    """Pass :code:`True`, if answer options can be added to the poll after creation; not supported for anonymous polls and quizzes"""
    hide_results_until_closes: bool | None = None
    """Pass :code:`True`, if poll results must be shown only after the poll closes"""
    members_only: bool | None = None
    """Pass :code:`True`, if voting is limited to users who have been members of the chat where the poll is being sent for more than 24 hours; for channel chats only"""
    country_codes: list[str] | None = None
    """A JSON-serialized list of 0-12 two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which users can vote in the poll; for channel chats only. Use 'FT' as a country code to allow users with anonymous numbers to vote. If omitted or empty, then users from any country can participate in the poll"""
    correct_option_ids: list[int] | None = None
    """A JSON-serialized list of monotonically increasing 0-based identifiers of the correct answer options, required for polls in quiz mode"""
    explanation: str | None = None
    """Text that is shown when a user chooses an incorrect answer or taps on the lamp icon in a quiz-style poll, 0-200 characters with at most 2 line feeds after entities parsing"""
    explanation_parse_mode: str | Default | None = Default("parse_mode")
    """Mode for parsing entities in the explanation. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    explanation_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the poll explanation. It can be specified instead of *explanation_parse_mode*"""
    explanation_media: InputPollMediaUnion | None = None
    """Media added to the quiz explanation"""
    open_period: int | None = None
    """Amount of time in seconds the poll will be active after creation, 5-2628000. Can't be used together with *close_date*"""
    close_date: DateTimeUnion | None = None
    """Point in time (Unix timestamp) when the poll will be automatically closed. Must be at least 5 and no more than 2628000 seconds in the future. Can't be used together with *open_period*"""
    is_closed: bool | None = None
    """Pass :code:`True` if the poll needs to be immediately closed. This can be useful for poll preview"""
    description: str | None = None
    """Description of the poll to be sent, 0-1024 characters after entities parsing"""
    description_parse_mode: str | Default | None = Default("parse_mode")
    """Mode for parsing entities in the poll description. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    description_entities: list[MessageEntity] | None = None
    """A JSON-serialized list of special entities that appear in the poll description, which can be specified instead of *description_parse_mode*"""
    media: InputPollMediaUnion | None = None
    """Media added to the poll description"""
    disable_notification: bool | None = None
    """Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound"""
    protect_content: bool | Default | None = Default("protect_content")
    """Protects the contents of the sent message from forwarding and saving"""
    allow_paid_broadcast: bool | None = None
    """Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance"""
    message_effect_id: str | None = None
    """Unique identifier of the message effect to be added to the message; for private chats only"""
    reply_parameters: ReplyParameters | None = None
    """Description of the message to reply to"""
    reply_markup: ReplyMarkupUnion | None = None
    """Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user"""
    allow_sending_without_reply: bool | None = Field(None, json_schema_extra={"deprecated": True})
    """Pass :code:`True` if the message should be sent even if the specified replied-to message is not found

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    correct_option_id: int | None = Field(None, json_schema_extra={"deprecated": True})
    """0-based identifier of the correct answer option, required for polls in quiz mode

.. deprecated:: API:9.6
   https://core.telegram.org/bots/api-changelog#april-3-2026"""
    reply_to_message_id: int | None = Field(None, json_schema_extra={"deprecated": True})
    """If the message is a reply, ID of the original message

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            chat_id: ChatIdUnion,
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
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                chat_id=chat_id,
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
                **__pydantic_kwargs,
            )
