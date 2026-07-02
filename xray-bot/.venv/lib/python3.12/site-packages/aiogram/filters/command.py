from __future__ import annotations

import re
from collections.abc import Iterable, Sequence
from dataclasses import dataclass, field, replace
from re import Match, Pattern
from typing import TYPE_CHECKING, Any, cast

from aiogram.filters.base import Filter
from aiogram.types import BotCommand, Message
from aiogram.utils.deep_linking import decode_payload

if TYPE_CHECKING:
    from magic_filter import MagicFilter

    from aiogram import Bot

CommandPatternType = str | re.Pattern[str] | BotCommand


class CommandException(Exception):
    pass


class Command(Filter):
    """
    This filter can be helpful for handling commands from the text messages.

    Works only with :class:`aiogram.types.message.Message` events which have the :code:`text`.
    """

    __slots__ = (
        "commands",
        "ignore_case",
        "ignore_mention",
        "magic",
        "prefix",
    )

    def __init__(
        self,
        *values: CommandPatternType,
        commands: Sequence[CommandPatternType] | CommandPatternType | None = None,
        prefix: str = "/",
        ignore_case: bool = False,
        ignore_mention: bool = False,
        magic: MagicFilter | None = None,
    ):
        """
        List of commands (string or compiled regexp patterns)

        :param prefix: Prefix for command.
            Prefix is always a single char but here you can pass all of allowed prefixes,
            for example: :code:`"/!"` will work with commands prefixed
            by :code:`"/"` or :code:`"!"`.
        :param ignore_case: Ignore case (Does not work with regexp, use flags instead)
        :param ignore_mention: Ignore bot mention. By default,
            bot can not handle commands intended for other bots
        :param magic: Validate command object via Magic filter after all checks done
        """
        if commands is None:
            commands = []
        if isinstance(commands, (str, re.Pattern, BotCommand)):
            commands = [commands]

        if not isinstance(commands, Iterable):
            msg = (
                "Command filter only supports str, re.Pattern, BotCommand object or their Iterable"
            )
            raise ValueError(msg)

        items = []
        for command in (*values, *commands):
            if isinstance(command, BotCommand):
                command = command.command
            if not isinstance(command, (str, re.Pattern)):
                msg = (
                    "Command filter only supports str, re.Pattern, BotCommand object"
                    " or their Iterable"
                )
                raise ValueError(msg)
            if ignore_case and isinstance(command, str):
                command = command.casefold()
            items.append(command)

        if not items:
            msg = "At least one command should be specified"
            raise ValueError(msg)

        self.commands = tuple(items)
        self.prefix = prefix
        self.ignore_case = ignore_case
        self.ignore_mention = ignore_mention
        self.magic = magic

    def __str__(self) -> str:
        return self._signature_to_string(
            *self.commands,
            prefix=self.prefix,
            ignore_case=self.ignore_case,
            ignore_mention=self.ignore_mention,
            magic=self.magic,
        )

    def update_handler_flags(self, flags: dict[str, Any]) -> None:
        commands = flags.setdefault("commands", [])
        commands.append(self)

    async def __call__(self, message: Message, bot: Bot) -> bool | dict[str, Any]:
        if not isinstance(message, Message):
            return False

        text = message.text or message.caption
        if not text:
            return False

        try:
            command = await self.parse_command(text=text, bot=bot)
        except CommandException:
            return False
        result = {"command": command}
        if command.magic_result and isinstance(command.magic_result, dict):
            result.update(command.magic_result)
        return result

    @classmethod
    def extract_command(cls, text: str) -> CommandObject:
        # First step: separate command with arguments
        # "/command@mention arg1 arg2" -> "/command@mention", ["arg1 arg2"]
        try:
            full_command, *args = text.split(maxsplit=1)
        except ValueError as e:
            msg = "not enough values to unpack"
            raise CommandException(msg) from e

        # Separate command into valuable parts
        # "/command@mention" -> "/", ("command", "@", "mention")
        prefix, (command, _, mention) = full_command[0], full_command[1:].partition("@")
        return CommandObject(
            prefix=prefix,
            command=command,
            mention=mention or None,
            args=args[0] if args else None,
        )

    def validate_prefix(self, command: CommandObject) -> None:
        if command.prefix not in self.prefix:
            msg = "Invalid command prefix"
            raise CommandException(msg)

    async def validate_mention(self, bot: Bot, command: CommandObject) -> None:
        if command.mention and not self.ignore_mention:
            me = await bot.me()
            if me.username and command.mention.lower() != me.username.lower():
                msg = "Mention did not match"
                raise CommandException(msg)

    def validate_command(self, command: CommandObject) -> CommandObject:
        for allowed_command in cast(Sequence[CommandPatternType], self.commands):
            # Command can be presented as regexp pattern or raw string
            # then need to validate that in different ways
            if isinstance(allowed_command, Pattern):  # Regexp
                result = allowed_command.match(command.command)
                if result:
                    return replace(command, regexp_match=result)

            command_name = command.command
            if self.ignore_case:
                command_name = command_name.casefold()

            if command_name == allowed_command:  # String
                return command
        msg = "Command did not match pattern"
        raise CommandException(msg)

    async def parse_command(self, text: str, bot: Bot) -> CommandObject:
        """
        Extract command from the text and validate

        :param text:
        :param bot:
        :return:
        """
        command = self.extract_command(text)
        self.validate_prefix(command=command)
        await self.validate_mention(bot=bot, command=command)
        command = self.validate_command(command)
        command = self.do_magic(command=command)
        return cast(CommandObject, command)

    def do_magic(self, command: CommandObject) -> Any:
        if self.magic is None:
            return command
        result = self.magic.resolve(command)
        if not result:
            msg = "Rejected via magic filter"
            raise CommandException(msg)
        return replace(command, magic_result=result)


