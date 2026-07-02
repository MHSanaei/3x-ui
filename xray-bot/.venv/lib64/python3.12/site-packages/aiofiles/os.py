"""Async executor versions of file functions from the os module."""

import os

from . import ospath as path
from .base import wrap

__all__ = [
    "path",
    "stat",
    "rename",
    "renames",
    "replace",
    "remove",
    "unlink",
    "mkdir",
    "makedirs",
    "rmdir",
    "removedirs",
    "symlink",
    "readlink",
    "listdir",
    "scandir",
    "access",
    "wrap",
    "getcwd",
]

access = wrap(os.access)

getcwd = wrap(os.getcwd)

listdir = wrap(os.listdir)

makedirs = wrap(os.makedirs)
mkdir = wrap(os.mkdir)

readlink = wrap(os.readlink)
remove = wrap(os.remove)
removedirs = wrap(os.removedirs)
rename = wrap(os.rename)
renames = wrap(os.renames)
replace = wrap(os.replace)
rmdir = wrap(os.rmdir)

scandir = wrap(os.scandir)
stat = wrap(os.stat)
symlink = wrap(os.symlink)

unlink = wrap(os.unlink)


if hasattr(os, "link"):
    __all__ += ["link"]
    link = wrap(os.link)
if hasattr(os, "sendfile"):
    __all__ += ["sendfile"]
    sendfile = wrap(os.sendfile)
if hasattr(os, "statvfs"):
    __all__ += ["statvfs"]
    statvfs = wrap(os.statvfs)
