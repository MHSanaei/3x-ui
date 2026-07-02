from typing import Protocol


class Downloadable(Protocol):
    @property
    def file_id(self) -> str: ...
