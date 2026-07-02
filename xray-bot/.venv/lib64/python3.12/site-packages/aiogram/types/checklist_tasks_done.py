from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .message import Message


class ChecklistTasksDone(TelegramObject):
    """
    Describes a service message about checklist tasks marked as done or not done.

    Source: https://core.telegram.org/bots/api#checklisttasksdone
    """

    checklist_message: Message | None = None
    """*Optional*. Message containing the checklist whose tasks were marked as done or not done. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""
    marked_as_done_task_ids: list[int] | None = None
    """*Optional*. Identifiers of the tasks that were marked as done"""
    marked_as_not_done_task_ids: list[int] | None = None
    """*Optional*. Identifiers of the tasks that were marked as not done"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            checklist_message: Message | None = None,
            marked_as_done_task_ids: list[int] | None = None,
            marked_as_not_done_task_ids: list[int] | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                checklist_message=checklist_message,
                marked_as_done_task_ids=marked_as_done_task_ids,
                marked_as_not_done_task_ids=marked_as_not_done_task_ids,
                **__pydantic_kwargs,
            )
