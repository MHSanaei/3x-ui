from enum import Enum


class ChatAction(str, Enum):
    """
    This object represents bot actions.

    Choose one, depending on what the user is about to receive:

    - typing for text messages,
    - upload_photo for photos,
    - record_video or upload_video for videos,
    - record_voice or upload_voice for voice notes,
    - upload_document for general files,
    - choose_sticker for stickers,
    - find_location for location data,
    - record_video_note or upload_video_note for video notes.

    Source: https://core.telegram.org/bots/api#sendchataction
    """

    TYPING = "typing"
    UPLOAD_PHOTO = "upload_photo"
    RECORD_VIDEO = "record_video"
    UPLOAD_VIDEO = "upload_video"
    RECORD_VOICE = "record_voice"
    UPLOAD_VOICE = "upload_voice"
    UPLOAD_DOCUMENT = "upload_document"
    CHOOSE_STICKER = "choose_sticker"
    FIND_LOCATION = "find_location"
    RECORD_VIDEO_NOTE = "record_video_note"
    UPLOAD_VIDEO_NOTE = "upload_video_note"
