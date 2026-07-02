from __future__ import annotations

import datetime
from typing import TypeAlias

DateTimeUnion: TypeAlias = datetime.datetime | datetime.timedelta | int
