from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .input_checklist_task import InputChecklistTask
    from .message_entity import MessageEntity


class InputChecklist(TelegramObject):
    """
    Describes a checklist to create.

    Source: https://core.telegram.org/bots/api#inputchecklist
    """

    title: str
    """Title of the checklist; 1-255 characters after entities parsing"""
    tasks: list[InputChecklistTask]
    """List of 1-30 tasks in the checklist"""
    parse_mode: str | None = None
    """*Optional*. Mode for parsing entities in the title. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details"""
    title_entities: list[MessageEntity] | None = None
    """*Optional*. List of special entities that appear in the title, which can be specified instead of parse_mode. Currently, only *bold*, *italic*, *underline*, *strikethrough*, *spoiler*, *custom_emoji*, and *date_time* entities are allowed"""
    others_can_add_tasks: bool | None = None
    """*Optional*. Pass :code:`True` if other users can add tasks to the checklist"""
    others_can_mark_tasks_as_done: bool | None = None
    """*Optional*. Pass :code:`True` if other users can mark tasks as done or not done in the checklist"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            title: str,
            tasks: list[InputChecklistTask],
            parse_mode: str | None = None,
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
                parse_mode=parse_mode,
                title_entities=title_entities,
                others_can_add_tasks=others_can_add_tasks,
                others_can_mark_tasks_as_done=others_can_mark_tasks_as_done,
                **__pydantic_kwargs,
            )
