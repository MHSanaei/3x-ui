from __future__ import annotations

import html
import re
from abc import ABC, abstractmethod
from datetime import date, datetime, time
from typing import TYPE_CHECKING, cast

from aiogram.enums import MessageEntityType
from aiogram.utils.link import create_tg_link

if TYPE_CHECKING:
    from collections.abc import Generator
    from re import Pattern

    from aiogram.types import MessageEntity

__all__ = (
    "HtmlDecoration",
    "MarkdownDecoration",
    "TextDecoration",
    "add_surrogates",
    "html_decoration",
    "markdown_decoration",
    "remove_surrogates",
)


def add_surrogates(text: str) -> bytes:
    return text.encode("utf-16-le")


def remove_surrogates(text: bytes) -> str:
    return text.decode("utf-16-le")


class TextDecoration(ABC):
    def apply_entity(self, entity: MessageEntity, text: str) -> str:
        """
        Apply single entity to text

        :param entity:
        :param text:
        :return:
        """
        if entity.type in {
            MessageEntityType.BOT_COMMAND,
            MessageEntityType.URL,
            MessageEntityType.MENTION,
            MessageEntityType.PHONE_NUMBER,
            MessageEntityType.HASHTAG,
            MessageEntityType.CASHTAG,
            MessageEntityType.EMAIL,
        }:
            # These entities should not be changed
            return text
        if entity.type in {
            MessageEntityType.BOLD,
            MessageEntityType.ITALIC,
            MessageEntityType.CODE,
            MessageEntityType.UNDERLINE,
            MessageEntityType.STRIKETHROUGH,
            MessageEntityType.SPOILER,
            MessageEntityType.BLOCKQUOTE,
            MessageEntityType.EXPANDABLE_BLOCKQUOTE,
        }:
            return cast(str, getattr(self, entity.type)(value=text))
        if entity.type == MessageEntityType.PRE:
            return (
                self.pre_language(value=text, language=entity.language)
                if entity.language
                else self.pre(value=text)
            )
        if entity.type == MessageEntityType.TEXT_MENTION:
            from aiogram.types import User

            user = cast(User, entity.user)
            return self.link(value=text, link=f"tg://user?id={user.id}")
        if entity.type == MessageEntityType.TEXT_LINK:
            return self.link(value=text, link=cast(str, entity.url))
        if entity.type == MessageEntityType.CUSTOM_EMOJI:
            return self.custom_emoji(value=text, custom_emoji_id=cast(str, entity.custom_emoji_id))
        if entity.type == MessageEntityType.DATE_TIME:
            return self.date_time(
                value=text,
                unix_time=cast(int, entity.unix_time),
                date_time_format=entity.date_time_format,
            )

        # This case is not possible because of `if` above, but if any new entity is added to
        # API it will be here too
        return self.quote(text)

    def unparse(self, text: str, entities: list[MessageEntity] | None = None) -> str:
        """
        Unparse message entities

        :param text: raw text
        :param entities: Array of MessageEntities
        :return:
        """
        return "".join(
            self._unparse_entities(
                add_surrogates(text),
                sorted(entities, key=lambda item: item.offset) if entities else [],
            ),
        )

    def _unparse_entities(
        self,
        text: bytes,
        entities: list[MessageEntity],
        offset: int | None = None,
        length: int | None = None,
    ) -> Generator[str, None, None]:
        if offset is None:
            offset = 0
        length = length or len(text)

        for index, entity in enumerate(entities):
            if entity.offset * 2 < offset:
                continue
            if entity.offset * 2 > offset:
                yield self.quote(remove_surrogates(text[offset : entity.offset * 2]))
            start = entity.offset * 2
            offset = entity.offset * 2 + entity.length * 2

            sub_entities = list(
                filter(lambda e: e.offset * 2 < (offset or 0), entities[index + 1 :]),
            )
            yield self.apply_entity(
                entity,
                "".join(self._unparse_entities(text, sub_entities, offset=start, length=offset)),
            )

        if offset < length:
            yield self.quote(remove_surrogates(text[offset:length]))

    @abstractmethod
    def link(self, value: str, link: str) -> str:
        pass

    @abstractmethod
    def bold(self, value: str) -> str:
        pass

    @abstractmethod
    def italic(self, value: str) -> str:
        pass

    @abstractmethod
    def code(self, value: str) -> str:
        pass

    @abstractmethod
    def pre(self, value: str) -> str:
        pass

    @abstractmethod
    def pre_language(self, value: str, language: str) -> str:
        pass

    @abstractmethod
    def underline(self, value: str) -> str:
        pass

    @abstractmethod
    def strikethrough(self, value: str) -> str:
        pass

    @abstractmethod
    def spoiler(self, value: str) -> str:
        pass

    @abstractmethod
    def quote(self, value: str) -> str:
        pass

    @abstractmethod
    def custom_emoji(self, value: str, custom_emoji_id: str) -> str:
        pass

    @abstractmethod
    def blockquote(self, value: str) -> str:
        pass

    @abstractmethod
    def expandable_blockquote(self, value: str) -> str:
        pass

    @abstractmethod
    def date_time(
        self,
        value: str,
        unix_time: int | datetime,
        date_time_format: str | None = None,
    ) -> str:
        pass


