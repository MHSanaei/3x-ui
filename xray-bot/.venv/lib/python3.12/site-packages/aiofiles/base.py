from asyncio import get_running_loop
from collections.abc import Awaitable
from contextlib import AbstractAsyncContextManager
from functools import partial, wraps


def wrap(func):
    @wraps(func)
    async def run(*args, loop=None, executor=None, **kwargs):
        if loop is None:
            loop = get_running_loop()
        pfunc = partial(func, *args, **kwargs)
        return await loop.run_in_executor(executor, pfunc)

    return run


class AsyncBase:
    def __init__(self, file, loop, executor):
        self._file = file
        self._executor = executor
        self._ref_loop = loop

    @property
    def _loop(self):
        return self._ref_loop or get_running_loop()

    def __aiter__(self):
        """We are our own iterator."""
        return self

    def __repr__(self):
        return super().__repr__() + " wrapping " + repr(self._file)

    async def __anext__(self):
        """Simulate normal file iteration."""

        if line := await self.readline():
            return line
        raise StopAsyncIteration


class AsyncIndirectBase(AsyncBase):
    def __init__(self, name, loop, executor, indirect):
        self._indirect = indirect
        self._name = name
        super().__init__(None, loop, executor)

    @property
    def _file(self):
        return self._indirect()

    @_file.setter
    def _file(self, v):
        pass  # discard writes


class AiofilesContextManager(Awaitable, AbstractAsyncContextManager):
    """An adjusted async context manager for aiofiles."""

    __slots__ = ("_coro", "_obj")

    def __init__(self, coro):
        self._coro = coro
        self._obj = None

    def __await__(self):
        if self._obj is None:
            self._obj = yield from self._coro.__await__()
        return self._obj

    async def __aenter__(self):
        return await self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        await get_running_loop().run_in_executor(
            None, self._obj._file.__exit__, exc_type, exc_val, exc_tb
        )
        self._obj = None
