class MagicFilterException(Exception):
    pass


class SwitchMode(MagicFilterException):
    pass


class SwitchModeToAll(SwitchMode):
    def __init__(self, key: slice) -> None:
        self.key = key


class SwitchModeToAny(SwitchMode):
    pass


class RejectOperations(MagicFilterException):
    pass


class ParamsConflict(MagicFilterException):
    pass
