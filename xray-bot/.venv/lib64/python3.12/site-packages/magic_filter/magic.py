import operator
import re
from functools import wraps
from typing import Any, Callable, Container, Optional, Pattern, Tuple, Type, TypeVar, Union
from warnings import warn

from magic_filter.exceptions import (
    ParamsConflict,
    RejectOperations,
    SwitchModeToAll,
    SwitchModeToAny,
)
from magic_filter.operations import (
    BaseOperation,
    CallOperation,
    CastOperation,
    CombinationOperation,
    ComparatorOperation,
    ExtractOperation,
    FunctionOperation,
    GetAttributeOperation,
    GetItemOperation,
    ImportantCombinationOperation,
    ImportantFunctionOperation,
    RCombinationOperation,
    SelectorOperation,
)
from magic_filter.util import and_op, contains_op, in_op, not_contains_op, not_in_op, or_op

MagicT = TypeVar("MagicT", bound="MagicFilter")


class RegexpMode:
    SEARCH = "search"
    MATCH = "match"
    FINDALL = "findall"
    FINDITER = "finditer"
    FULLMATCH = "fullmatch"


class MagicFilter:
    __slots__ = ("_operations",)

    def __init__(self, operations: Tuple[BaseOperation, ...] = ()) -> None:
        self._operations = operations

    # An instance of MagicFilter cannot be used as an iterable object because objects
    # with a __getitem__ method can be endlessly iterated, which is not the desired behavior.
    __iter__ = None

    @classmethod
    def ilter(cls, magic: "MagicFilter") -> Callable[[Any], Any]:
        @wraps(magic.resolve)
        def wrapper(value: Any) -> Any:
            return magic.resolve(value)

        return wrapper

    @classmethod
    def _new(cls: Type[MagicT], operations: Tuple[BaseOperation, ...]) -> MagicT:
        return cls(operations=operations)

    def _extend(self: MagicT, operation: BaseOperation) -> MagicT:
        return self._new(self._operations + (operation,))

    def _replace_last(self: MagicT, operation: BaseOperation) -> MagicT:
        return self._new(self._operations[:-1] + (operation,))

    def _exclude_last(self: MagicT) -> MagicT:
        return self._new(self._operations[:-1])

    def _resolve(self, value: Any, operations: Optional[Tuple[BaseOperation, ...]] = None) -> Any:
        initial_value = value
        if operations is None:
            operations = self._operations
        rejected = False
        for index, operation in enumerate(operations):
            if rejected and not operation.important:
                continue
            try:
                value = operation.resolve(value=value, initial_value=initial_value)
            except SwitchModeToAll:
                return all(self._resolve(value=item, operations=operations[index + 1 :]) for item in value)
            except SwitchModeToAny:
                return any(self._resolve(value=item, operations=operations[index + 1 :]) for item in value)
            except RejectOperations:
                rejected = True
                value = None
                continue
            rejected = False
        return value

    def __bool__(self) -> bool:
        return True

    def resolve(self: MagicT, value: Any) -> Any:
        return self._resolve(value=value)

    def __getattr__(self: MagicT, item: Any) -> MagicT:
        if item.startswith("_"):
            raise AttributeError(f"{type(self).__name__!r} object has no attribute {item!r}")
        return self._extend(GetAttributeOperation(name=item))

    attr_ = __getattr__

    def __getitem__(self: MagicT, item: Any) -> MagicT:
        if isinstance(item, MagicFilter):
            return self._extend(SelectorOperation(inner=item))
        return self._extend(GetItemOperation(key=item))

    def __len__(self) -> int:
        raise TypeError(f"Length can't be taken using len() function. Use {type(self).__name__}.len() instead.")

    def __eq__(self: MagicT, other: Any) -> MagicT:  # type: ignore
        return self._extend(ComparatorOperation(right=other, comparator=operator.eq))

    def __ne__(self: MagicT, other: Any) -> MagicT:  # type: ignore
        return self._extend(ComparatorOperation(right=other, comparator=operator.ne))

    def __lt__(self: MagicT, other: Any) -> MagicT:
        return self._extend(ComparatorOperation(right=other, comparator=operator.lt))

    def __gt__(self: MagicT, other: Any) -> MagicT:
        return self._extend(ComparatorOperation(right=other, comparator=operator.gt))

    def __le__(self: MagicT, other: Any) -> MagicT:
        return self._extend(ComparatorOperation(right=other, comparator=operator.le))

    def __ge__(self: MagicT, other: Any) -> MagicT:
        return self._extend(ComparatorOperation(right=other, comparator=operator.ge))

    def __invert__(self: MagicT) -> MagicT:
        if (
            self._operations
            and isinstance(self._operations[-1], ImportantFunctionOperation)
            and self._operations[-1].function == operator.not_
        ):
            return self._exclude_last()
        return self._extend(ImportantFunctionOperation(function=operator.not_))

    def __call__(self: MagicT, *args: Any, **kwargs: Any) -> MagicT:
        return self._extend(CallOperation(args=args, kwargs=kwargs))

    def __and__(self: MagicT, other: Any) -> MagicT:
        if isinstance(other, MagicFilter):
            return self._extend(CombinationOperation(right=other, combinator=and_op))
        return self._extend(CombinationOperation(right=other, combinator=operator.and_))

    def __rand__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.and_))

    def __or__(self: MagicT, other: Any) -> MagicT:
        if isinstance(other, MagicFilter):
            return self._extend(ImportantCombinationOperation(right=other, combinator=or_op))
        return self._extend(ImportantCombinationOperation(right=other, combinator=operator.or_))

    def __ror__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.or_))

    def __xor__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.xor))

    def __rxor__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.xor))

    def __rshift__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.rshift))

    def __rrshift__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.rshift))

    def __lshift__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.lshift))

    def __rlshift__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.lshift))

    def __add__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.add))

    def __radd__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.add))

    def __sub__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.sub))

    def __rsub__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.sub))

    def __mul__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.mul))

    def __rmul__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.mul))

    def __truediv__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.truediv))

    def __rtruediv__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.truediv))

    def __floordiv__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.floordiv))

    def __rfloordiv__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.floordiv))

    def __mod__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.mod))

    def __rmod__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.mod))

    def __matmul__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.matmul))

    def __rmatmul__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.matmul))

    def __pow__(self: MagicT, other: Any) -> MagicT:
        return self._extend(CombinationOperation(right=other, combinator=operator.pow))

    def __rpow__(self: MagicT, other: Any) -> MagicT:
        return self._extend(RCombinationOperation(left=other, combinator=operator.pow))

    def __pos__(self: MagicT) -> MagicT:
        return self._extend(FunctionOperation(function=operator.pos))

    def __neg__(self: MagicT) -> MagicT:
        return self._extend(FunctionOperation(function=operator.neg))

    def is_(self: MagicT, value: Any) -> MagicT:
        return self._extend(CombinationOperation(right=value, combinator=operator.is_))

    def is_not(self: MagicT, value: Any) -> MagicT:
        return self._extend(CombinationOperation(right=value, combinator=operator.is_not))

    def in_(self: MagicT, iterable: Union[Container[Any], MagicT]) -> MagicT:
        return self._extend(FunctionOperation(in_op, iterable))

    def not_in(self: MagicT, iterable: Union[Container[Any], MagicT]) -> MagicT:
        return self._extend(FunctionOperation(not_in_op, iterable))

    def contains(self: MagicT, value: Any) -> MagicT:
        return self._extend(FunctionOperation(contains_op, value))

    def not_contains(self: MagicT, value: Any) -> MagicT:
        return self._extend(FunctionOperation(not_contains_op, value))

    def len(self: MagicT) -> MagicT:
        return self._extend(FunctionOperation(len))

    def regexp(
        self: MagicT,
        pattern: Union[str, Pattern[str]],
        *,
        mode: Optional[str] = None,
        search: Optional[bool] = None,
        flags: Union[int, re.RegexFlag] = 0,
    ) -> MagicT:

        if search is not None:
            warn(
                "Param 'search' is deprecated, use 'mode' instead.",
                DeprecationWarning,
            )

            if mode is not None:
                msg = "Can't pass both 'search' and 'mode' params."
                raise ParamsConflict(msg)

            mode = RegexpMode.SEARCH if search else RegexpMode.MATCH

        if mode is None:
            mode = RegexpMode.MATCH

        if isinstance(pattern, str):
            pattern = re.compile(pattern, flags=flags)

        regexp_func = getattr(pattern, mode)
        return self._extend(FunctionOperation(regexp_func))

    def func(self: MagicT, func: Callable[[Any], Any], *args: Any, **kwargs: Any) -> MagicT:
        return self._extend(FunctionOperation(func, *args, **kwargs))

    def cast(self: MagicT, func: Callable[[Any], Any]) -> MagicT:
        return self._extend(CastOperation(func))

    def extract(self: MagicT, magic: "MagicT") -> MagicT:
        return self._extend(ExtractOperation(magic))