class HtmlDecoration(TextDecoration):
    BOLD_TAG = "b"
    ITALIC_TAG = "i"
    UNDERLINE_TAG = "u"
    STRIKETHROUGH_TAG = "s"
    CODE_TAG = "code"
    PRE_TAG = "pre"
    LINK_TAG = "a"
    SPOILER_TAG = "tg-spoiler"
    EMOJI_TAG = "tg-emoji"
    DATE_TIME_TAG = "tg-time"
    BLOCKQUOTE_TAG = "blockquote"

    def _tag(
        self,
        tag: str,
        content: str,
        *,
        attrs: dict[str, str] | None = None,
        flags: list[str] | None = None,
    ) -> str:
        prepared_attrs: list[str] = []
        if attrs:
            prepared_attrs.extend(f'{k}="{v}"' for k, v in attrs.items())
        if flags:
            prepared_attrs.extend(f"{flag}" for flag in flags)

        attrs_str = " ".join(prepared_attrs)
        if attrs_str:
            attrs_str = " " + attrs_str

        return f"<{tag}{attrs_str}>{content}</{tag}>"

    def link(self, value: str, link: str) -> str:
        return self._tag(self.LINK_TAG, value, attrs={"href": link})

    def bold(self, value: str) -> str:
        return self._tag(self.BOLD_TAG, value)

    def italic(self, value: str) -> str:
        return self._tag(self.ITALIC_TAG, value)

    def code(self, value: str) -> str:
        return self._tag(self.CODE_TAG, value)

    def pre(self, value: str) -> str:
        return self._tag(self.PRE_TAG, value)

    def pre_language(self, value: str, language: str) -> str:
        return self._tag(
            self.PRE_TAG,
            self._tag(self.CODE_TAG, value, attrs={"language": f"language-{language}"}),
        )

    def underline(self, value: str) -> str:
        return self._tag(self.UNDERLINE_TAG, value)

    def strikethrough(self, value: str) -> str:
        return self._tag(self.STRIKETHROUGH_TAG, value)

    def spoiler(self, value: str) -> str:
        return self._tag(self.SPOILER_TAG, value)

    def quote(self, value: str) -> str:
        return html.escape(value, quote=False)

    def custom_emoji(self, value: str, custom_emoji_id: str) -> str:
        return self._tag(self.EMOJI_TAG, value, attrs={"emoji-id": custom_emoji_id})

    def blockquote(self, value: str) -> str:
        return self._tag(self.BLOCKQUOTE_TAG, value)

    def expandable_blockquote(self, value: str) -> str:
        return self._tag(self.BLOCKQUOTE_TAG, value, flags=["expandable"])

    def date_time(
        self,
        value: str,
        unix_time: int | datetime,
        date_time_format: str | None = None,
    ) -> str:
        if isinstance(unix_time, datetime):
            unix_time = int(unix_time.timestamp())

        args = {"unix": str(unix_time)}
        if date_time_format:
            args["format"] = date_time_format

        return self._tag(self.DATE_TIME_TAG, value, attrs=args)


class MarkdownDecoration(TextDecoration):
    MARKDOWN_QUOTE_PATTERN: Pattern[str] = re.compile(r"([_*\[\]()~`>#+\-=|{}.!\\])")

    def link(self, value: str, link: str) -> str:
        return f"[{value}]({link})"

    def bold(self, value: str) -> str:
        return f"*{value}*"

    def italic(self, value: str) -> str:
        return f"_\r{value}_\r"

    def code(self, value: str) -> str:
        return f"`{value}`"

    def pre(self, value: str) -> str:
        return f"```\n{value}\n```"

    def pre_language(self, value: str, language: str) -> str:
        return f"```{language}\n{value}\n```"

    def underline(self, value: str) -> str:
        return f"__\r{value}__\r"

    def strikethrough(self, value: str) -> str:
        return f"~{value}~"

    def spoiler(self, value: str) -> str:
        return f"||{value}||"

    def quote(self, value: str) -> str:
        return re.sub(pattern=self.MARKDOWN_QUOTE_PATTERN, repl=r"\\\1", string=value)

    def custom_emoji(self, value: str, custom_emoji_id: str) -> str:
        link = create_tg_link("emoji", emoji_id=custom_emoji_id)
        return f"!{self.link(value=value, link=link)}"

    def blockquote(self, value: str) -> str:
        return "\n".join(f">{line}" for line in value.splitlines())

    def expandable_blockquote(self, value: str) -> str:
        return "\n".join(f">{line}" for line in value.splitlines()) + "||"

    def date_time(
        self,
        value: str,
        unix_time: int | datetime,
        date_time_format: str | None = None,
    ) -> str:
        if isinstance(unix_time, datetime):
            unix_time = int(unix_time.timestamp())

        link_params = {"unix": str(unix_time)}
        if date_time_format:
            link_params["format"] = date_time_format
        link = create_tg_link("time", **link_params)

        return f"!{self.link(value, link=link)}"


html_decoration = HtmlDecoration()
markdown_decoration = MarkdownDecoration()
