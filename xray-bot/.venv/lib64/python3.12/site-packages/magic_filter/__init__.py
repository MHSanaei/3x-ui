from . import operations
from .attrdict import AttrDict
from .magic import MagicFilter, MagicT, RegexpMode

__all__ = (
    "__version__",
    "operations",
    "MagicFilter",
    "MagicT",
    "RegexpMode",
    "F",
    "AttrDict",
)

__version__ = "1.0.12"

F = MagicFilter()
