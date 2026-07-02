from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .checklist_task import ChecklistTask
    from .message_entity import MessageEntity


class Checklist(TelegramObject):
    """
    Describes a checklist.

    Source: https://core.telegram.org/bots/api#checklist
    """

    title: str
    """Title of the checklist"""
    tasks: list[ChecklistTask]
    """List of tasks in the checklist"""
    title_entities: list[MessageEntity] | None = None
    """*Optional*. Special entities that appear in the checklist title"""
    others_can_add_tasks: bool | None = None
    """*Optional*. :code:`True`, if users other than the creator of the list can add tasks to the list"""
    others_can_mark_tasks_as_done: bool | None = None
    """*Optional*. :code:`True`, if users other than the creator of the list can mark tasks as done or not done"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            title: str,
            tasks: list[ChecklistTask],
            title_entities: list[MessageEntity] | None = None,
            others_can_add_tasks: bool | None = None,
            others_can_mark_tasks_as_done: bool | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                title=title,
                tasks=tasks,
                title_entities=title_entities,
                others_can_add_tasks=others_can_add_tasks,
                others_can_mark_tasks_as_done=others_can_mark_tasks_as_done,
                **__pydantic_kwargs,
            )
