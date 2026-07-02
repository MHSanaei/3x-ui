from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .base import TelegramObject

if TYPE_CHECKING:
    from .animation import Animation
    from .audio import Audio
    from .chat import Chat
    from .checklist import Checklist
    from .contact import Contact
    from .dice import Dice
    from .document import Document
    from .game import Game
    from .giveaway import Giveaway
    from .giveaway_winners import GiveawayWinners
    from .invoice import Invoice
    from .link_preview_options import LinkPreviewOptions
    from .live_photo import LivePhoto
    from .location import Location
    from .message_origin_union import MessageOriginUnion
    from .paid_media_info import PaidMediaInfo
    from .photo_size import PhotoSize
    from .poll import Poll
    from .sticker import Sticker
    from .story import Story
    from .venue import Venue
    from .video import Video
    from .video_note import VideoNote
    from .voice import Voice


class ExternalReplyInfo(TelegramObject):
    """
    This object contains information about a message that is being replied to, which may come from another chat or forum topic.

    Source: https://core.telegram.org/bots/api#externalreplyinfo
    """

    origin: MessageOriginUnion
    """Origin of the message replied to by the given message"""
    chat: Chat | None = None
    """*Optional*. Chat the original message belongs to. Available only if the chat is a supergroup or a channel"""
    message_id: int | None = None
    """*Optional*. Unique message identifier inside the original chat. Available only if the original chat is a supergroup or a channel"""
    link_preview_options: LinkPreviewOptions | None = None
    """*Optional*. Options used for link preview generation for the original message, if it is a text message"""
    animation: Animation | None = None
    """*Optional*. Message is an animation, information about the animation"""
    audio: Audio | None = None
    """*Optional*. Message is an audio file, information about the file"""
    document: Document | None = None
    """*Optional*. Message is a general file, information about the file"""
    live_photo: LivePhoto | None = None
    """*Optional*. Message is a live photo, information about the live photo"""
    paid_media: PaidMediaInfo | None = None
    """*Optional*. Message contains paid media; information about the paid media"""
    photo: list[PhotoSize] | None = None
    """*Optional*. Message is a photo, available sizes of the photo"""
    sticker: Sticker | None = None
    """*Optional*. Message is a sticker, information about the sticker"""
    story: Story | None = None
    """*Optional*. Message is a forwarded story"""
    video: Video | None = None
    """*Optional*. Message is a video, information about the video"""
    video_note: VideoNote | None = None
    """*Optional*. Message is a `video note <https://telegram.org/blog/video-messages-and-telescope>`_, information about the video message"""
    voice: Voice | None = None
    """*Optional*. Message is a voice message, information about the file"""
    has_media_spoiler: bool | None = None
    """*Optional*. :code:`True`, if the message media is covered by a spoiler animation"""
    checklist: Checklist | None = None
    """*Optional*. Message is a checklist"""
    contact: Contact | None = None
    """*Optional*. Message is a shared contact, information about the contact"""
    dice: Dice | None = None
    """*Optional*. Message is a dice with random value"""
    game: Game | None = None
    """*Optional*. Message is a game, information about the game. `More about games » <https://core.telegram.org/bots/api#games>`_"""
    giveaway: Giveaway | None = None
    """*Optional*. Message is a scheduled giveaway, information about the giveaway"""
    giveaway_winners: GiveawayWinners | None = None
    """*Optional*. A giveaway with public winners was completed"""
    invoice: Invoice | None = None
    """*Optional*. Message is an invoice for a `payment <https://core.telegram.org/bots/api#payments>`_, information about the invoice. `More about payments » <https://core.telegram.org/bots/api#payments>`_"""
    location: Location | None = None
    """*Optional*. Message is a shared location, information about the location"""
    poll: Poll | None = None
    """*Optional*. Message is a native poll, information about the poll"""
    venue: Venue | None = None
    """*Optional*. Message is a venue, information about the venue"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            origin: MessageOriginUnion,
            chat: Chat | None = None,
            message_id: int | None = None,
            link_preview_options: LinkPreviewOptions | None = None,
            animation: Animation | None = None,
            audio: Audio | None = None,
            document: Document | None = None,
            live_photo: LivePhoto | None = None,
            paid_media: PaidMediaInfo | None = None,
            photo: list[PhotoSize] | None = None,
            sticker: Sticker | None = None,
            story: Story | None = None,
            video: Video | None = None,
            video_note: VideoNote | None = None,
            voice: Voice | None = None,
            has_media_spoiler: bool | None = None,
            checklist: Checklist | None = None,
            contact: Contact | None = None,
            dice: Dice | None = None,
            game: Game | None = None,
            giveaway: Giveaway | None = None,
            giveaway_winners: GiveawayWinners | None = None,
            invoice: Invoice | None = None,
            location: Location | None = None,
            poll: Poll | None = None,
            venue: Venue | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                origin=origin,
                chat=chat,
                message_id=message_id,
                link_preview_options=link_preview_options,
                animation=animation,
                audio=audio,
                document=document,
                live_photo=live_photo,
                paid_media=paid_media,
                photo=photo,
                sticker=sticker,
                story=story,
                video=video,
                video_note=video_note,
                voice=voice,
                has_media_spoiler=has_media_spoiler,
                checklist=checklist,
                contact=contact,
                dice=dice,
                game=game,
                giveaway=giveaway,
                giveaway_winners=giveaway_winners,
                invoice=invoice,
                location=location,
                poll=poll,
                venue=venue,
                **__pydantic_kwargs,
            )
