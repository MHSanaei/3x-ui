from __future__ import annotations

import inspect
from collections import defaultdict
from collections.abc import Mapping
from dataclasses import dataclass, replace
from enum import Enum, auto
from typing import TYPE_CHECKING, Any, ClassVar, overload

from typing_extensions import Self

from aiogram import loggers
from aiogram.dispatcher.dispatcher import Dispatcher
from aiogram.dispatcher.event.handler import CallableObject, CallbackType
from aiogram.dispatcher.flags import extract_flags_from_object
from aiogram.dispatcher.router import Router
from aiogram.exceptions import SceneException
from aiogram.filters import StateFilter
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State
from aiogram.fsm.storage.memory import MemoryStorageRecord
from aiogram.types import TelegramObject, Update
from aiogram.utils.class_attrs_resolver import (
    ClassAttrsResolver,
    get_sorted_mro_attrs_resolver,
)

if TYPE_CHECKING:
    from aiogram.dispatcher.event.bases import NextMiddlewareType


class HistoryManager:
    def __init__(self, state: FSMContext, destiny: str = "scenes_history", size: int = 10):
        self._size = size
        self._state = state
        self._history_state = FSMContext(
            storage=state.storage,
            key=replace(state.key, destiny=destiny),
        )

    async def push(self, state: str | None, data: dict[str, Any]) -> None:
        history_data = await self._history_state.get_data()
        history = history_data.setdefault("history", [])
        history.append({"state": state, "data": data})
        if len(history) > self._size:
            history = history[-self._size :]
        loggers.scene.debug("Push state=%s data=%s to history", state, data)

        await self._history_state.update_data(history=history)

    async def pop(self) -> MemoryStorageRecord | None:
        history_data = await self._history_state.get_data()
        history = history_data.setdefault("history", [])
        if not history:
            return None
        record = history.pop()
        state = record["state"]
        data = record["data"]
        if not history:
            await self._history_state.set_data({})
        else:
            await self._history_state.update_data(history=history)
        loggers.scene.debug("Pop state=%s data=%s from history", state, data)
        return MemoryStorageRecord(state=state, data=data)

    async def get(self) -> MemoryStorageRecord | None:
        history_data = await self._history_state.get_data()
        history = history_data.setdefault("history", [])
        if not history:
            return None
        return MemoryStorageRecord(**history[-1])

    async def all(self) -> list[MemoryStorageRecord]:
        history_data = await self._history_state.get_data()
        history = history_data.setdefault("history", [])
        return [MemoryStorageRecord(**item) for item in history]

    async def clear(self) -> None:
        loggers.scene.debug("Clear history")
        await self._history_state.set_data({})

    async def snapshot(self) -> None:
        state = await self._state.get_state()
        data = await self._state.get_data()
        await self.push(state, data)

    async def _set_state(self, state: str | None, data: dict[str, Any]) -> None:
        await self._state.set_state(state)
        await self._state.set_data(data)

    async def rollback(self) -> str | None:
        previous_state = await self.pop()
        if not previous_state:
            await self._set_state(None, {})
            return None

        loggers.scene.debug(
            "Rollback to state=%s data=%s",
            previous_state.state,
            previous_state.data,
        )
        await self._set_state(previous_state.state, previous_state.data)
        return previous_state.state


