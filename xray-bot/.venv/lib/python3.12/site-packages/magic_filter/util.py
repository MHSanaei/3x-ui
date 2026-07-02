from typing import Any, Container


def in_op(a: Container[Any], b: Any) -> bool:
    try:
        return b in a
    except TypeError:
        return False


def not_in_op(a: Container[Any], b: Any) -> bool:
    try:
        return b not in a
    except TypeError:
        return False


def contains_op(a: Any, b: Container[Any]) -> bool:
    try:
        return a in b
    except TypeError:
        return False


def not_contains_op(a: Any, b: Container[Any]) -> bool:
    try:
        return a not in b
    except TypeError:
        return False


def and_op(a: Any, b: Any) -> Any:
    return a and b


def or_op(a: Any, b: Any) -> Any:
    return a or b
