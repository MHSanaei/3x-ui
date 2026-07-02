from __future__ import annotations

__all__ = [
    "create_deep_link",
    "create_start_link",
    "create_startapp_link",
    "create_startgroup_link",
    "create_telegram_link",
    "decode_payload",
    "encode_payload",
]

import re
from typing import TYPE_CHECKING, Literal, cast

from aiogram.utils.link import create_telegram_link
from aiogram.utils.payload import decode_payload, encode_payload

if TYPE_CHECKING:
    from collections.abc import Callable

    from aiogram import Bot

BAD_PATTERN = re.compile(r"[^a-zA-Z0-9-_]")
DEEPLINK_PAYLOAD_LENGTH = 64


async def create_start_link(
    bot: Bot,
    payload: str,
    encode: bool = False,
    encoder: Callable[[bytes], bytes] | None = None,
) -> str:
    """
    Create 'start' deep link with your payload.

    If you need to encode payload or pass special characters - set encode as True

    :param bot: bot instance
    :param payload: args passed with /start
    :param encode: encode payload with base64url or custom encoder
    :param encoder: custom encoder callable
    :return: link
    """
    username = (await bot.me()).username
    return create_deep_link(
        username=cast(str, username),
        link_type="start",
        payload=payload,
        encode=encode,
        encoder=encoder,
    )


async def create_startgroup_link(
    bot: Bot,
    payload: str,
    encode: bool = False,
    encoder: Callable[[bytes], bytes] | None = None,
) -> str:
    """
    Create 'startgroup' deep link with your payload.

    If you need to encode payload or pass special characters - set encode as True

    :param bot: bot instance
    :param payload: args passed with /start
    :param encode: encode payload with base64url or custom encoder
    :param encoder: custom encoder callable
    :return: link
    """
    username = (await bot.me()).username
    return create_deep_link(
        username=cast(str, username),
        link_type="startgroup",
        payload=payload,
        encode=encode,
        encoder=encoder,
    )


async def create_startapp_link(
    bot: Bot,
    payload: str,
    encode: bool = False,
    app_name: str | None = None,
    encoder: Callable[[bytes], bytes] | None = None,
) -> str:
    """
    Create 'startapp' deep link with your payload.

    If you need to encode payload or pass special characters - set encode as True

    **Read more**:

    - `Main Mini App links <https://core.telegram.org/api/links#main-mini-app-links>`_
    - `Direct mini app links <https://core.telegram.org/api/links#direct-mini-app-links>`_

    :param bot: bot instance
    :param payload: args passed with /start
    :param encode: encode payload with base64url or custom encoder
    :param app_name: if you want direct mini app link
    :param encoder: custom encoder callable
    :return: link
    """
    username = (await bot.me()).username
    return create_deep_link(
        username=cast(str, username),
        link_type="startapp",
        payload=payload,
        app_name=app_name,
        encode=encode,
        encoder=encoder,
    )


def create_deep_link(
    username: str,
    link_type: Literal["start", "startgroup", "startapp"],
    payload: str,
    app_name: str | None = None,
    encode: bool = False,
    encoder: Callable[[bytes], bytes] | None = None,
) -> str:
    """
    Create deep link.

    :param username:
    :param link_type: `start`, `startgroup` or `startapp`
    :param payload: any string-convertible data
    :param app_name: if you want direct mini app link
    :param encode: encode payload with base64url or custom encoder
    :param encoder: custom encoder callable
    :return: deeplink
    """
    if not isinstance(payload, str):
        payload = str(payload)

    if encode or encoder:
        payload = encode_payload(payload, encoder=encoder)

    if re.search(BAD_PATTERN, payload):
        msg = (
            "Wrong payload! Only A-Z, a-z, 0-9, _ and - are allowed. "
            "Pass `encode=True` or encode payload manually."
        )
        raise ValueError(msg)

    if len(payload) > DEEPLINK_PAYLOAD_LENGTH:
        msg = f"Payload must be up to {DEEPLINK_PAYLOAD_LENGTH} characters long."
        raise ValueError(msg)

    if not app_name:
        deep_link = create_telegram_link(username, **{cast(str, link_type): payload})
    else:
        deep_link = create_telegram_link(username, app_name, **{cast(str, link_type): payload})

    return deep_link
