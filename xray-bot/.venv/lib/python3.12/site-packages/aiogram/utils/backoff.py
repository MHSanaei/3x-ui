import asyncio
import time
from dataclasses import dataclass
from random import normalvariate


@dataclass(frozen=True)
class BackoffConfig:
    min_delay: float
    max_delay: float
    factor: float
    jitter: float

    def __post_init__(self) -> None:
        if self.max_delay <= self.min_delay:
            msg = "`max_delay` should be greater than `min_delay`"
            raise ValueError(msg)
        if self.factor <= 1:
            msg = "`factor` should be greater than 1"
            raise ValueError(msg)


class Backoff:
    def __init__(self, config: BackoffConfig) -> None:
        self.config = config
        self._next_delay = config.min_delay
        self._current_delay = 0.0
        self._counter = 0

    def __iter__(self) -> "Backoff":
        return self

    @property
    def min_delay(self) -> float:
        return self.config.min_delay

    @property
    def max_delay(self) -> float:
        return self.config.max_delay

    @property
    def factor(self) -> float:
        return self.config.factor

    @property
    def jitter(self) -> float:
        return self.config.jitter

    @property
    def next_delay(self) -> float:
        return self._next_delay

    @property
    def current_delay(self) -> float:
        return self._current_delay

    @property
    def counter(self) -> int:
        return self._counter

    def sleep(self) -> None:
        time.sleep(next(self))

    async def asleep(self) -> None:
        await asyncio.sleep(next(self))

    def _calculate_next(self, value: float) -> float:
        return normalvariate(min(value * self.factor, self.max_delay), self.jitter)

    def __next__(self) -> float:
        self._current_delay = self._next_delay
        self._next_delay = self._calculate_next(self._next_delay)
        self._counter += 1
        return self._current_delay

    def reset(self) -> None:
        self._current_delay = 0.0
        self._counter = 0
        self._next_delay = self.min_delay

    def __str__(self) -> str:
        return (
            f"Backoff(tryings={self._counter}, current_delay={self._current_delay}, "
            f"next_delay={self._next_delay})"
        )
