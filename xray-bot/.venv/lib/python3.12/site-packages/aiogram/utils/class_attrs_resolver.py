import inspect
from collections.abc import Generator
from dataclasses import dataclass
from operator import itemgetter
from typing import Any, NamedTuple, Protocol

from aiogram.utils.dataclass import dataclass_kwargs


class ClassAttrsResolver(Protocol):
    def __call__(self, cls: type) -> Generator[tuple[str, Any], None, None]: ...


def inspect_members_resolver(cls: type) -> Generator[tuple[str, Any], None, None]:
    """
    Inspects and resolves attributes of a given class.

    This function uses the `inspect.getmembers` utility to yield all attributes of
    a provided class. The output is a generator that produces tuples containing
    attribute names and their corresponding values. This function is suitable for
    analyzing class attributes dynamically. However, it guarantees alphabetical
    order of attributes.

    :param cls: The class for which the attributes will be resolved.
    :return: A generator yielding tuples containing attribute names and their values.
    """
    yield from inspect.getmembers(cls)


def get_reversed_mro_unique_attrs_resolver(cls: type) -> Generator[tuple[str, Any], None, None]:
    """
    Resolve and yield attributes from the reversed method resolution order (MRO) of a given class.

    This function iterates through the reversed MRO of a class and yields attributes
    that have not yet been encountered. It avoids duplicates by keeping track of
    attribute names that have already been processed.

    :param cls: The class for which the attributes will be resolved.
    :return: A generator yielding tuples containing attribute names and their values.
    """
    known_attrs = set()
    for base in reversed(inspect.getmro(cls)):
        for name, value in base.__dict__.items():
            if name in known_attrs:
                continue

            yield name, value
            known_attrs.add(name)


class _Position(NamedTuple):
    in_mro: int
    in_class: int


@dataclass(**dataclass_kwargs(slots=True))
class _AttributeContainer:
    position: _Position
    value: Any

    def __lt__(self, other: "_AttributeContainer") -> bool:
        return self.position < other.position


def get_sorted_mro_attrs_resolver(cls: type) -> Generator[tuple[str, Any], None, None]:
    """
    Resolve and yield attributes from the method resolution order (MRO) of a given class.

    Iterates through a class's method resolution order (MRO) and collects its attributes
    along with their respective positions in the MRO and the class hierarchy. This generator
    yields a tuple containing the name of each attribute and its associated value.

    :param cls: The class for which the attributes will be resolved.
    :return: A generator yielding tuples containing attribute names and their values.
    """
    attributes: dict[str, _AttributeContainer] = {}
    for position_in_mro, base in enumerate(inspect.getmro(cls)):
        for position_in_class, (name, value) in enumerate(vars(base).items()):
            position = _Position(position_in_mro, position_in_class)
            if attribute := attributes.get(name):
                attribute.position = position
                continue

            attributes[name] = _AttributeContainer(value=value, position=position)

    for name, attribute in sorted(attributes.items(), key=itemgetter(1)):
        yield name, attribute.value
