from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from ..methods import DeleteStickerFromSet, SetStickerPositionInSet
    from .file import File
    from .mask_position import MaskPosition
    from .photo_size import PhotoSize


class Sticker(TelegramObject):
    """
    This object represents a sticker.

    Source: https://core.telegram.org/bots/api#sticker
    """

    file_id: str
    """Identifier for this file, which can be used to download or reuse the file"""
    file_unique_id: str
    """Unique identifier for this file, which is supposed to be the same over time and for different bots. Can't be used to download or reuse the file"""
    type: str
    """Type of the sticker, currently one of 'regular', 'mask', 'custom_emoji'. The type of the sticker is independent from its format, which is determined by the fields *is_animated* and *is_video*"""
    width: int
    """Sticker width"""
    height: int
    """Sticker height"""
    is_animated: bool
    """:code:`True`, if the sticker is `animated <https://telegram.org/blog/animated-stickers>`_"""
    is_video: bool
    """:code:`True`, if the sticker is a `video sticker <https://telegram.org/blog/video-stickers-better-reactions>`_"""
    thumbnail: PhotoSize | None = None
    """*Optional*. Sticker thumbnail in the .WEBP or .JPG format"""
    emoji: str | None = None
    """*Optional*. Emoji associated with the sticker"""
    set_name: str | None = None
    """*Optional*. Name of the sticker set to which the sticker belongs"""
    premium_animation: File | None = None
    """*Optional*. For premium regular stickers, premium animation for the sticker"""
    mask_position: MaskPosition | None = None
    """*Optional*. For mask stickers, the position where the mask should be placed"""
    custom_emoji_id: str | None = None
    """*Optional*. For custom emoji stickers, unique identifier of the custom emoji"""
    needs_repainting: bool | None = None
    """*Optional*. :code:`True`, if the sticker must be repainted to a text color in messages, the color of the Telegram Premium badge in emoji status, white color on chat photos, or another appropriate color in other places"""
    file_size: int | None = None
    """*Optional*. File size in bytes"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            file_id: str,
            file_unique_id: str,
            type: str,
            width: int,
            height: int,
            is_animated: bool,
            is_video: bool,
            thumbnail: PhotoSize | None = None,
            emoji: str | None = None,
            set_name: str | None = None,
            premium_animation: File | None = None,
            mask_position: MaskPosition | None = None,
            custom_emoji_id: str | None = None,
            needs_repainting: bool | None = None,
            file_size: int | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                file_id=file_id,
                file_unique_id=file_unique_id,
                type=type,
                width=width,
                height=height,
                is_animated=is_animated,
                is_video=is_video,
                thumbnail=thumbnail,
                emoji=emoji,
                set_name=set_name,
                premium_animation=premium_animation,
                mask_position=mask_position,
                custom_emoji_id=custom_emoji_id,
                needs_repainting=needs_repainting,
                file_size=file_size,
                **__pydantic_kwargs,
            )

    def set_position_in_set(
        self,
        position: int,
        **kwargs: Any,
    ) -> SetStickerPositionInSet:
        """
        Shortcut for method :class:`aiogram.methods.set_sticker_position_in_set.SetStickerPositionInSet`
        will automatically fill method attributes:

        - :code:`sticker`

        Use this method to move a sticker in a set created by the bot to a specific position. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickerpositioninset

        :param position: New sticker position in the set, zero-based
        :return: instance of method :class:`aiogram.methods.set_sticker_position_in_set.SetStickerPositionInSet`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SetStickerPositionInSet

        return SetStickerPositionInSet(
            sticker=self.file_id,
            position=position,
            **kwargs,
        ).as_(self._bot)

    def delete_from_set(
        self,
        **kwargs: Any,
    ) -> DeleteStickerFromSet:
        """
        Shortcut for method :class:`aiogram.methods.delete_sticker_from_set.DeleteStickerFromSet`
        will automatically fill method attributes:

        - :code:`sticker`

        Use this method to delete a sticker from a set created by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletestickerfromset

        :return: instance of method :class:`aiogram.methods.delete_sticker_from_set.DeleteStickerFromSet`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import DeleteStickerFromSet

        return DeleteStickerFromSet(
            sticker=self.file_id,
            **kwargs,
        ).as_(self._bot)
