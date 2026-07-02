from typing import Any, Dict, TypeVar

KT = TypeVar("KT")
VT = TypeVar("VT")


class AttrDict(Dict[KT, VT]):
    """
    A wrapper over dict which where element can be accessed as regular attributes
    """

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        super(AttrDict, self).__init__(*args, **kwargs)
        self.__dict__ = self  # type: ignore
