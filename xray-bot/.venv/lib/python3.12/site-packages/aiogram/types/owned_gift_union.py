from typing import TypeAlias

from .owned_gift_regular import OwnedGiftRegular
from .owned_gift_unique import OwnedGiftUnique

OwnedGiftUnion: TypeAlias = OwnedGiftRegular | OwnedGiftUnique
