from typing import Any
from urllib.parse import urlencode, urljoin

BASE_DOCS_URL = "https://docs.aiogram.dev/"
BRANCH = "dev-3.x"

BASE_PAGE_URL = f"{BASE_DOCS_URL}/en/{BRANCH}/"


def _format_url(url: str, *path: str, fragment_: str | None = None, **query: Any) -> str:
    url = urljoin(url, "/".join(path), allow_fragments=True)
    if query:
        url += "?" + urlencode(query)
    if fragment_:
        url += "#" + fragment_
    return url


def docs_url(*path: str, fragment_: str | None = None, **query: Any) -> str:
    return _format_url(BASE_PAGE_URL, *path, fragment_=fragment_, **query)


def create_tg_link(link: str, **kwargs: Any) -> str:
    return _format_url(f"tg://{link}", **kwargs)


def create_telegram_link(*path: str, **kwargs: Any) -> str:
    return _format_url("https://t.me", *path, **kwargs)


def create_channel_bot_link(
    username: str,
    parameter: str | None = None,
    change_info: bool = False,
    post_messages: bool = False,
    edit_messages: bool = False,
    delete_messages: bool = False,
    restrict_members: bool = False,
    invite_users: bool = False,
    pin_messages: bool = False,
    promote_members: bool = False,
    manage_video_chats: bool = False,
    anonymous: bool = False,
    manage_chat: bool = False,
) -> str:
    params = {}
    if parameter is not None:
        params["startgroup"] = parameter
    permissions = []
    if change_info:
        permissions.append("change_info")
    if post_messages:
        permissions.append("post_messages")
    if edit_messages:
        permissions.append("edit_messages")
    if delete_messages:
        permissions.append("delete_messages")
    if restrict_members:
        permissions.append("restrict_members")
    if invite_users:
        permissions.append("invite_users")
    if pin_messages:
        permissions.append("pin_messages")
    if promote_members:
        permissions.append("promote_members")
    if manage_video_chats:
        permissions.append("manage_video_chats")
    if anonymous:
        permissions.append("anonymous")
    if manage_chat:
        permissions.append("manage_chat")
    if permissions:
        params["admin"] = "+".join(permissions)
    return create_telegram_link(username, **params)
