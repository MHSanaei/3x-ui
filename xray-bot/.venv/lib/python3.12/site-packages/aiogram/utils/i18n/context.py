from typing import Any

from aiogram.utils.i18n.core import I18n
from aiogram.utils.i18n.lazy_proxy import LazyProxy


def get_i18n() -> I18n:
    i18n = I18n.get_current(no_error=True)
    if i18n is None:
        msg = "I18n context is not set"
        raise LookupError(msg)
    return i18n


def gettext(*args: Any, **kwargs: Any) -> str:
    return get_i18n().gettext(*args, **kwargs)


def lazy_gettext(*args: Any, **kwargs: Any) -> LazyProxy:
    return LazyProxy(gettext, *args, **kwargs, enable_cache=False)


ngettext = gettext
lazy_ngettext = lazy_gettext
