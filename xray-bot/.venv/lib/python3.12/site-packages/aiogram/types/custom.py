import sys
from datetime import datetime, timezone
from typing import Annotated

from pydantic import PlainSerializer

if sys.platform == "win32":  # pragma: no cover

    def _datetime_serializer(value: datetime) -> int:
        tz = timezone.utc if value.tzinfo else None

        # https://github.com/aiogram/aiogram/issues/349
        # https://github.com/aiogram/aiogram/pull/880
        return round((value - datetime(1970, 1, 1, tzinfo=tz)).total_seconds())

else:  # pragma: no cover

    def _datetime_serializer(value: datetime) -> int:
        return round(value.timestamp())


# Make datetime compatible with Telegram Bot API (unixtime)
DateTime = Annotated[
    datetime,
    PlainSerializer(
        func=_datetime_serializer,
        return_type=int,
        when_used="unless-none",
    ),
]
