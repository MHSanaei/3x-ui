from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .checklist_task import ChecklistTask
    from .message import Message


class ChecklistTasksAdded(TelegramObject):
    """
    Describes a service message about tasks added to a checklist.

    Source: https://core.telegram.org/bots/api#checklisttasksadded
    """

    tasks: list[ChecklistTask]
    """List of tasks added to the checklist"""
    checklist_message: Message | None = None
    """*Optional*. Message containing the checklist to which the tasks were added. Note that the :class:`aiogram.types.message.Message` object in this field will not contain the *reply_to_message* field even if it itself is a reply"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            tasks: list[ChecklistTask],
            checklist_message: Message | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(tasks=tasks, checklist_message=checklist_message, **__pydantic_kwargs)
