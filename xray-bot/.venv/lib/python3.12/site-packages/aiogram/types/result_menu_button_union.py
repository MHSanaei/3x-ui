from typing import TypeAlias

from .menu_button_commands import MenuButtonCommands
from .menu_button_default import MenuButtonDefault
from .menu_button_web_app import MenuButtonWebApp

ResultMenuButtonUnion: TypeAlias = MenuButtonDefault | MenuButtonWebApp | MenuButtonCommands