class ObserverDecorator:
    def __init__(
        self,
        name: str,
        filters: tuple[CallbackType, ...],
        action: SceneAction | None = None,
        after: After | None = None,
    ) -> None:
        self.name = name
        self.filters = filters
        self.action = action
        self.after = after

    def _wrap_filter(self, target: type[Scene] | CallbackType) -> None:
        handlers = getattr(target, "__aiogram_handler__", None)
        if not handlers:
            handlers = []
            target.__aiogram_handler__ = handlers  # type: ignore[union-attr]

        handlers.append(
            HandlerContainer(
                name=self.name,
                handler=target,
                filters=self.filters,
                after=self.after,
            ),
        )

    def _wrap_action(self, target: CallbackType) -> None:
        assert self.action is not None, "Scene action is not specified"

        action = getattr(target, "__aiogram_action__", None)
        if action is None:
            action = defaultdict(dict)
            target.__aiogram_action__ = action  # type: ignore[attr-defined]
        action[self.action][self.name] = CallableObject(target)

    def __call__(self, target: CallbackType) -> CallbackType:
        if inspect.isfunction(target):
            if self.action is None:
                self._wrap_filter(target)
            else:
                self._wrap_action(target)
        else:
            msg = "Only function or method is allowed"
            raise TypeError(msg)
        return target

    def leave(self) -> ActionContainer:
        return ActionContainer(self.name, self.filters, SceneAction.leave)

    def enter(self, target: type[Scene]) -> ActionContainer:
        return ActionContainer(self.name, self.filters, SceneAction.enter, target)

    def exit(self) -> ActionContainer:
        return ActionContainer(self.name, self.filters, SceneAction.exit)

    def back(self) -> ActionContainer:
        return ActionContainer(self.name, self.filters, SceneAction.back)


class SceneAction(Enum):
    enter = auto()
    leave = auto()
    exit = auto()
    back = auto()


class ActionContainer:
    def __init__(
        self,
        name: str,
        filters: tuple[CallbackType, ...],
        action: SceneAction,
        target: type[Scene] | State | str | None = None,
    ) -> None:
        self.name = name
        self.filters = filters
        self.action = action
        self.target = target

    async def execute(self, wizard: SceneWizard) -> None:
        if self.action == SceneAction.enter and self.target is not None:
            await wizard.goto(self.target)
        elif self.action == SceneAction.leave:
            await wizard.leave()
        elif self.action == SceneAction.exit:
            await wizard.exit()
        elif self.action == SceneAction.back:
            await wizard.back()


@dataclass(slots=True)
class HandlerContainer:
    name: str
    handler: CallbackType
    filters: tuple[CallbackType, ...]
    after: After | None = None


@dataclass
class SceneConfig:
    state: str | None
    """Scene state"""
    handlers: list[HandlerContainer]
    """Scene handlers"""
    actions: dict[SceneAction, dict[str, CallableObject]]
    """Scene actions"""
    reset_data_on_enter: bool | None = None
    """Reset scene data on enter"""
    reset_history_on_enter: bool | None = None
    """Reset scene history on enter"""
    callback_query_without_state: bool | None = None
    """Allow callback query without state"""
    attrs_resolver: ClassAttrsResolver = get_sorted_mro_attrs_resolver
    """
    Attributes resolver.

    .. danger::
        This attribute should only be changed when you know what you are doing.

    .. versionadded:: 3.19.0
    """


async def _empty_handler(*args: Any, **kwargs: Any) -> None:
    pass


class SceneHandlerWrapper:
    def __init__(
        self,
        scene: type[Scene],
        handler: CallbackType,
        after: After | None = None,
    ) -> None:
        self.scene = scene
        self.handler = CallableObject(handler)
        self.after = after

    async def __call__(
        self,
        event: TelegramObject,
        **kwargs: Any,
    ) -> Any:
        try:
            state: FSMContext = kwargs["state"]
            scenes: ScenesManager = kwargs["scenes"]
        except KeyError as error:
            missing_key = error.args[0]
            msg = (
                f"Scene context key {missing_key!r} is not available. "
                "Ensure FSM is enabled and pipeline is intact."
            )
            raise SceneException(msg) from None
        event_update: Update = kwargs["event_update"]
        scenes.data = {**scenes.data, **kwargs}
        scene = self.scene(
            wizard=SceneWizard(
                scene_config=self.scene.__scene_config__,
                manager=scenes,
                state=state,
                update_type=event_update.event_type,
                event=event,
                data=kwargs,
            ),
        )

        result = await self.handler.call(scene, event, **kwargs)

        if self.after:
            action_container = ActionContainer(
                "after",
                (),
                self.after.action,
                self.after.scene,
            )
            await action_container.execute(scene.wizard)
        return result

    def __await__(self) -> Self:
        return self

    def __str__(self) -> str:
        result = f"SceneHandlerWrapper({self.scene}, {self.handler.callback}"
        if self.after:
            result += f", after={self.after}"
        result += ")"
        return result


