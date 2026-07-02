import os
import warnings
from collections.abc import Iterator
from functools import reduce
from pathlib import Path
from typing import TYPE_CHECKING, Any, Literal, Optional

from ...exceptions import SettingsError
from ...utils import path_type_label
from ..base import PydanticBaseSettingsSource
from ..utils import parse_env_vars
from .env import EnvSettingsSource
from .secrets import SecretsSettingsSource

if TYPE_CHECKING:
    from ...main import BaseSettings
    from ...sources import PathType


SECRETS_DIR_MAX_SIZE = 16 * 2**20  # 16 MiB seems to be a reasonable default


class NestedSecretsSettingsSource(EnvSettingsSource):
    def __init__(
        self,
        file_secret_settings: PydanticBaseSettingsSource | SecretsSettingsSource,
        secrets_dir: Optional['PathType'] = None,
        secrets_dir_missing: Literal['ok', 'warn', 'error'] | None = None,
        secrets_dir_max_size: int | None = None,
        secrets_case_sensitive: bool | None = None,
        secrets_prefix: str | None = None,
        secrets_nested_delimiter: str | None = None,
        secrets_nested_subdir: bool | None = None,
        # args for compatibility with SecretsSettingsSource, don't use directly
        case_sensitive: bool | None = None,
        env_prefix: str | None = None,
    ) -> None:
        # We allow the first argument to be settings_cls like original
        # SecretsSettingsSource. However, it is recommended to pass
        # SecretsSettingsSource instance instead (as it is shown in usage examples),
        # otherwise `_secrets_dir` arg passed to Settings() constructor will be ignored.
        settings_cls: type[BaseSettings] = getattr(
            file_secret_settings,
            'settings_cls',
            file_secret_settings,  # type: ignore[arg-type]
        )
        # config options
        conf = settings_cls.model_config
        self.secrets_dir: PathType | None = first_not_none(
            getattr(file_secret_settings, 'secrets_dir', None),
            secrets_dir,
            conf.get('secrets_dir'),
        )
        self.secrets_dir_missing: Literal['ok', 'warn', 'error'] = first_not_none(
            secrets_dir_missing,
            conf.get('secrets_dir_missing'),
            'warn',
        )
        if self.secrets_dir_missing not in ('ok', 'warn', 'error'):
            raise SettingsError(f'invalid secrets_dir_missing value: {self.secrets_dir_missing}')
        self.secrets_dir_max_size: int = first_not_none(
            secrets_dir_max_size,
            conf.get('secrets_dir_max_size'),
            SECRETS_DIR_MAX_SIZE,
        )
        self.case_sensitive: bool = first_not_none(
            secrets_case_sensitive,
            conf.get('secrets_case_sensitive'),
            case_sensitive,
            conf.get('case_sensitive'),
            False,
        )
        self.secrets_prefix: str = first_not_none(
            secrets_prefix,
            conf.get('secrets_prefix'),
            env_prefix,
            conf.get('env_prefix'),
            '',
        )

        # nested options
        self.secrets_nested_delimiter: str | None = first_not_none(
            secrets_nested_delimiter,
            conf.get('secrets_nested_delimiter'),
            conf.get('env_nested_delimiter'),
        )
        self.secrets_nested_subdir: bool = first_not_none(
            secrets_nested_subdir,
            conf.get('secrets_nested_subdir'),
            False,
        )
        if self.secrets_nested_subdir:
            if secrets_nested_delimiter or conf.get('secrets_nested_delimiter'):
                raise SettingsError('Options secrets_nested_delimiter and secrets_nested_subdir are mutually exclusive')
            else:
                self.secrets_nested_delimiter = os.sep

        # ensure valid secrets_path
        if self.secrets_dir is None:
            paths = []
        elif isinstance(self.secrets_dir, (Path, str)):
            paths = [self.secrets_dir]
        else:
            paths = list(self.secrets_dir)
        self.secrets_paths: list[Path] = [Path(p).expanduser().resolve() for p in paths]
        for path in self.secrets_paths:
            self.validate_secrets_path(path)

        # construct parent
        super().__init__(
            settings_cls,
            case_sensitive=self.case_sensitive,
            env_prefix=self.secrets_prefix,
            env_nested_delimiter=self.secrets_nested_delimiter,
            env_ignore_empty=False,  # match SecretsSettingsSource behaviour
            env_parse_enums=True,  # we can pass everything here, it will still behave as "True"
            env_parse_none_str=None,  # match SecretsSettingsSource behaviour
        )
        self.env_parse_none_str = None  # update manually because of None

        # update parent members
        if not len(self.secrets_paths):
            self.env_vars = {}
        else:
            secrets = reduce(
                lambda d1, d2: dict((*d1.items(), *d2.items())),
                (self.load_secrets(p) for p in self.secrets_paths),
            )
            self.env_vars = parse_env_vars(
                secrets,
                self.case_sensitive,
                self.env_ignore_empty,
                self.env_parse_none_str,
            )

    def validate_secrets_path(self, path: Path) -> None:
        if not path.exists():
            if self.secrets_dir_missing == 'ok':
                pass
            elif self.secrets_dir_missing == 'warn':
                warnings.warn(f'directory "{path}" does not exist', stacklevel=2)
            elif self.secrets_dir_missing == 'error':
                raise SettingsError(f'directory "{path}" does not exist')
            else:
                raise ValueError  # unreachable, checked before
        else:
            if not path.is_dir():
                raise SettingsError(f'secrets_dir must reference a directory, not a {path_type_label(path)}')
            secrets_dir_size = sum(f.stat().st_size for f in self._iter_secret_files(path))
            if secrets_dir_size > self.secrets_dir_max_size:
                raise SettingsError(f'secrets_dir size is above {self.secrets_dir_max_size} bytes')

    @staticmethod
    def _iter_secret_files(path: Path) -> Iterator[Path]:
        """Yield the secret files contained in ``path``.

        ``path`` is expected to already be resolved. The directory tree is walked
        explicitly so that symbolic links are handled safely:

        * a file is only yielded if its real location stays within ``path``; entries
          that resolve outside of it (e.g. through a symbolic link) are skipped, so
          they neither contribute to the ``secrets_dir_max_size`` accounting nor get
          loaded;
        * each real directory is visited at most once, so cyclic or repeated
          symlinks cannot make the walk loop and inflate the size accounting or the
          number of loaded secrets.

        Because the size check and the loader share this iterator, they always see
        the same set of files.
        """
        seen_dirs: set[Path] = set()

        def walk(directory: Path) -> Iterator[Path]:
            # Guard against symlink loops / a directory reachable through multiple
            # links being traversed more than once.
            resolved_dir = directory.resolve()
            if resolved_dir in seen_dirs:
                return
            seen_dirs.add(resolved_dir)
            try:
                entries = sorted(directory.iterdir())
            except OSError:
                return
            for entry in entries:
                resolved = entry.resolve()
                if resolved.is_dir():
                    # Only descend into directories that stay within secrets_dir.
                    # A symlinked directory pointing outside of ``path`` is not
                    # followed at all, so we never walk (potentially large) external
                    # trees and never read files from outside secrets_dir.
                    if resolved == path or path in resolved.parents:
                        yield from walk(entry)
                elif resolved.is_file() and path in resolved.parents:
                    # Defense in depth: a file whose real location escapes
                    # secrets_dir (e.g. a symlink pointing outside of ``path``) is
                    # skipped from both the size accounting and the load.
                    yield entry

        yield from walk(path)

    @classmethod
    def load_secrets(cls, path: Path) -> dict[str, str]:
        return {str(p.relative_to(path)): p.read_text().strip() for p in cls._iter_secret_files(path)}

    def __repr__(self) -> str:
        return f'NestedSecretsSettingsSource(secrets_dir={self.secrets_dir!r})'


def first_not_none(*objs: Any) -> Any:
    return next(filter(lambda o: o is not None, objs), None)
