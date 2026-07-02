from typing import Any

from .text_decorations import html_decoration, markdown_decoration


def _join(*content: Any, sep: str = " ") -> str:
    return sep.join(map(str, content))


def text(*content: Any, sep: str = " ") -> str:
    """
    Join all elements with a separator

    :param content:
    :param sep:
    :return:
    """
    return _join(*content, sep=sep)


def bold(*content: Any, sep: str = " ") -> str:
    """
    Make bold text (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.bold(value=markdown_decoration.quote(_join(*content, sep=sep)))


def hbold(*content: Any, sep: str = " ") -> str:
    """
    Make bold text (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.bold(value=html_decoration.quote(_join(*content, sep=sep)))


def italic(*content: Any, sep: str = " ") -> str:
    """
    Make italic text (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.italic(value=markdown_decoration.quote(_join(*content, sep=sep)))


def hitalic(*content: Any, sep: str = " ") -> str:
    """
    Make italic text (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.italic(value=html_decoration.quote(_join(*content, sep=sep)))


def code(*content: Any, sep: str = " ") -> str:
    """
    Make mono-width text (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.code(value=markdown_decoration.quote(_join(*content, sep=sep)))


def hcode(*content: Any, sep: str = " ") -> str:
    """
    Make mono-width text (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.code(value=html_decoration.quote(_join(*content, sep=sep)))


def pre(*content: Any, sep: str = "\n") -> str:
    """
    Make mono-width text block (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.pre(value=markdown_decoration.quote(_join(*content, sep=sep)))


def hpre(*content: Any, sep: str = "\n") -> str:
    """
    Make mono-width text block (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.pre(value=html_decoration.quote(_join(*content, sep=sep)))


def underline(*content: Any, sep: str = " ") -> str:
    """
    Make underlined text (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.underline(value=markdown_decoration.quote(_join(*content, sep=sep)))


def hunderline(*content: Any, sep: str = " ") -> str:
    """
    Make underlined text (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.underline(value=html_decoration.quote(_join(*content, sep=sep)))


def strikethrough(*content: Any, sep: str = " ") -> str:
    """
    Make strikethrough text (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.strikethrough(
        value=markdown_decoration.quote(_join(*content, sep=sep)),
    )


def hstrikethrough(*content: Any, sep: str = " ") -> str:
    """
    Make strikethrough text (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.strikethrough(value=html_decoration.quote(_join(*content, sep=sep)))


def link(title: str, url: str) -> str:
    """
    Format URL (Markdown)

    :param title:
    :param url:
    :return:
    """
    return markdown_decoration.link(value=markdown_decoration.quote(title), link=url)


def hlink(title: str, url: str) -> str:
    """
    Format URL (HTML)

    :param title:
    :param url:
    :return:
    """
    return html_decoration.link(value=html_decoration.quote(title), link=url)


def blockquote(*content: Any, sep: str = "\n") -> str:
    """
    Make blockquote (Markdown)

    :param content:
    :param sep:
    :return:
    """
    return markdown_decoration.blockquote(
        value=markdown_decoration.quote(_join(*content, sep=sep)),
    )


def hblockquote(*content: Any, sep: str = "\n") -> str:
    """
    Make blockquote (HTML)

    :param content:
    :param sep:
    :return:
    """
    return html_decoration.blockquote(value=html_decoration.quote(_join(*content, sep=sep)))


def hide_link(url: str) -> str:
    """
    Hide URL (HTML only)
    Can be used for adding an image to a text message

    :param url:
    :return:
    """
    return f'<a href="{url}">&#8203;</a>'