class Scene:
    """
    Represents a scene in a conversation flow.

    A scene is a specific state in a conversation where certain actions can take place.

    Each scene has a set of filters that determine when it should be triggered,
    and a set of handlers that define the actions to be executed when the scene is active.

    .. note::
        This class is not meant to be used directly. Instead, it should be subclassed
        to define custom scenes.
    """

    __scene_config__: ClassVar[SceneConfig]
    """Scene configuration."""

    def __init__(
        self,
        wizard: SceneWizard,
    ) -> None:
        self.wizard = wizard
        self.wizard.scene = self

    def __init_subclass__(cls, **kwargs: Any) -> None:
        state_name = kwargs.pop("state", None)
        reset_data_on_enter = kwargs.pop("reset_data_on_enter", None)
        reset_history_on_enter = kwargs.pop("reset_history_on_enter", None)
        callback_query_without_state = kwargs.pop("callback_query_without_state", None)
        attrs_resolver = kwargs.pop("attrs_resolver", None)

        super().__init_subclass__(**kwargs)

        handlers: list[HandlerContainer] = []
        actions: defaultdict[SceneAction, dict[str, CallableObject]] = defaultdict(dict)

        for base in cls.__bases__:
            if not issubclass(base, Scene):
                continue

            parent_scene_config = getattr(base, "__scene_config__", None)
            if not parent_scene_config:
                continue

            if reset_data_on_enter is None:
                reset_data_on_enter = parent_scene_config.reset_data_on_enter
            if reset_history_on_enter is None:
                reset_history_on_enter = parent_scene_config.reset_history_on_enter
            if callback_query_without_state is None:
                callback_query_without_state = parent_scene_config.callback_query_without_state
            if attrs_resolver is None:
                attrs_resolver = parent_scene_config.attrs_resolver

        if attrs_resolver is None:
            attrs_resolver = get_sorted_mro_attrs_resolver

        for _name, value in attrs_resolver(cls):
            if scene_handlers := getattr(value, "__aiogram_handler__", None):
                handlers.extend(scene_handlers)
            if isinstance(value, ObserverDecorator):
                handlers.append(
                    HandlerContainer(
                        value.name,
                        _empty_handler,
                        value.filters,
                        after=value.after,
                    ),
                )
            if hasattr(value, "__aiogram_action__"):
                for action, action_handlers in value.__aiogram_action__.items():
                    actions[action].update(action_handlers)

        cls.__scene_config__ = SceneConfig(
            state=state_name,
            handlers=handlers,
            actions=dict(actions),
            reset_data_on_enter=reset_data_on_enter,
            reset_history_on_enter=reset_history_on_enter,
            callback_query_without_state=callback_query_without_state,
            attrs_resolver=attrs_resolver,
        )

    @classmethod
    def add_to_router(cls, router: Router) -> None:
        """
        Adds the scene to the given router.

        :param router:
        :return:
        """
        scene_config = cls.__scene_config__
        used_observers = set()

        for handler in scene_config.handlers:
            router.observers[handler.name].register(
                SceneHandlerWrapper(
                    cls,
                    handler.handler,
                    after=handler.after,
                ),
                *handler.filters,
                flags=extract_flags_from_object(handler.handler),
            )
            used_observers.add(handler.name)

        for observer_name in used_observers:
            if scene_config.callback_query_without_state and observer_name == "callback_query":
                continue
            router.observers[observer_name].filter(StateFilter(scene_config.state))

    @classmethod
    def as_router(cls, name: str | None = None) -> Router:
        """
        Returns the scene as a router.

        :return: new router
        """
        if name is None:
            name = (
                f"Scene '{cls.__module__}.{cls.__qualname__}' "
                f"for state {cls.__scene_config__.state!r}"
            )
        router = Router(name=name)
        cls.add_to_router(router)
        return router

    @classmethod
    def as_handler(cls, **handler_kwargs: Any) -> CallbackType:
        """
        Create an entry point handler for the scene, can be used to simplify the handler
        that starts the scene.

        >>> router.message.register(MyScene.as_handler(), Command("start"))
        """

        async def enter_to_scene_handler(
            event: TelegramObject,
            scenes: ScenesManager,
            **middleware_kwargs: Any,
        ) -> None:
            await scenes.enter(cls, **{**handler_kwargs, **middleware_kwargs})

        return enter_to_scene_handler


