"""Async executor versions of file functions from the os.path module."""

from os import path

from .base import wrap

__all__ = [
    "abspath",
    "getatime",
    "getctime",
    "getmtime",
    "getsize",
    "exists",
    "isdir",
    "isfile",
    "islink",
    "ismount",
    "samefile",
    "sameopenfile",
]

abspath = wrap(path.abspath)

getatime = wrap(path.getatime)
getctime = wrap(path.getctime)
getmtime = wrap(path.getmtime)
getsize = wrap(path.getsize)

exists = wrap(path.exists)

isdir = wrap(path.isdir)
isfile = wrap(path.isfile)
islink = wrap(path.islink)
ismount = wrap(path.ismount)

samefile = wrap(path.samefile)
sameopenfile = wrap(path.sameopenfile)