@dataclass(frozen=True)
class CommandObject:
    """
    Instance of this object is always has command and it prefix.
    Can be passed as keyword argument **command** to the handler
    """

    prefix: str = "/"
    """Command prefix"""
    command: str = ""
    """Command without prefix and mention"""
    mention: str | None = None
    """Mention (if available)"""
    args: str | None = field(repr=False, default=None)
    """Command argument"""
    regexp_match: Match[str] | None = field(repr=False, default=None)
    """Will be presented match result if the command is presented as regexp in filter"""
    magic_result: Any | None = field(repr=False, default=None)

    @property
    def mentioned(self) -> bool:
        """
        This command has mention?
        """
        return bool(self.mention)

    @property
    def text(self) -> str:
        """
        Generate original text from object
        """
        line = self.prefix + self.command
        if self.mention:
            line += "@" + self.mention
        if self.args:
            line += " " + self.args
        return line


class CommandStart(Command):
    def __init__(
        self,
        deep_link: bool | None = None,
        deep_link_encoded: bool = False,
        ignore_case: bool = False,
        ignore_mention: bool = False,
        magic: MagicFilter | None = None,
    ):
        super().__init__(
            "start",
            prefix="/",
            ignore_case=ignore_case,
            ignore_mention=ignore_mention,
            magic=magic,
        )
        self.deep_link = deep_link
        self.deep_link_encoded = deep_link_encoded

    def __str__(self) -> str:
        return self._signature_to_string(
            ignore_case=self.ignore_case,
            ignore_mention=self.ignore_mention,
            magic=self.magic,
            deep_link=self.deep_link,
            deep_link_encoded=self.deep_link_encoded,
        )

    async def parse_command(self, text: str, bot: Bot) -> CommandObject:
        """
        Extract command from the text and validate

        :param text:
        :param bot:
        :return:
        """
        command = self.extract_command(text)
        self.validate_prefix(command=command)
        await self.validate_mention(bot=bot, command=command)
        command = self.validate_command(command)
        command = self.validate_deeplink(command=command)
        command = self.do_magic(command=command)
        return cast(CommandObject, command)

    def validate_deeplink(self, command: CommandObject) -> CommandObject:
        if self.deep_link is None:
            return command
        if self.deep_link is False:
            if command.args:
                msg = "Deep-link was not expected"
                raise CommandException(msg)
            return command
        if not command.args:
            msg = "Deep-link was missing"
            raise CommandException(msg)
        args = command.args
        if self.deep_link_encoded:
            try:
                args = decode_payload(args)
            except UnicodeDecodeError as e:
                msg = f"Failed to decode Base64: {e}"
                raise CommandException(msg) from e
            return replace(command, args=args)
        return command