class SceneWizard:
    """
    A class that represents a wizard for managing scenes in a Telegram bot.

    Instance of this class is passed to each scene as a parameter.
    So, you can use it to transition between scenes, get and set data, etc.

    .. note::

        This class is not meant to be used directly. Instead, it should be used
        as a parameter in the scene constructor.

    """

    def __init__(
        self,
        scene_config: SceneConfig,
        manager: ScenesManager,
        state: FSMContext,
        update_type: str,
        event: TelegramObject,
        data: dict[str, Any],
    ):
        """
        A class that represents a wizard for managing scenes in a Telegram bot.

        :param scene_config: The configuration of the scene.
        :param manager: The scene manager.
        :param state: The FSMContext object for storing the state of the scene.
        :param update_type: The type of the update event.
        :param event: The TelegramObject represents the event.
        :param data: Additional data for the scene.
        """
        self.scene_config = scene_config
        self.manager = manager
        self.state = state
        self.update_type = update_type
        self.event = event
        self.data = data

        self.scene: Scene | None = None

    async def enter(self, **kwargs: Any) -> None:
        """
        Enter method is used to transition into a scene in the SceneWizard class.
        It sets the state, clears data and history if specified,
        and triggers entering event of the scene.

        :param kwargs: Additional keyword arguments.
        :return: None
        """
        loggers.scene.debug("Entering scene %r", self.scene_config.state)
        if self.scene_config.reset_data_on_enter:
            await self.state.set_data({})
        if self.scene_config.reset_history_on_enter:
            await self.manager.history.clear()
        await self.state.set_state(self.scene_config.state)
        await self._on_action(SceneAction.enter, **kwargs)

    async def leave(self, _with_history: bool = True, **kwargs: Any) -> None:
        """
        Leaves the current scene.
        This method is used to exit a scene and transition to the next scene.

        :param _with_history: Whether to include history in the snapshot. Defaults to True.
        :param kwargs: Additional keyword arguments.
        :return: None

        """
        loggers.scene.debug("Leaving scene %r", self.scene_config.state)
        if _with_history:
            await self.manager.history.snapshot()
        await self._on_action(SceneAction.leave, **kwargs)

    async def exit(self, **kwargs: Any) -> None:
        """
        Exit the current scene and enter the default scene/state.

        :param kwargs: Additional keyword arguments.
        :return: None
        """
        loggers.scene.debug("Exiting scene %r", self.scene_config.state)
        await self.manager.history.clear()
        await self._on_action(SceneAction.exit, **kwargs)
        await self.manager.enter(None, _check_active=False, **kwargs)

    async def back(self, **kwargs: Any) -> None:
        """
        This method is used to go back to the previous scene.

        :param kwargs: Keyword arguments that can be passed to the method.
        :return: None
        """
        loggers.scene.debug("Back to previous scene from scene %s", self.scene_config.state)
        await self.leave(_with_history=False, **kwargs)
        new_scene = await self.manager.history.rollback()
        await self.manager.enter(new_scene, _check_active=False, **kwargs)

    async def retake(self, **kwargs: Any) -> None:
        """
        This method allows to re-enter the current scene.

        :param kwargs: Additional keyword arguments to pass to the scene.
        :return: None
        """
        assert self.scene_config.state is not None, "Scene state is not specified"
        await self.goto(self.scene_config.state, **kwargs)

    async def goto(self, scene: type[Scene] | State | str, **kwargs: Any) -> None:
        """
        The `goto` method transitions to a new scene.
        It first calls the `leave` method to perform any necessary cleanup
        in the current scene, then calls the `enter` event to enter the specified scene.

        :param scene: The scene to transition to. Can be either a `Scene` instance
            `State` instance or a string representing the scene.
        :param kwargs: Additional keyword arguments to pass to the `enter`
            method of the scene manager.
        :return: None
        """
        await self.leave(**kwargs)
        await self.manager.enter(scene, _check_active=False, **kwargs)

    async def _on_action(self, action: SceneAction, **kwargs: Any) -> bool:
        if not self.scene:
            msg = "Scene is not initialized"
            raise SceneException(msg)

        loggers.scene.debug("Call action %r in scene %r", action.name, self.scene_config.state)
        action_config = self.scene_config.actions.get(action, {})
        if not action_config:
            loggers.scene.debug(
                "Action %r not found in scene %r",
                action.name,
                self.scene_config.state,
            )
            return False

        event_type = self.update_type
        if event_type not in action_config:
            loggers.scene.debug(
                "Action %r for event %r not found in scene %r",
                action.name,
                event_type,
                self.scene_config.state,
            )
            return False

        await action_config[event_type].call(self.scene, self.event, **{**self.data, **kwargs})
        return True

    async def set_data(self, data: Mapping[str, Any]) -> None:
        """
        Sets custom data in the current state.

        :param data: A mapping containing the custom data to be set in the current state.
        :return: None
        """
        await self.state.set_data(data=data)

    async def get_data(self) -> dict[str, Any]:
        """
        This method returns the data stored in the current state.

        :return: A dictionary containing the data stored in the scene state.
        """
        return await self.state.get_data()

    @overload
    async def get_value(self, key: str) -> Any | None:
        """
        This method returns the value from key in the data of the current state.

        :param key: The keyname of the item you want to return the value from.

        :return: A dictionary containing the data stored in the scene state.
        """

    @overload
    async def get_value(self, key: str, default: Any) -> Any:
        """
        This method returns the value from key in the data of the current state.

        :param key: The keyname of the item you want to return the value from.
        :param default: Default value to return, if ``key`` was not found.

        :return: A dictionary containing the data stored in the scene state.
        """

    async def get_value(self, key: str, default: Any | None = None) -> Any | None:
        return await self.state.get_value(key, default)

    async def update_data(
        self,
        data: Mapping[str, Any] | None = None,
        **kwargs: Any,
    ) -> dict[str, Any]:
        """
        This method updates the data stored in the current state

        :param data: Optional mapping of data to update.
        :param kwargs: Additional key-value pairs of data to update.
        :return: Dictionary of updated data
        """
        if data:
            kwargs.update(data)
        return await self.state.update_data(data=kwargs)

    async def clear_data(self) -> None:
        """
        Clears the data.

        :return: None
        """
        await self.set_data({})


