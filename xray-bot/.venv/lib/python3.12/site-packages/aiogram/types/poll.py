from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from .base import TelegramObject
from .custom import DateTime

if TYPE_CHECKING:
    from .message_entity import MessageEntity
    from .poll_media import PollMedia
    from .poll_option import PollOption


class Poll(TelegramObject):
    """
    This object contains information about a poll.

    Source: https://core.telegram.org/bots/api#poll
    """

    id: str
    """Unique poll identifier"""
    question: str
    """Poll question, 1-300 characters"""
    options: list[PollOption]
    """List of poll options"""
    total_voter_count: int
    """Total number of users that voted in the poll"""
    is_closed: bool
    """:code:`True`, if the poll is closed"""
    is_anonymous: bool
    """:code:`True`, if the poll is anonymous"""
    type: str
    """Poll type, currently can be 'regular' or 'quiz'"""
    allows_multiple_answers: bool
    """:code:`True`, if the poll allows multiple answers"""
    allows_revoting: bool
    """:code:`True`, if the poll allows to change the chosen answer options"""
    members_only: bool
    """:code:`True` if voting is limited to users who have been members of the chat where the poll was originally sent for more than 24 hours"""
    question_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the *question*. Currently, only custom emoji entities are allowed in poll questions"""
    country_codes: list[str] | None = None
    """*Optional*. A list of two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which users can vote in the poll. The country code 'FT' is used for users with anonymous numbers. If omitted, then users from any country can participate in the poll"""
    correct_option_ids: list[int] | None = None
    """*Optional*. Array of 0-based identifiers of the correct answer options. Available only for polls in quiz mode which are closed or were sent (not forwarded) by the bot or to the private chat with the bot"""
    explanation: str | None = None
    """*Optional*. Text that is shown when a user chooses an incorrect answer or taps on the lamp icon in a quiz-style poll, 0-200 characters"""
    explanation_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities like usernames, URLs, bot commands, etc. that appear in the *explanation*"""
    explanation_media: PollMedia | None = None
    """*Optional*. Media added to the quiz explanation"""
    open_period: int | None = None
    """*Optional*. Amount of time in seconds the poll will be active after creation"""
    close_date: DateTime | None = None
    """*Optional*. Point in time (Unix timestamp) when the poll will be automatically closed"""
    description: str | None = None
    """*Optional*. Description of the poll; for polls inside the :class:`aiogram.types.message.Message` object only"""
    description_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities like usernames, URLs, bot commands, etc. that appear in the description"""
    media: PollMedia | None = None
    """*Optional*. Media added to the poll description; for polls inside the :class:`aiogram.types.message.Message` object only"""
    correct_option_id: int | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. 0-based identifier of the correct answer option. Available only for polls in the quiz mode, which are closed, or was sent (not forwarded) by the bot or to the private chat with the bot

.. deprecated:: API:9.6
   https://core.telegram.org/bots/api-changelog#april-3-2026"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            id: str,
            question: str,
            options: list[PollOption],
            total_voter_count: int,
            is_closed: bool,
            is_anonymous: bool,
            type: str,
            allows_multiple_answers: bool,
            allows_revoting: bool,
            members_only: bool,
            question_entities: list[MessageEntity] | None = None,
            country_codes: list[str] | None = None,
            correct_option_ids: list[int] | None = None,
            explanation: str | None = None,
            explanation_entities: list[MessageEntity] | None = None,
            explanation_media: PollMedia | None = None,
            open_period: int | None = None,
            close_date: DateTime | None = None,
            description: str | None = None,
            description_entities: list[MessageEntity] | None = None,
            media: PollMedia | None = None,
            correct_option_id: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                id=id,
                question=question,
                options=options,
                total_voter_count=total_voter_count,
                is_closed=is_closed,
                is_anonymous=is_anonymous,
                type=type,
                allows_multiple_answers=allows_multiple_answers,
                allows_revoting=allows_revoting,
                members_only=members_only,
                question_entities=question_entities,
                country_codes=country_codes,
                correct_option_ids=correct_option_ids,
                explanation=explanation,
                explanation_entities=explanation_entities,
                explanation_media=explanation_media,
                open_period=open_period,
                close_date=close_date,
                description=description,
                description_entities=description_entities,
                media=media,
                correct_option_id=correct_option_id,
                **__pydantic_kwargs,
            )
