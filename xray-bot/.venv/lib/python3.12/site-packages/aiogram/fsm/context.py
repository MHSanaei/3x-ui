from collections.abc import Mapping
from typing import Any, overload

from aiogram.fsm.storage.base import BaseStorage, StateType, StorageKey


class FSMContext:
    def __init__(self, storage: BaseStorage, key: StorageKey) -> None:
        self.storage = storage
        self.key = key

    async def set_state(self, state: StateType = None) -> None:
        await self.storage.set_state(key=self.key, state=state)

    async def get_state(self) -> str | None:
        return await self.storage.get_state(key=self.key)

    async def set_data(self, data: Mapping[str, Any]) -> None:
        await self.storage.set_data(key=self.key, data=data)

    async def get_data(self) -> dict[str, Any]:
        return await self.storage.get_data(key=self.key)

    @overload
    async def get_value(self, key: str) -> Any | None: ...

    @overload
    async def get_value(self, key: str, default: Any) -> Any: ...

    async def get_value(self, key: str, default: Any | None = None) -> Any | None:
        return await self.storage.get_value(storage_key=self.key, dict_key=key, default=default)

    async def update_data(
        self,
        data: Mapping[str, Any] | None = None,
        **kwargs: Any,
    ) -> dict[str, Any]:
        if data:
            kwargs.update(data)
        return await self.storage.update_data(key=self.key, data=kwargs)

    async def clear(self) -> None:
        await self.set_state(state=None)
        await self.set_data({})