class ScenesManager:
    """
    The ScenesManager class is responsible for managing scenes in an application.
    It provides methods for entering and exiting scenes, as well as retrieving the active scene.
    """

    def __init__(
        self,
        registry: SceneRegistry,
        update_type: str,
        event: TelegramObject,
        state: FSMContext,
        data: dict[str, Any],
    ) -> None:
        self.registry = registry
        self.update_type = update_type
        self.event = event
        self.state = state
        self.data = data

        self.history = HistoryManager(self.state)

    async def _get_scene(self, scene_type: type[Scene] | State | str | None) -> Scene:
        scene_type = self.registry.get(scene_type)
        return scene_type(
            wizard=SceneWizard(
                scene_config=scene_type.__scene_config__,
                manager=self,
                state=self.state,
                update_type=self.update_type,
                event=self.event,
                data=self.data,
            ),
        )

    async def _get_active_scene(self) -> Scene | None:
        state = await self.state.get_state()
        try:
            return await self._get_scene(state)
        except SceneException:
            return None

    async def enter(
        self,
        scene_type: type[Scene] | State | str | None,
        _check_active: bool = True,
        **kwargs: Any,
    ) -> None:
        """
        Enters the specified scene.

        :param scene_type: Optional Type[Scene], State or str representing the scene type to enter.
        :param _check_active: Optional bool indicating whether to check if
            there is an active scene to exit before entering the new scene. Defaults to True.
        :param kwargs: Additional keyword arguments to pass to the scene's wizard.enter() method.
        :return: None
        """
        if kwargs:
            self.data = {**self.data, **kwargs}

        if _check_active:
            active_scene = await self._get_active_scene()
            if active_scene is not None:
                await active_scene.wizard.exit(**kwargs)

        try:
            scene = await self._get_scene(scene_type)
        except SceneException:
            if scene_type is not None:
                raise
            await self.state.set_state(None)
        else:
            await scene.wizard.enter(**kwargs)

    async def close(self, **kwargs: Any) -> None:
        """
        Close method is used to exit the currently active scene in the ScenesManager.

        :param kwargs: Additional keyword arguments passed to the scene's exit method.
        :return: None
        """
        scene = await self._get_active_scene()
        if not scene:
            return
        await scene.wizard.exit(**kwargs)


class SceneRegistry:
    """
    A class that represents a registry for scenes in a Telegram bot.
    """

    def __init__(self, router: Router, register_on_add: bool = True) -> None:
        """
        Initialize a new instance of the SceneRegistry class.

        :param router: The router instance used for scene registration.
        :param register_on_add: Whether to register the scenes to the router when they are added.
        """
        self.router = router
        self.register_on_add = register_on_add

        self._scenes: dict[str | None, type[Scene]] = {}
        self._setup_middleware(router)

    def _setup_middleware(self, router: Router) -> None:
        if isinstance(router, Dispatcher):
            # Small optimization for Dispatcher
            # - we don't need to set up middleware for all observers
            router.update.outer_middleware(self._update_middleware)
            return

        for observer in router.observers.values():
            if observer.event_name in {"update", "error"}:
                continue
            observer.outer_middleware(self._middleware)

    async def _update_middleware(
        self,
        handler: NextMiddlewareType[TelegramObject],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        assert isinstance(event, Update), "Event must be an Update instance"
        state = data.get("state")
        if state is None:
            return await handler(event, data)

        data["scenes"] = ScenesManager(
            registry=self,
            update_type=event.event_type,
            event=event.event,
            state=state,
            data=data,
        )
        return await handler(event, data)

    async def _middleware(
        self,
        handler: NextMiddlewareType[TelegramObject],
        event: TelegramObject,
        data: dict[str, Any],
    ) -> Any:
        state = data.get("state")
        if state is None:
            return await handler(event, data)

        update: Update = data["event_update"]
        data["scenes"] = ScenesManager(
            registry=self,
            update_type=update.event_type,
            event=event,
            state=state,
            data=data,
        )
        return await handler(event, data)

    def add(self, *scenes: type[Scene], router: Router | None = None) -> None:
        """
        This method adds the specified scenes to the registry
        and optionally registers it to the router.

        If a scene with the same state already exists in the registry, a SceneException is raised.

        .. warning::

            If the router is not specified, the scenes will not be registered to the router.
            You will need to include the scenes manually to the router or use the register method.

        :param scenes: A variable length parameter that accepts one or more types of scenes.
            These scenes are instances of the Scene class.
        :param router: An optional parameter that specifies the router
            to which the scenes should be added.
        :return: None
        """
        if not scenes:
            msg = "At least one scene must be specified"
            raise ValueError(msg)

        for scene in scenes:
            if scene.__scene_config__.state in self._scenes:
                msg = f"Scene with state {scene.__scene_config__.state!r} already exists"
                raise SceneException(msg)

            self._scenes[scene.__scene_config__.state] = scene

            if router:
                router.include_router(scene.as_router())
            elif self.register_on_add:
                self.router.include_router(scene.as_router())

    def register(self, *scenes: type[Scene]) -> None:
        """
        Registers one or more scenes to the SceneRegistry.

        :param scenes: One or more scene classes to register.
        :return: None
        """
        self.add(*scenes, router=self.router)

    def get(self, scene: type[Scene] | State | str | None) -> type[Scene]:
        """
        This method returns the registered Scene object for the specified scene.
        The scene parameter can be either a Scene object, State object or a string representing
        the name of the scene. If a Scene object is provided, the state attribute
        of the SceneConfig object associated with the Scene object will be used as the scene name.
        If a State object is provided, the state attribute of the State object will be used as the
        scene name. If None or an invalid type is provided, a SceneException will be raised.

        If the specified scene is not registered in the SceneRegistry object,
        a SceneException will be raised.

        :param scene: A Scene object, State object or a string representing the name of the scene.
        :return: The registered Scene object corresponding to the given scene parameter.

        """
        if inspect.isclass(scene) and issubclass(scene, Scene):
            scene = scene.__scene_config__.state
        if isinstance(scene, State):
            scene = scene.state
        if scene is not None and not isinstance(scene, str):
            msg = "Scene must be a subclass of Scene, State or a string"
            raise SceneException(msg)

        try:
            return self._scenes[scene]
        except KeyError:
            msg = f"Scene {scene!r} is not registered"
            raise SceneException(msg) from None


@dataclass
class After:
    action: SceneAction
    scene: type[Scene] | State | str | None = None

    @classmethod
    def exit(cls) -> After:
        return cls(action=SceneAction.exit)

    @classmethod
    def back(cls) -> After:
        return cls(action=SceneAction.back)

    @classmethod
    def goto(cls, scene: type[Scene] | State | str | None) -> After:
        return cls(action=SceneAction.enter, scene=scene)


class ObserverMarker:
    def __init__(self, name: str) -> None:
        self.name = name

    def __call__(
        self,
        *filters: CallbackType,
        after: After | None = None,
    ) -> ObserverDecorator:
        return ObserverDecorator(
            self.name,
            filters,
            after=after,
        )

    def enter(self, *filters: CallbackType) -> ObserverDecorator:
        return ObserverDecorator(self.name, filters, action=SceneAction.enter)

    def leave(self) -> ObserverDecorator:
        return ObserverDecorator(self.name, (), action=SceneAction.leave)

    def exit(self) -> ObserverDecorator:
        return ObserverDecorator(self.name, (), action=SceneAction.exit)

    def back(self) -> ObserverDecorator:
        return ObserverDecorator(self.name, (), action=SceneAction.back)


class OnMarker:
    """
    The `OnMarker` class is used as a marker class to define different
    types of events in the Scenes.

    Attributes:

    - :code:`message`: Event marker for handling `Message` events.
    - :code:`edited_message`: Event marker for handling edited `Message` events.
    - :code:`channel_post`: Event marker for handling channel `Post` events.
    - :code:`edited_channel_post`: Event marker for handling edited channel `Post` events.
    - :code:`inline_query`: Event marker for handling `InlineQuery` events.
    - :code:`chosen_inline_result`: Event marker for handling chosen `InlineResult` events.
    - :code:`callback_query`: Event marker for handling `CallbackQuery` events.
    - :code:`shipping_query`: Event marker for handling `ShippingQuery` events.
    - :code:`pre_checkout_query`: Event marker for handling `PreCheckoutQuery` events.
    - :code:`poll`: Event marker for handling `Poll` events.
    - :code:`poll_answer`: Event marker for handling `PollAnswer` events.
    - :code:`my_chat_member`: Event marker for handling my chat `Member` events.
    - :code:`chat_member`: Event marker for handling chat `Member` events.
    - :code:`chat_join_request`: Event marker for handling chat `JoinRequest` events.
    - :code:`error`: Event marker for handling `Error` events.

    .. note::

        This is a marker class and does not contain any methods or implementation logic.
    """

    message = ObserverMarker("message")
    edited_message = ObserverMarker("edited_message")
    channel_post = ObserverMarker("channel_post")
    edited_channel_post = ObserverMarker("edited_channel_post")
    inline_query = ObserverMarker("inline_query")
    chosen_inline_result = ObserverMarker("chosen_inline_result")
    callback_query = ObserverMarker("callback_query")
    shipping_query = ObserverMarker("shipping_query")
    pre_checkout_query = ObserverMarker("pre_checkout_query")
    poll = ObserverMarker("poll")
    poll_answer = ObserverMarker("poll_answer")
    my_chat_member = ObserverMarker("my_chat_member")
    chat_member = ObserverMarker("chat_member")
    chat_join_request = ObserverMarker("chat_join_request")


on = OnMarker()
