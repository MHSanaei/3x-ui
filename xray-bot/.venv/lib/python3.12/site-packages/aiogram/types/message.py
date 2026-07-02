from __future__ import annotations

from typing import TYPE_CHECKING, Any

from pydantic import Field

from aiogram.utils.text_decorations import (
    TextDecoration,
    html_decoration,
    markdown_decoration,
)

from ..client.default import Default
from ..enums import ContentType
from .custom import DateTime
from .maybe_inaccessible_message import MaybeInaccessibleMessage
from .reply_parameters import ReplyParameters

if TYPE_CHECKING:
    from ..methods import (
        AnswerGuestQuery,
        CopyMessage,
        DeleteMessage,
        EditMessageCaption,
        EditMessageLiveLocation,
        EditMessageMedia,
        EditMessageReplyMarkup,
        EditMessageText,
        ForwardMessage,
        PinChatMessage,
        SendAnimation,
        SendAudio,
        SendContact,
        SendDice,
        SendDocument,
        SendGame,
        SendInvoice,
        SendLocation,
        SendMediaGroup,
        SendMessage,
        SendPaidMedia,
        SendPhoto,
        SendPoll,
        SendRichMessage,
        SendRichMessageDraft,
        SendSticker,
        SendVenue,
        SendVideo,
        SendVideoNote,
        SendVoice,
        SetMessageReaction,
        StopMessageLiveLocation,
        UnpinChatMessage,
    )
    from .animation import Animation
    from .audio import Audio
    from .chat import Chat
    from .chat_background import ChatBackground
    from .chat_boost_added import ChatBoostAdded
    from .chat_id_union import ChatIdUnion
    from .chat_owner_changed import ChatOwnerChanged
    from .chat_owner_left import ChatOwnerLeft
    from .chat_shared import ChatShared
    from .checklist import Checklist
    from .checklist_tasks_added import ChecklistTasksAdded
    from .checklist_tasks_done import ChecklistTasksDone
    from .contact import Contact
    from .date_time_union import DateTimeUnion
    from .dice import Dice
    from .direct_message_price_changed import DirectMessagePriceChanged
    from .direct_messages_topic import DirectMessagesTopic
    from .document import Document
    from .external_reply_info import ExternalReplyInfo
    from .forum_topic_closed import ForumTopicClosed
    from .forum_topic_created import ForumTopicCreated
    from .forum_topic_edited import ForumTopicEdited
    from .forum_topic_reopened import ForumTopicReopened
    from .game import Game
    from .general_forum_topic_hidden import GeneralForumTopicHidden
    from .general_forum_topic_unhidden import GeneralForumTopicUnhidden
    from .gift_info import GiftInfo
    from .giveaway import Giveaway
    from .giveaway_completed import GiveawayCompleted
    from .giveaway_created import GiveawayCreated
    from .giveaway_winners import GiveawayWinners
    from .inline_keyboard_markup import InlineKeyboardMarkup
    from .inline_query_result_union import InlineQueryResultUnion
    from .input_file import InputFile
    from .input_file_union import InputFileUnion
    from .input_media_union import InputMediaUnion
    from .input_paid_media_union import InputPaidMediaUnion
    from .input_poll_media_union import InputPollMediaUnion
    from .input_poll_option_union import InputPollOptionUnion
    from .input_rich_message import InputRichMessage
    from .invoice import Invoice
    from .labeled_price import LabeledPrice
    from .link_preview_options import LinkPreviewOptions
    from .live_photo import LivePhoto
    from .location import Location
    from .managed_bot_created import ManagedBotCreated
    from .maybe_inaccessible_message_union import MaybeInaccessibleMessageUnion
    from .media_union import MediaUnion
    from .message_auto_delete_timer_changed import MessageAutoDeleteTimerChanged
    from .message_entity import MessageEntity
    from .message_origin_union import MessageOriginUnion
    from .paid_media_info import PaidMediaInfo
    from .paid_message_price_changed import PaidMessagePriceChanged
    from .passport_data import PassportData
    from .photo_size import PhotoSize
    from .poll import Poll
    from .poll_option_added import PollOptionAdded
    from .poll_option_deleted import PollOptionDeleted
    from .proximity_alert_triggered import ProximityAlertTriggered
    from .reaction_type_union import ReactionTypeUnion
    from .refunded_payment import RefundedPayment
    from .reply_keyboard_markup import ReplyKeyboardMarkup
    from .reply_markup_union import ReplyMarkupUnion
    from .rich_message import RichMessage
    from .sticker import Sticker
    from .story import Story
    from .successful_payment import SuccessfulPayment
    from .suggested_post_approval_failed import SuggestedPostApprovalFailed
    from .suggested_post_approved import SuggestedPostApproved
    from .suggested_post_declined import SuggestedPostDeclined
    from .suggested_post_info import SuggestedPostInfo
    from .suggested_post_paid import SuggestedPostPaid
    from .suggested_post_parameters import SuggestedPostParameters
    from .suggested_post_refunded import SuggestedPostRefunded
    from .text_quote import TextQuote
    from .unique_gift_info import UniqueGiftInfo
    from .user import User
    from .user_shared import UserShared
    from .users_shared import UsersShared
    from .venue import Venue
    from .video import Video
    from .video_chat_ended import VideoChatEnded
    from .video_chat_participants_invited import VideoChatParticipantsInvited
    from .video_chat_scheduled import VideoChatScheduled
    from .video_chat_started import VideoChatStarted
    from .video_note import VideoNote
    from .voice import Voice
    from .web_app_data import WebAppData
    from .write_access_allowed import WriteAccessAllowed


class Message(MaybeInaccessibleMessage):
    """
    This object represents a message.

    Source: https://core.telegram.org/bots/api#message
    """

    message_id: int
    """Unique message identifier inside this chat. In specific instances (e.g., message containing a video sent to a big chat), the server might automatically schedule a message instead of sending it immediately. In such cases, this field will be 0 and the relevant message will be unusable until it is actually sent"""
    date: DateTime
    """Date the message was sent in Unix time. It is always a positive number, representing a valid date"""
    chat: Chat
    """Chat the message belongs to"""
    message_thread_id: int | None = None
    """*Optional*. Unique identifier of a message thread or forum topic to which the message belongs; for supergroups and private chats only"""
    direct_messages_topic: DirectMessagesTopic | None = None
    """*Optional*. Information about the direct messages chat topic that contains the message"""
    from_user: User | None = Field(None, alias="from")
    """*Optional*. Sender of the message; may be empty for messages sent to channels. For backward compatibility, if the message was sent on behalf of a chat, the field contains a fake sender user in non-channel chats"""
    sender_chat: Chat | None = None
    """*Optional*. Sender of the message when sent on behalf of a chat. For example, the supergroup itself for messages sent by its anonymous administrators or a linked channel for messages automatically forwarded to the channel's discussion group. For backward compatibility, if the message was sent on behalf of a chat, the field *from* contains a fake sender user in non-channel chats"""
    sender_boost_count: int | None = None
    """*Optional*. If the sender of the message boosted the chat, the number of boosts added by the user"""
    sender_business_bot: User | None = None
    """*Optional*. The bot that actually sent the message on behalf of the business account. Available only for outgoing messages sent on behalf of the connected business account"""
    sender_tag: str | None = None
    """*Optional*. Tag or custom title of the sender of the message; for supergroups only"""
    guest_query_id: str | None = None
    """*Optional*. The unique identifier for the guest query. Use this identifier with the method :class:`aiogram.methods.answer_guest_query.AnswerGuestQuery` to send a response message. If non-empty, the message belongs to the chat where the guest bot was summoned, which may not coincide with other existing bot chats sharing the same identifier"""
    business_connection_id: str | None = None
    """*Optional*. Unique identifier of the business connection from which the message was received. If non-empty, the message belongs to a chat of the corresponding business account that is independent from any potential bot chat which might share the same identifier"""
    forward_origin: MessageOriginUnion | None = None
    """*Optional*. Information about the original message for forwarded messages"""
    is_topic_message: bool | None = None
    """*Optional*. :code:`True`, if the message is sent to a topic in a forum supergroup or a private chat with the bot"""
    is_automatic_forward: bool | None = None
    """*Optional*. :code:`True`, if the message is a channel post that was automatically forwarded to the connected discussion group"""
    reply_to_message: Message | None = None
    """*Optional*. For replies in the same chat and message thread, the original message. Note that the :class:`aiogram.types.message.Message` object in this field will not contain further *reply_to_message* fields even if it itself is a reply"""
    external_reply: ExternalReplyInfo | None = None
    """*Optional*. Information about the message that is being replied to, which may come from another chat or forum topic"""
    quote: TextQuote | None = None
    """*Optional*. For replies that quote part of the original message, the quoted part of the message"""
    reply_to_story: Story | None = None
    """*Optional*. For replies to a story, the original story"""
    reply_to_checklist_task_id: int | None = None
    """*Optional*. Identifier of the specific checklist task that is being replied to"""
    reply_to_poll_option_id: str | None = None
    """*Optional*. Persistent identifier of the specific poll option that is being replied to"""
    via_bot: User | None = None
    """*Optional*. Bot through which the message was sent"""
    guest_bot_caller_user: User | None = None
    """*Optional*. For a message sent by a guest bot, this is the user whose original message triggered the bot's response"""
    guest_bot_caller_chat: Chat | None = None
    """*Optional*. For a message sent by a guest bot, this is the chat whose original message triggered the bot's response"""
    edit_date: int | None = None
    """*Optional*. Date the message was last edited in Unix time"""
    has_protected_content: bool | None = None
    """*Optional*. :code:`True`, if the message can't be forwarded"""
    is_from_offline: bool | None = None
    """*Optional*. :code:`True`, if the message was sent by an implicit action, for example, as an away or a greeting business message, or as a scheduled message"""
    is_paid_post: bool | None = None
    """*Optional*. :code:`True`, if the message is a paid post. Note that such posts must not be deleted for 24 hours to receive the payment and can't be edited"""
    media_group_id: str | None = None
    """*Optional*. The unique identifier inside this chat of a media message group this message belongs to"""
    author_signature: str | None = None
    """*Optional*. Signature of the post author for messages in channels, or the custom title of an anonymous group administrator"""
    paid_star_count: int | None = None
    """*Optional*. The number of Telegram Stars that were paid by the sender of the message to send it"""
    text: str | None = None
    """*Optional*. For text messages, the actual UTF-8 text of the message"""
    entities: list[MessageEntity] | None = None
    """*Optional*. For text messages, special entities like usernames, URLs, bot commands, etc. that appear in the text"""
    link_preview_options: LinkPreviewOptions | None = None
    """*Optional*. Options used for link preview generation for the message, if it is a text message and link preview options were changed"""
    suggested_post_info: SuggestedPostInfo | None = None
    """*Optional*. Information about suggested post parameters if the message is a suggested post in a channel direct messages chat. If the message is an approved or declined suggested post, then it can't be edited"""
    effect_id: str | None = None
    """*Optional*. Unique identifier of the message effect added to the message"""
    animation: Animation | None = None
    """*Optional*. Message is an animation, information about the animation. For backward compatibility, when this field is set, the *document* field will also be set"""
    audio: Audio | None = None
    """*Optional*. Message is an audio file, information about the file"""
    document: Document | None = None
    """*Optional*. Message is a general file, information about the file"""
    live_photo: LivePhoto | None = None
    """*Optional*. Message is a live photo, information about the live photo. For backward compatibility, when this field is set, the *photo* field will also be set"""
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
    caption: str | None = None
    """*Optional*. Caption for the animation, audio, document, paid media, photo, video or voice"""
    caption_entities: list[MessageEntity] | None = None
    """*Optional*. For messages with a caption, special entities like usernames, URLs, bot commands, etc. that appear in the caption"""
    show_caption_above_media: bool | None = None
    """*Optional*. :code:`True`, if the caption must be shown above the message media"""
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
    poll: Poll | None = None
    """*Optional*. Message is a native poll, information about the poll"""
    venue: Venue | None = None
    """*Optional*. Message is a venue, information about the venue. For backward compatibility, when this field is set, the *location* field will also be set"""
    location: Location | None = None
    """*Optional*. Message is a shared location, information about the location"""
    new_chat_members: list[User] | None = None
    """*Optional*. New members that were added to the group or supergroup and information about them (the bot itself may be one of these members)"""
    left_chat_member: User | None = None
    """*Optional*. A member was removed from the group, information about them (this member may be the bot itself)"""
    chat_owner_left: ChatOwnerLeft | None = None
    """*Optional*. Service message: chat owner has left"""
    chat_owner_changed: ChatOwnerChanged | None = None
    """*Optional*. Service message: chat owner has changed"""
    new_chat_title: str | None = None
    """*Optional*. A chat title was changed to this value"""
    new_chat_photo: list[PhotoSize] | None = None
    """*Optional*. A chat photo was change to this value"""
    delete_chat_photo: bool | None = None
    """*Optional*. Service message: the chat photo was deleted"""
    group_chat_created: bool | None = None
    """*Optional*. Service message: the group has been created"""
    supergroup_chat_created: bool | None = None
    """*Optional*. Service message: the supergroup has been created. This field can't be received in a message coming through updates, because bot can't be a member of a supergroup when it is created. It can only be found in reply_to_message if someone replies to a very first message in a directly created supergroup"""
    channel_chat_created: bool | None = None
    """*Optional*. Service message: the channel has been created. This field can't be received in a message coming through updates, because bot can't be a member of a channel when it is created. It can only be found in reply_to_message if someone replies to a very first message in a channel"""
    message_auto_delete_timer_changed: MessageAutoDeleteTimerChanged | None = None
    """*Optional*. Service message: auto-delete timer settings changed in the chat"""
    migrate_to_chat_id: int | None = None
    """*Optional*. The group has been migrated to a supergroup with the specified identifier. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a signed 64-bit integer or double-precision float type are safe for storing this identifier"""
    migrate_from_chat_id: int | None = None
    """*Optional*. The supergroup has been migrated from a group with the specified identifier. This number may have more than 32 significant bits and some programming languages may have difficulty/silent defects in interpreting it. But it has at most 52 significant bits, so a signed 64-bit integer or double-precision float type are safe for storing this identifier"""
    pinned_message: MaybeInaccessibleMessageUnion | None = None
    """*Optional*. Specified message was pinned. Note that the :class:`aiogram.types.message.Message` object in this field will not contain further *reply_to_message* fields even if it itself is a reply"""
    invoice: Invoice | None = None
    """*Optional*. Message is an invoice for a `payment <https://core.telegram.org/bots/api#payments>`_, information about the invoice. `More about payments » <https://core.telegram.org/bots/api#payments>`_"""
    successful_payment: SuccessfulPayment | None = None
    """*Optional*. Message is a service message about a successful payment, information about the payment. `More about payments » <https://core.telegram.org/bots/api#payments>`_"""
    refunded_payment: RefundedPayment | None = None
    """*Optional*. Message is a service message about a refunded payment, information about the payment. `More about payments » <https://core.telegram.org/bots/api#payments>`_"""
    users_shared: UsersShared | None = None
    """*Optional*. Service message: users were shared with the bot"""
    chat_shared: ChatShared | None = None
    """*Optional*. Service message: a chat was shared with the bot"""
    gift: GiftInfo | None = None
    """*Optional*. Service message: a regular gift was sent or received"""
    unique_gift: UniqueGiftInfo | None = None
    """*Optional*. Service message: a unique gift was sent or received"""
    gift_upgrade_sent: GiftInfo | None = None
    """*Optional*. Service message: upgrade of a gift was purchased after the gift was sent"""
    connected_website: str | None = None
    """*Optional*. The domain name of the website on which the user has logged in. `More about Telegram Login » <https://core.telegram.org/widgets/login>`_"""
    write_access_allowed: WriteAccessAllowed | None = None
    """*Optional*. Service message: the user allowed the bot to write messages after adding it to the attachment or side menu, launching a Web App from a link, or accepting an explicit request from a Web App sent by the method `requestWriteAccess <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_"""
    passport_data: PassportData | None = None
    """*Optional*. Telegram Passport data"""
    proximity_alert_triggered: ProximityAlertTriggered | None = None
    """*Optional*. Service message. A user in the chat triggered another user's proximity alert while sharing Live Location"""
    boost_added: ChatBoostAdded | None = None
    """*Optional*. Service message: user boosted the chat"""
    chat_background_set: ChatBackground | None = None
    """*Optional*. Service message: chat background set"""
    checklist_tasks_done: ChecklistTasksDone | None = None
    """*Optional*. Service message: some tasks in a checklist were marked as done or not done"""
    checklist_tasks_added: ChecklistTasksAdded | None = None
    """*Optional*. Service message: tasks were added to a checklist"""
    direct_message_price_changed: DirectMessagePriceChanged | None = None
    """*Optional*. Service message: the price for paid messages in the corresponding direct messages chat of a channel has changed"""
    forum_topic_created: ForumTopicCreated | None = None
    """*Optional*. Service message: forum topic created"""
    forum_topic_edited: ForumTopicEdited | None = None
    """*Optional*. Service message: forum topic edited"""
    forum_topic_closed: ForumTopicClosed | None = None
    """*Optional*. Service message: forum topic closed"""
    forum_topic_reopened: ForumTopicReopened | None = None
    """*Optional*. Service message: forum topic reopened"""
    general_forum_topic_hidden: GeneralForumTopicHidden | None = None
    """*Optional*. Service message: the 'General' forum topic hidden"""
    general_forum_topic_unhidden: GeneralForumTopicUnhidden | None = None
    """*Optional*. Service message: the 'General' forum topic unhidden"""
    giveaway_created: GiveawayCreated | None = None
    """*Optional*. Service message: a scheduled giveaway was created"""
    giveaway: Giveaway | None = None
    """*Optional*. The message is a scheduled giveaway message"""
    giveaway_winners: GiveawayWinners | None = None
    """*Optional*. A giveaway with public winners was completed"""
    giveaway_completed: GiveawayCompleted | None = None
    """*Optional*. Service message: a giveaway without public winners was completed"""
    managed_bot_created: ManagedBotCreated | None = None
    """*Optional*. Service message: user created a bot that will be managed by the current bot"""
    paid_message_price_changed: PaidMessagePriceChanged | None = None
    """*Optional*. Service message: the price for paid messages has changed in the chat"""
    poll_option_added: PollOptionAdded | None = None
    """*Optional*. Service message: answer option was added to a poll"""
    poll_option_deleted: PollOptionDeleted | None = None
    """*Optional*. Service message: answer option was deleted from a poll"""
    suggested_post_approved: SuggestedPostApproved | None = None
    """*Optional*. Service message: a suggested post was approved"""
    suggested_post_approval_failed: SuggestedPostApprovalFailed | None = None
    """*Optional*. Service message: approval of a suggested post has failed"""
    suggested_post_declined: SuggestedPostDeclined | None = None
    """*Optional*. Service message: a suggested post was declined"""
    suggested_post_paid: SuggestedPostPaid | None = None
    """*Optional*. Service message: payment for a suggested post was received"""
    suggested_post_refunded: SuggestedPostRefunded | None = None
    """*Optional*. Service message: payment for a suggested post was refunded"""
    video_chat_scheduled: VideoChatScheduled | None = None
    """*Optional*. Service message: video chat scheduled"""
    video_chat_started: VideoChatStarted | None = None
    """*Optional*. Service message: video chat started"""
    video_chat_ended: VideoChatEnded | None = None
    """*Optional*. Service message: video chat ended"""
    video_chat_participants_invited: VideoChatParticipantsInvited | None = None
    """*Optional*. Service message: new participants invited to a video chat"""
    web_app_data: WebAppData | None = None
    """*Optional*. Service message: data sent by a Web App"""
    reply_markup: InlineKeyboardMarkup | None = None
    """*Optional*. `Inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ attached to the message. :code:`login_url` buttons are represented as ordinary :code:`url` buttons"""
    rich_message: RichMessage | None = None
    """*Optional*. Message is a rich formatted message"""
    forward_date: DateTime | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. For forwarded messages, date the original message was sent in Unix time

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    forward_from: User | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. For forwarded messages, sender of the original message

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    forward_from_chat: Chat | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. For messages forwarded from channels or from anonymous administrators, information about the original sender chat

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    forward_from_message_id: int | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. For messages forwarded from channels, identifier of the original message in the channel

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    forward_sender_name: str | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. Sender's name for messages forwarded from users who disallow adding a link to their account in forwarded messages

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    forward_signature: str | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. For forwarded messages that were originally sent in channels or by an anonymous chat administrator, signature of the message sender if present

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""
    user_shared: UserShared | None = Field(None, json_schema_extra={"deprecated": True})
    """*Optional*. Service message: a user was shared with the bot

.. deprecated:: API:7.0
   https://core.telegram.org/bots/api-changelog#december-29-2023"""

    if TYPE_CHECKING:
        # DO NOT EDIT MANUALLY!!!
        # This section was auto-generated via `butcher`

        def __init__(
            __pydantic__self__,
            *,
            message_id: int,
            date: DateTime,
            chat: Chat,
            message_thread_id: int | None = None,
            direct_messages_topic: DirectMessagesTopic | None = None,
            from_user: User | None = None,
            sender_chat: Chat | None = None,
            sender_boost_count: int | None = None,
            sender_business_bot: User | None = None,
            sender_tag: str | None = None,
            guest_query_id: str | None = None,
            business_connection_id: str | None = None,
            forward_origin: MessageOriginUnion | None = None,
            is_topic_message: bool | None = None,
            is_automatic_forward: bool | None = None,
            reply_to_message: Message | None = None,
            external_reply: ExternalReplyInfo | None = None,
            quote: TextQuote | None = None,
            reply_to_story: Story | None = None,
            reply_to_checklist_task_id: int | None = None,
            reply_to_poll_option_id: str | None = None,
            via_bot: User | None = None,
            guest_bot_caller_user: User | None = None,
            guest_bot_caller_chat: Chat | None = None,
            edit_date: int | None = None,
            has_protected_content: bool | None = None,
            is_from_offline: bool | None = None,
            is_paid_post: bool | None = None,
            media_group_id: str | None = None,
            author_signature: str | None = None,
            paid_star_count: int | None = None,
            text: str | None = None,
            entities: list[MessageEntity] | None = None,
            link_preview_options: LinkPreviewOptions | None = None,
            suggested_post_info: SuggestedPostInfo | None = None,
            effect_id: str | None = None,
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
            caption: str | None = None,
            caption_entities: list[MessageEntity] | None = None,
            show_caption_above_media: bool | None = None,
            has_media_spoiler: bool | None = None,
            checklist: Checklist | None = None,
            contact: Contact | None = None,
            dice: Dice | None = None,
            game: Game | None = None,
            poll: Poll | None = None,
            venue: Venue | None = None,
            location: Location | None = None,
            new_chat_members: list[User] | None = None,
            left_chat_member: User | None = None,
            chat_owner_left: ChatOwnerLeft | None = None,
            chat_owner_changed: ChatOwnerChanged | None = None,
            new_chat_title: str | None = None,
            new_chat_photo: list[PhotoSize] | None = None,
            delete_chat_photo: bool | None = None,
            group_chat_created: bool | None = None,
            supergroup_chat_created: bool | None = None,
            channel_chat_created: bool | None = None,
            message_auto_delete_timer_changed: MessageAutoDeleteTimerChanged | None = None,
            migrate_to_chat_id: int | None = None,
            migrate_from_chat_id: int | None = None,
            pinned_message: MaybeInaccessibleMessageUnion | None = None,
            invoice: Invoice | None = None,
            successful_payment: SuccessfulPayment | None = None,
            refunded_payment: RefundedPayment | None = None,
            users_shared: UsersShared | None = None,
            chat_shared: ChatShared | None = None,
            gift: GiftInfo | None = None,
            unique_gift: UniqueGiftInfo | None = None,
            gift_upgrade_sent: GiftInfo | None = None,
            connected_website: str | None = None,
            write_access_allowed: WriteAccessAllowed | None = None,
            passport_data: PassportData | None = None,
            proximity_alert_triggered: ProximityAlertTriggered | None = None,
            boost_added: ChatBoostAdded | None = None,
            chat_background_set: ChatBackground | None = None,
            checklist_tasks_done: ChecklistTasksDone | None = None,
            checklist_tasks_added: ChecklistTasksAdded | None = None,
            direct_message_price_changed: DirectMessagePriceChanged | None = None,
            forum_topic_created: ForumTopicCreated | None = None,
            forum_topic_edited: ForumTopicEdited | None = None,
            forum_topic_closed: ForumTopicClosed | None = None,
            forum_topic_reopened: ForumTopicReopened | None = None,
            general_forum_topic_hidden: GeneralForumTopicHidden | None = None,
            general_forum_topic_unhidden: GeneralForumTopicUnhidden | None = None,
            giveaway_created: GiveawayCreated | None = None,
            giveaway: Giveaway | None = None,
            giveaway_winners: GiveawayWinners | None = None,
            giveaway_completed: GiveawayCompleted | None = None,
            managed_bot_created: ManagedBotCreated | None = None,
            paid_message_price_changed: PaidMessagePriceChanged | None = None,
            poll_option_added: PollOptionAdded | None = None,
            poll_option_deleted: PollOptionDeleted | None = None,
            suggested_post_approved: SuggestedPostApproved | None = None,
            suggested_post_approval_failed: SuggestedPostApprovalFailed | None = None,
            suggested_post_declined: SuggestedPostDeclined | None = None,
            suggested_post_paid: SuggestedPostPaid | None = None,
            suggested_post_refunded: SuggestedPostRefunded | None = None,
            video_chat_scheduled: VideoChatScheduled | None = None,
            video_chat_started: VideoChatStarted | None = None,
            video_chat_ended: VideoChatEnded | None = None,
            video_chat_participants_invited: VideoChatParticipantsInvited | None = None,
            web_app_data: WebAppData | None = None,
            reply_markup: InlineKeyboardMarkup | None = None,
            rich_message: RichMessage | None = None,
            forward_date: DateTime | None = None,
            forward_from: User | None = None,
            forward_from_chat: Chat | None = None,
            forward_from_message_id: int | None = None,
            forward_sender_name: str | None = None,
            forward_signature: str | None = None,
            user_shared: UserShared | None = None,
            **__pydantic_kwargs: Any,
        ) -> None:
            # DO NOT EDIT MANUALLY!!!
            # This method was auto-generated via `butcher`
            # Is needed only for type checking and IDE support without any additional plugins

            super().__init__(
                message_id=message_id,
                date=date,
                chat=chat,
                message_thread_id=message_thread_id,
                direct_messages_topic=direct_messages_topic,
                from_user=from_user,
                sender_chat=sender_chat,
                sender_boost_count=sender_boost_count,
                sender_business_bot=sender_business_bot,
                sender_tag=sender_tag,
                guest_query_id=guest_query_id,
                business_connection_id=business_connection_id,
                forward_origin=forward_origin,
                is_topic_message=is_topic_message,
                is_automatic_forward=is_automatic_forward,
                reply_to_message=reply_to_message,
                external_reply=external_reply,
                quote=quote,
                reply_to_story=reply_to_story,
                reply_to_checklist_task_id=reply_to_checklist_task_id,
                reply_to_poll_option_id=reply_to_poll_option_id,
                via_bot=via_bot,
                guest_bot_caller_user=guest_bot_caller_user,
                guest_bot_caller_chat=guest_bot_caller_chat,
                edit_date=edit_date,
                has_protected_content=has_protected_content,
                is_from_offline=is_from_offline,
                is_paid_post=is_paid_post,
                media_group_id=media_group_id,
                author_signature=author_signature,
                paid_star_count=paid_star_count,
                text=text,
                entities=entities,
                link_preview_options=link_preview_options,
                suggested_post_info=suggested_post_info,
                effect_id=effect_id,
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
                caption=caption,
                caption_entities=caption_entities,
                show_caption_above_media=show_caption_above_media,
                has_media_spoiler=has_media_spoiler,
                checklist=checklist,
                contact=contact,
                dice=dice,
                game=game,
                poll=poll,
                venue=venue,
                location=location,
                new_chat_members=new_chat_members,
                left_chat_member=left_chat_member,
                chat_owner_left=chat_owner_left,
                chat_owner_changed=chat_owner_changed,
                new_chat_title=new_chat_title,
                new_chat_photo=new_chat_photo,
                delete_chat_photo=delete_chat_photo,
                group_chat_created=group_chat_created,
                supergroup_chat_created=supergroup_chat_created,
                channel_chat_created=channel_chat_created,
                message_auto_delete_timer_changed=message_auto_delete_timer_changed,
                migrate_to_chat_id=migrate_to_chat_id,
                migrate_from_chat_id=migrate_from_chat_id,
                pinned_message=pinned_message,
                invoice=invoice,
                successful_payment=successful_payment,
                refunded_payment=refunded_payment,
                users_shared=users_shared,
                chat_shared=chat_shared,
                gift=gift,
                unique_gift=unique_gift,
                gift_upgrade_sent=gift_upgrade_sent,
                connected_website=connected_website,
                write_access_allowed=write_access_allowed,
                passport_data=passport_data,
                proximity_alert_triggered=proximity_alert_triggered,
                boost_added=boost_added,
                chat_background_set=chat_background_set,
                checklist_tasks_done=checklist_tasks_done,
                checklist_tasks_added=checklist_tasks_added,
                direct_message_price_changed=direct_message_price_changed,
                forum_topic_created=forum_topic_created,
                forum_topic_edited=forum_topic_edited,
                forum_topic_closed=forum_topic_closed,
                forum_topic_reopened=forum_topic_reopened,
                general_forum_topic_hidden=general_forum_topic_hidden,
                general_forum_topic_unhidden=general_forum_topic_unhidden,
                giveaway_created=giveaway_created,
                giveaway=giveaway,
                giveaway_winners=giveaway_winners,
                giveaway_completed=giveaway_completed,
                managed_bot_created=managed_bot_created,
                paid_message_price_changed=paid_message_price_changed,
                poll_option_added=poll_option_added,
                poll_option_deleted=poll_option_deleted,
                suggested_post_approved=suggested_post_approved,
                suggested_post_approval_failed=suggested_post_approval_failed,
                suggested_post_declined=suggested_post_declined,
                suggested_post_paid=suggested_post_paid,
                suggested_post_refunded=suggested_post_refunded,
                video_chat_scheduled=video_chat_scheduled,
                video_chat_started=video_chat_started,
                video_chat_ended=video_chat_ended,
                video_chat_participants_invited=video_chat_participants_invited,
                web_app_data=web_app_data,
                reply_markup=reply_markup,
                rich_message=rich_message,
                forward_date=forward_date,
                forward_from=forward_from,
                forward_from_chat=forward_from_chat,
                forward_from_message_id=forward_from_message_id,
                forward_sender_name=forward_sender_name,
                forward_signature=forward_signature,
                user_shared=user_shared,
                **__pydantic_kwargs,
            )

    @property
    def content_type(self) -> str:
        if self.text:
            return ContentType.TEXT
        if self.audio:
            return ContentType.AUDIO
        if self.animation:
            return ContentType.ANIMATION
        if self.document:
            return ContentType.DOCUMENT
        if self.game:
            return ContentType.GAME
        if self.photo:
            return ContentType.PHOTO
        if self.sticker:
            return ContentType.STICKER
        if self.video:
            return ContentType.VIDEO
        if self.video_note:
            return ContentType.VIDEO_NOTE
        if self.voice:
            return ContentType.VOICE
        if self.checklist:
            return ContentType.CHECKLIST
        if self.contact:
            return ContentType.CONTACT
        if self.venue:
            return ContentType.VENUE
        if self.location:
            return ContentType.LOCATION
        if self.new_chat_members:
            return ContentType.NEW_CHAT_MEMBERS
        if self.left_chat_member:
            return ContentType.LEFT_CHAT_MEMBER
        if self.chat_owner_left:
            return ContentType.CHAT_OWNER_LEFT
        if self.chat_owner_changed:
            return ContentType.CHAT_OWNER_CHANGED
        if self.invoice:
            return ContentType.INVOICE
        if self.successful_payment:
            return ContentType.SUCCESSFUL_PAYMENT
        if self.users_shared:
            return ContentType.USERS_SHARED
        if self.connected_website:
            return ContentType.CONNECTED_WEBSITE
        if self.migrate_from_chat_id:
            return ContentType.MIGRATE_FROM_CHAT_ID
        if self.migrate_to_chat_id:
            return ContentType.MIGRATE_TO_CHAT_ID
        if self.pinned_message:
            return ContentType.PINNED_MESSAGE
        if self.new_chat_title:
            return ContentType.NEW_CHAT_TITLE
        if self.new_chat_photo:
            return ContentType.NEW_CHAT_PHOTO
        if self.delete_chat_photo:
            return ContentType.DELETE_CHAT_PHOTO
        if self.group_chat_created:
            return ContentType.GROUP_CHAT_CREATED
        if self.supergroup_chat_created:
            return ContentType.SUPERGROUP_CHAT_CREATED
        if self.channel_chat_created:
            return ContentType.CHANNEL_CHAT_CREATED
        if self.paid_media:
            return ContentType.PAID_MEDIA
        if self.passport_data:
            return ContentType.PASSPORT_DATA
        if self.proximity_alert_triggered:
            return ContentType.PROXIMITY_ALERT_TRIGGERED
        if self.poll:
            return ContentType.POLL
        if self.dice:
            return ContentType.DICE
        if self.message_auto_delete_timer_changed:
            return ContentType.MESSAGE_AUTO_DELETE_TIMER_CHANGED
        if self.forum_topic_created:
            return ContentType.FORUM_TOPIC_CREATED
        if self.forum_topic_edited:
            return ContentType.FORUM_TOPIC_EDITED
        if self.forum_topic_closed:
            return ContentType.FORUM_TOPIC_CLOSED
        if self.forum_topic_reopened:
            return ContentType.FORUM_TOPIC_REOPENED
        if self.general_forum_topic_hidden:
            return ContentType.GENERAL_FORUM_TOPIC_HIDDEN
        if self.general_forum_topic_unhidden:
            return ContentType.GENERAL_FORUM_TOPIC_UNHIDDEN
        if self.giveaway_created:
            return ContentType.GIVEAWAY_CREATED
        if self.giveaway:
            return ContentType.GIVEAWAY
        if self.giveaway_completed:
            return ContentType.GIVEAWAY_COMPLETED
        if self.giveaway_winners:
            return ContentType.GIVEAWAY_WINNERS
        if self.video_chat_scheduled:
            return ContentType.VIDEO_CHAT_SCHEDULED
        if self.video_chat_started:
            return ContentType.VIDEO_CHAT_STARTED
        if self.video_chat_ended:
            return ContentType.VIDEO_CHAT_ENDED
        if self.video_chat_participants_invited:
            return ContentType.VIDEO_CHAT_PARTICIPANTS_INVITED
        if self.web_app_data:
            return ContentType.WEB_APP_DATA
        if self.user_shared:
            return ContentType.USER_SHARED
        if self.chat_shared:
            return ContentType.CHAT_SHARED
        if self.story:
            return ContentType.STORY
        if self.write_access_allowed:
            return ContentType.WRITE_ACCESS_ALLOWED
        if self.chat_background_set:
            return ContentType.CHAT_BACKGROUND_SET
        if self.boost_added:
            return ContentType.BOOST_ADDED
        if self.checklist_tasks_done:
            return ContentType.CHECKLIST_TASKS_DONE
        if self.checklist_tasks_added:
            return ContentType.CHECKLIST_TASKS_ADDED
        if self.direct_message_price_changed:
            return ContentType.DIRECT_MESSAGE_PRICE_CHANGED
        if self.refunded_payment:
            return ContentType.REFUNDED_PAYMENT
        if self.gift:
            return ContentType.GIFT
        if self.unique_gift:
            return ContentType.UNIQUE_GIFT
        if self.gift_upgrade_sent:
            return ContentType.GIFT_UPGRADE_SENT
        if self.paid_message_price_changed:
            return ContentType.PAID_MESSAGE_PRICE_CHANGED
        if self.suggested_post_approved:
            return ContentType.SUGGESTED_POST_APPROVED
        if self.suggested_post_approval_failed:
            return ContentType.SUGGESTED_POST_APPROVAL_FAILED
        if self.suggested_post_declined:
            return ContentType.SUGGESTED_POST_DECLINED
        if self.suggested_post_paid:
            return ContentType.SUGGESTED_POST_PAID
        if self.suggested_post_refunded:
            return ContentType.SUGGESTED_POST_REFUNDED
        if self.managed_bot_created:
            return ContentType.MANAGED_BOT_CREATED
        if self.poll_option_added:
            return ContentType.POLL_OPTION_ADDED
        if self.poll_option_deleted:
            return ContentType.POLL_OPTION_DELETED
        if self.live_photo:
            return ContentType.LIVE_PHOTO
        if self.rich_message:
            return ContentType.RICH_MESSAGE
        return ContentType.UNKNOWN

    def _unparse_entities(self, text_decoration: TextDecoration) -> str:
        text = self.text or self.caption or ""
        entities = self.entities or self.caption_entities or []
        return text_decoration.unparse(text=text, entities=entities)

    @property
    def html_text(self) -> str:
        return self._unparse_entities(html_decoration)

    @property
    def md_text(self) -> str:
        return self._unparse_entities(markdown_decoration)

    def as_reply_parameters(
        self,
        allow_sending_without_reply: bool | Default | None = Default(
            "allow_sending_without_reply"
        ),
        quote: str | None = None,
        quote_parse_mode: str | Default | None = Default("parse_mode"),
        quote_entities: list[MessageEntity] | None = None,
        quote_position: int | None = None,
    ) -> ReplyParameters:
        return ReplyParameters(
            message_id=self.message_id,
            chat_id=self.chat.id,
            allow_sending_without_reply=allow_sending_without_reply,
            quote=quote,
            quote_parse_mode=quote_parse_mode,
            quote_entities=quote_entities,
            quote_position=quote_position,
        )

    def reply_animation(
        self,
        animation: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        width: int | None = None,
        height: int | None = None,
        thumbnail: InputFile | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendAnimation:
        """
        Shortcut for method :class:`aiogram.methods.send_animation.SendAnimation`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send animation files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendanimation

        :param animation: Animation to send. Pass a file_id as String to send an animation that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an animation from the Internet, or upload a new animation using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent animation in seconds
        :param width: Animation width
        :param height: Animation height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Animation caption (may also be used when resending animation by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the animation caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the animation needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_animation.SendAnimation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendAnimation

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendAnimation(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            animation=animation,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            width=width,
            height=height,
            thumbnail=thumbnail,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_animation(
        self,
        animation: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        width: int | None = None,
        height: int | None = None,
        thumbnail: InputFile | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendAnimation:
        """
        Shortcut for method :class:`aiogram.methods.send_animation.SendAnimation`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send animation files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendanimation

        :param animation: Animation to send. Pass a file_id as String to send an animation that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an animation from the Internet, or upload a new animation using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent animation in seconds
        :param width: Animation width
        :param height: Animation height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Animation caption (may also be used when resending animation by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the animation caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the animation needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_animation.SendAnimation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendAnimation

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendAnimation(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            animation=animation,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            width=width,
            height=height,
            thumbnail=thumbnail,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_audio(
        self,
        audio: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        duration: int | None = None,
        performer: str | None = None,
        title: str | None = None,
        thumbnail: InputFile | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendAudio:
        """
        Shortcut for method :class:`aiogram.methods.send_audio.SendAudio`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio must be in the .MP3 or .M4A format. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send audio files of up to 50 MB in size, this limit may be changed in the future.
        For sending voice messages, use the :class:`aiogram.methods.send_voice.SendVoice` method instead.

        Source: https://core.telegram.org/bots/api#sendaudio

        :param audio: Audio file to send. Pass a file_id as String to send an audio file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an audio file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Audio caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the audio caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the audio in seconds
        :param performer: Performer
        :param title: Track name
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_audio.SendAudio`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendAudio

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendAudio(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            audio=audio,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            duration=duration,
            performer=performer,
            title=title,
            thumbnail=thumbnail,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_audio(
        self,
        audio: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        duration: int | None = None,
        performer: str | None = None,
        title: str | None = None,
        thumbnail: InputFile | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendAudio:
        """
        Shortcut for method :class:`aiogram.methods.send_audio.SendAudio`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio must be in the .MP3 or .M4A format. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send audio files of up to 50 MB in size, this limit may be changed in the future.
        For sending voice messages, use the :class:`aiogram.methods.send_voice.SendVoice` method instead.

        Source: https://core.telegram.org/bots/api#sendaudio

        :param audio: Audio file to send. Pass a file_id as String to send an audio file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an audio file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Audio caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the audio caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the audio in seconds
        :param performer: Performer
        :param title: Track name
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_audio.SendAudio`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendAudio

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendAudio(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            audio=audio,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            duration=duration,
            performer=performer,
            title=title,
            thumbnail=thumbnail,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_contact(
        self,
        phone_number: str,
        first_name: str,
        direct_messages_topic_id: int | None = None,
        last_name: str | None = None,
        vcard: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendContact:
        """
        Shortcut for method :class:`aiogram.methods.send_contact.SendContact`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send phone contacts. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendcontact

        :param phone_number: Contact's phone number
        :param first_name: Contact's first name
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param last_name: Contact's last name
        :param vcard: Additional data about the contact in the form of a `vCard <https://en.wikipedia.org/wiki/VCard>`_, 0-2048 bytes
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_contact.SendContact`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendContact

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendContact(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            phone_number=phone_number,
            first_name=first_name,
            direct_messages_topic_id=direct_messages_topic_id,
            last_name=last_name,
            vcard=vcard,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_contact(
        self,
        phone_number: str,
        first_name: str,
        direct_messages_topic_id: int | None = None,
        last_name: str | None = None,
        vcard: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendContact:
        """
        Shortcut for method :class:`aiogram.methods.send_contact.SendContact`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send phone contacts. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendcontact

        :param phone_number: Contact's phone number
        :param first_name: Contact's first name
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param last_name: Contact's last name
        :param vcard: Additional data about the contact in the form of a `vCard <https://en.wikipedia.org/wiki/VCard>`_, 0-2048 bytes
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_contact.SendContact`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendContact

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendContact(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            phone_number=phone_number,
            first_name=first_name,
            direct_messages_topic_id=direct_messages_topic_id,
            last_name=last_name,
            vcard=vcard,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_document(
        self,
        document: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        thumbnail: InputFile | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        disable_content_type_detection: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendDocument:
        """
        Shortcut for method :class:`aiogram.methods.send_document.SendDocument`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send general files. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send files of any type of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#senddocument

        :param document: File to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Document caption (may also be used when resending documents by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the document caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param disable_content_type_detection: Disables automatic server-side content type detection for files uploaded using multipart/form-data
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_document.SendDocument`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendDocument

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendDocument(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            document=document,
            direct_messages_topic_id=direct_messages_topic_id,
            thumbnail=thumbnail,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            disable_content_type_detection=disable_content_type_detection,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_document(
        self,
        document: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        thumbnail: InputFile | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        disable_content_type_detection: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendDocument:
        """
        Shortcut for method :class:`aiogram.methods.send_document.SendDocument`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send general files. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send files of any type of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#senddocument

        :param document: File to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Document caption (may also be used when resending documents by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the document caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param disable_content_type_detection: Disables automatic server-side content type detection for files uploaded using multipart/form-data
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_document.SendDocument`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendDocument

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendDocument(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            document=document,
            direct_messages_topic_id=direct_messages_topic_id,
            thumbnail=thumbnail,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            disable_content_type_detection=disable_content_type_detection,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_game(
        self,
        game_short_name: str,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendGame:
        """
        Shortcut for method :class:`aiogram.methods.send_game.SendGame`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send a game. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendgame

        :param game_short_name: Short name of the game, serves as the unique identifier for the game. Set up your games via `@BotFather <https://t.me/botfather>`_
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Play game_title' button will be shown. If not empty, the first button must launch the game
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_game.SendGame`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendGame

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendGame(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            game_short_name=game_short_name,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_game(
        self,
        game_short_name: str,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendGame:
        """
        Shortcut for method :class:`aiogram.methods.send_game.SendGame`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send a game. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendgame

        :param game_short_name: Short name of the game, serves as the unique identifier for the game. Set up your games via `@BotFather <https://t.me/botfather>`_
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Play game_title' button will be shown. If not empty, the first button must launch the game
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_game.SendGame`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendGame

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendGame(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            game_short_name=game_short_name,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_invoice(
        self,
        title: str,
        description: str,
        payload: str,
        currency: str,
        prices: list[LabeledPrice],
        direct_messages_topic_id: int | None = None,
        provider_token: str | None = None,
        max_tip_amount: int | None = None,
        suggested_tip_amounts: list[int] | None = None,
        start_parameter: str | None = None,
        provider_data: str | None = None,
        photo_url: str | None = None,
        photo_size: int | None = None,
        photo_width: int | None = None,
        photo_height: int | None = None,
        need_name: bool | None = None,
        need_phone_number: bool | None = None,
        need_email: bool | None = None,
        need_shipping_address: bool | None = None,
        send_phone_number_to_provider: bool | None = None,
        send_email_to_provider: bool | None = None,
        is_flexible: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendInvoice:
        """
        Shortcut for method :class:`aiogram.methods.send_invoice.SendInvoice`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send invoices. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendinvoice

        :param title: Product name, 1-32 characters
        :param description: Product description, 1-255 characters
        :param payload: Bot-defined invoice payload, 1-128 bytes. This will not be displayed to the user, use it for your internal processes
        :param currency: Three-letter ISO 4217 currency code, see `more on currencies <https://core.telegram.org/bots/payments#supported-currencies>`_. Pass 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param prices: Price breakdown, a JSON-serialized list of components (e.g. product price, tax, discount, delivery cost, delivery tax, bonus, etc.). Must contain exactly one item for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param provider_token: Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param max_tip_amount: The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param suggested_tip_amounts: A JSON-serialized array of suggested amounts of tips in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*
        :param start_parameter: Unique deep-linking parameter. If left empty, **forwarded copies** of the sent message will have a *Pay* button, allowing multiple users to pay directly from the forwarded message, using the same invoice. If non-empty, forwarded copies of the sent message will have a *URL* button with a deep link to the bot (instead of a *Pay* button), with the value used as the start parameter
        :param provider_data: JSON-serialized data about the invoice, which will be shared with the payment provider. A detailed description of required fields should be provided by the payment provider
        :param photo_url: URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service. People like it better when they see what they are paying for
        :param photo_size: Photo size in bytes
        :param photo_width: Photo width
        :param photo_height: Photo height
        :param need_name: Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_phone_number: Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_email: Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_shipping_address: Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param send_phone_number_to_provider: Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param send_email_to_provider: Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param is_flexible: Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Pay :code:`total price`' button will be shown. If not empty, the first button must be a Pay button
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_invoice.SendInvoice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendInvoice

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendInvoice(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            title=title,
            description=description,
            payload=payload,
            currency=currency,
            prices=prices,
            direct_messages_topic_id=direct_messages_topic_id,
            provider_token=provider_token,
            max_tip_amount=max_tip_amount,
            suggested_tip_amounts=suggested_tip_amounts,
            start_parameter=start_parameter,
            provider_data=provider_data,
            photo_url=photo_url,
            photo_size=photo_size,
            photo_width=photo_width,
            photo_height=photo_height,
            need_name=need_name,
            need_phone_number=need_phone_number,
            need_email=need_email,
            need_shipping_address=need_shipping_address,
            send_phone_number_to_provider=send_phone_number_to_provider,
            send_email_to_provider=send_email_to_provider,
            is_flexible=is_flexible,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_invoice(
        self,
        title: str,
        description: str,
        payload: str,
        currency: str,
        prices: list[LabeledPrice],
        direct_messages_topic_id: int | None = None,
        provider_token: str | None = None,
        max_tip_amount: int | None = None,
        suggested_tip_amounts: list[int] | None = None,
        start_parameter: str | None = None,
        provider_data: str | None = None,
        photo_url: str | None = None,
        photo_size: int | None = None,
        photo_width: int | None = None,
        photo_height: int | None = None,
        need_name: bool | None = None,
        need_phone_number: bool | None = None,
        need_email: bool | None = None,
        need_shipping_address: bool | None = None,
        send_phone_number_to_provider: bool | None = None,
        send_email_to_provider: bool | None = None,
        is_flexible: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendInvoice:
        """
        Shortcut for method :class:`aiogram.methods.send_invoice.SendInvoice`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send invoices. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendinvoice

        :param title: Product name, 1-32 characters
        :param description: Product description, 1-255 characters
        :param payload: Bot-defined invoice payload, 1-128 bytes. This will not be displayed to the user, use it for your internal processes
        :param currency: Three-letter ISO 4217 currency code, see `more on currencies <https://core.telegram.org/bots/payments#supported-currencies>`_. Pass 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param prices: Price breakdown, a JSON-serialized list of components (e.g. product price, tax, discount, delivery cost, delivery tax, bonus, etc.). Must contain exactly one item for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param provider_token: Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param max_tip_amount: The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param suggested_tip_amounts: A JSON-serialized array of suggested amounts of tips in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*
        :param start_parameter: Unique deep-linking parameter. If left empty, **forwarded copies** of the sent message will have a *Pay* button, allowing multiple users to pay directly from the forwarded message, using the same invoice. If non-empty, forwarded copies of the sent message will have a *URL* button with a deep link to the bot (instead of a *Pay* button), with the value used as the start parameter
        :param provider_data: JSON-serialized data about the invoice, which will be shared with the payment provider. A detailed description of required fields should be provided by the payment provider
        :param photo_url: URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service. People like it better when they see what they are paying for
        :param photo_size: Photo size in bytes
        :param photo_width: Photo width
        :param photo_height: Photo height
        :param need_name: Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_phone_number: Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_email: Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param need_shipping_address: Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param send_phone_number_to_provider: Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param send_email_to_provider: Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param is_flexible: Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Pay :code:`total price`' button will be shown. If not empty, the first button must be a Pay button
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_invoice.SendInvoice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendInvoice

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendInvoice(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            title=title,
            description=description,
            payload=payload,
            currency=currency,
            prices=prices,
            direct_messages_topic_id=direct_messages_topic_id,
            provider_token=provider_token,
            max_tip_amount=max_tip_amount,
            suggested_tip_amounts=suggested_tip_amounts,
            start_parameter=start_parameter,
            provider_data=provider_data,
            photo_url=photo_url,
            photo_size=photo_size,
            photo_width=photo_width,
            photo_height=photo_height,
            need_name=need_name,
            need_phone_number=need_phone_number,
            need_email=need_email,
            need_shipping_address=need_shipping_address,
            send_phone_number_to_provider=send_phone_number_to_provider,
            send_email_to_provider=send_email_to_provider,
            is_flexible=is_flexible,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_location(
        self,
        latitude: float,
        longitude: float,
        direct_messages_topic_id: int | None = None,
        horizontal_accuracy: float | None = None,
        live_period: int | None = None,
        heading: int | None = None,
        proximity_alert_radius: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendLocation:
        """
        Shortcut for method :class:`aiogram.methods.send_location.SendLocation`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send point on the map. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendlocation

        :param latitude: Latitude of the location
        :param longitude: Longitude of the location
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param horizontal_accuracy: The radius of uncertainty for the location, measured in meters; 0-1500
        :param live_period: Period in seconds during which the location will be updated (see `Live Locations <https://telegram.org/blog/live-locations>`_, should be between 60 and 86400, or 0x7FFFFFFF for live locations that can be edited indefinitely
        :param heading: For live locations, a direction in which the user is moving, in degrees. Must be between 1 and 360 if specified
        :param proximity_alert_radius: For live locations, a maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_location.SendLocation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendLocation

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendLocation(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            latitude=latitude,
            longitude=longitude,
            direct_messages_topic_id=direct_messages_topic_id,
            horizontal_accuracy=horizontal_accuracy,
            live_period=live_period,
            heading=heading,
            proximity_alert_radius=proximity_alert_radius,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_location(
        self,
        latitude: float,
        longitude: float,
        direct_messages_topic_id: int | None = None,
        horizontal_accuracy: float | None = None,
        live_period: int | None = None,
        heading: int | None = None,
        proximity_alert_radius: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendLocation:
        """
        Shortcut for method :class:`aiogram.methods.send_location.SendLocation`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send point on the map. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendlocation

        :param latitude: Latitude of the location
        :param longitude: Longitude of the location
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param horizontal_accuracy: The radius of uncertainty for the location, measured in meters; 0-1500
        :param live_period: Period in seconds during which the location will be updated (see `Live Locations <https://telegram.org/blog/live-locations>`_, should be between 60 and 86400, or 0x7FFFFFFF for live locations that can be edited indefinitely
        :param heading: For live locations, a direction in which the user is moving, in degrees. Must be between 1 and 360 if specified
        :param proximity_alert_radius: For live locations, a maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_location.SendLocation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendLocation

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendLocation(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            latitude=latitude,
            longitude=longitude,
            direct_messages_topic_id=direct_messages_topic_id,
            horizontal_accuracy=horizontal_accuracy,
            live_period=live_period,
            heading=heading,
            proximity_alert_radius=proximity_alert_radius,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_media_group(
        self,
        media: list[MediaUnion],
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendMediaGroup:
        """
        Shortcut for method :class:`aiogram.methods.send_media_group.SendMediaGroup`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send a group of photos, live photos, videos, documents or audios as an album. Documents and audio files can be only grouped in an album with messages of the same type. On success, an array of :class:`aiogram.types.message.Message` objects that were sent is returned.

        Source: https://core.telegram.org/bots/api#sendmediagroup

        :param media: A JSON-serialized array describing messages to be sent, must include 2-10 items
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the messages will be sent; required if the messages are sent to a direct messages chat
        :param disable_notification: Sends messages `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent messages from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_media_group.SendMediaGroup`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendMediaGroup

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendMediaGroup(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            media=media,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_media_group(
        self,
        media: list[MediaUnion],
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendMediaGroup:
        """
        Shortcut for method :class:`aiogram.methods.send_media_group.SendMediaGroup`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send a group of photos, live photos, videos, documents or audios as an album. Documents and audio files can be only grouped in an album with messages of the same type. On success, an array of :class:`aiogram.types.message.Message` objects that were sent is returned.

        Source: https://core.telegram.org/bots/api#sendmediagroup

        :param media: A JSON-serialized array describing messages to be sent, must include 2-10 items
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the messages will be sent; required if the messages are sent to a direct messages chat
        :param disable_notification: Sends messages `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent messages from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the messages are a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_media_group.SendMediaGroup`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendMediaGroup

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendMediaGroup(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            media=media,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply(
        self,
        text: str,
        direct_messages_topic_id: int | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        entities: list[MessageEntity] | None = None,
        link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview"),
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        disable_web_page_preview: bool | Default | None = Default("link_preview_is_disabled"),
        **kwargs: Any,
    ) -> SendMessage:
        """
        Shortcut for method :class:`aiogram.methods.send_message.SendMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send text messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendmessage

        :param text: Text of the message to be sent, 1-4096 characters after entities parsing
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param link_preview_options: Link preview generation options for the message
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param disable_web_page_preview: Disables link previews for links in this message
        :return: instance of method :class:`aiogram.methods.send_message.SendMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendMessage(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            text=text,
            direct_messages_topic_id=direct_messages_topic_id,
            parse_mode=parse_mode,
            entities=entities,
            link_preview_options=link_preview_options,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            disable_web_page_preview=disable_web_page_preview,
            **kwargs,
        ).as_(self._bot)

    def answer(
        self,
        text: str,
        direct_messages_topic_id: int | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        entities: list[MessageEntity] | None = None,
        link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview"),
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        disable_web_page_preview: bool | Default | None = Default("link_preview_is_disabled"),
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendMessage:
        """
        Shortcut for method :class:`aiogram.methods.send_message.SendMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send text messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendmessage

        :param text: Text of the message to be sent, 1-4096 characters after entities parsing
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param link_preview_options: Link preview generation options for the message
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param disable_web_page_preview: Disables link previews for links in this message
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_message.SendMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendMessage(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            text=text,
            direct_messages_topic_id=direct_messages_topic_id,
            parse_mode=parse_mode,
            entities=entities,
            link_preview_options=link_preview_options,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            disable_web_page_preview=disable_web_page_preview,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_photo(
        self,
        photo: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendPhoto:
        """
        Shortcut for method :class:`aiogram.methods.send_photo.SendPhoto`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send photos. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendphoto

        :param photo: Photo to send. Pass a file_id as String to send a photo that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a photo from the Internet, or upload a new photo using multipart/form-data. The photo must be at most 10 MB in size. The photo's width and height must not exceed 10000 in total. Width and height ratio must be at most 20. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Photo caption (may also be used when resending photos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the photo caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the photo needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_photo.SendPhoto`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPhoto

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendPhoto(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            photo=photo,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_photo(
        self,
        photo: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendPhoto:
        """
        Shortcut for method :class:`aiogram.methods.send_photo.SendPhoto`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send photos. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendphoto

        :param photo: Photo to send. Pass a file_id as String to send a photo that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a photo from the Internet, or upload a new photo using multipart/form-data. The photo must be at most 10 MB in size. The photo's width and height must not exceed 10000 in total. Width and height ratio must be at most 20. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Photo caption (may also be used when resending photos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the photo caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the photo needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_photo.SendPhoto`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPhoto

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendPhoto(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            photo=photo,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_poll(
        self,
        question: str,
        options: list[InputPollOptionUnion],
        question_parse_mode: str | Default | None = Default("parse_mode"),
        question_entities: list[MessageEntity] | None = None,
        is_anonymous: bool | None = None,
        type: str | None = None,
        allows_multiple_answers: bool | None = None,
        allows_revoting: bool | None = None,
        shuffle_options: bool | None = None,
        allow_adding_options: bool | None = None,
        hide_results_until_closes: bool | None = None,
        members_only: bool | None = None,
        country_codes: list[str] | None = None,
        correct_option_ids: list[int] | None = None,
        explanation: str | None = None,
        explanation_parse_mode: str | Default | None = Default("parse_mode"),
        explanation_entities: list[MessageEntity] | None = None,
        explanation_media: InputPollMediaUnion | None = None,
        open_period: int | None = None,
        close_date: DateTimeUnion | None = None,
        is_closed: bool | None = None,
        description: str | None = None,
        description_parse_mode: str | Default | None = Default("parse_mode"),
        description_entities: list[MessageEntity] | None = None,
        media: InputPollMediaUnion | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        correct_option_id: int | None = None,
        **kwargs: Any,
    ) -> SendPoll:
        """
        Shortcut for method :class:`aiogram.methods.send_poll.SendPoll`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send a native poll. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpoll

        :param question: Poll question, 1-300 characters
        :param options: A JSON-serialized list of 1-12 answer options
        :param question_parse_mode: Mode for parsing entities in the question. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Currently, only custom emoji entities are allowed
        :param question_entities: A JSON-serialized list of special entities that appear in the poll question. It can be specified instead of *question_parse_mode*
        :param is_anonymous: :code:`True`, if the poll needs to be anonymous, defaults to :code:`True`
        :param type: Poll type, 'quiz' or 'regular', defaults to 'regular'
        :param allows_multiple_answers: Pass :code:`True`, if the poll allows multiple answers, defaults to :code:`False`
        :param allows_revoting: Pass :code:`True`, if the poll allows to change chosen answer options, defaults to :code:`False` for quizzes and to :code:`True` for regular polls
        :param shuffle_options: Pass :code:`True`, if the poll options must be shown in random order
        :param allow_adding_options: Pass :code:`True`, if answer options can be added to the poll after creation; not supported for anonymous polls and quizzes
        :param hide_results_until_closes: Pass :code:`True`, if poll results must be shown only after the poll closes
        :param members_only: Pass :code:`True`, if voting is limited to users who have been members of the chat where the poll is being sent for more than 24 hours; for channel chats only
        :param country_codes: A JSON-serialized list of 0-12 two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which users can vote in the poll; for channel chats only. Use 'FT' as a country code to allow users with anonymous numbers to vote. If omitted or empty, then users from any country can participate in the poll
        :param correct_option_ids: A JSON-serialized list of monotonically increasing 0-based identifiers of the correct answer options, required for polls in quiz mode
        :param explanation: Text that is shown when a user chooses an incorrect answer or taps on the lamp icon in a quiz-style poll, 0-200 characters with at most 2 line feeds after entities parsing
        :param explanation_parse_mode: Mode for parsing entities in the explanation. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param explanation_entities: A JSON-serialized list of special entities that appear in the poll explanation. It can be specified instead of *explanation_parse_mode*
        :param explanation_media: Media added to the quiz explanation
        :param open_period: Amount of time in seconds the poll will be active after creation, 5-2628000. Can't be used together with *close_date*
        :param close_date: Point in time (Unix timestamp) when the poll will be automatically closed. Must be at least 5 and no more than 2628000 seconds in the future. Can't be used together with *open_period*
        :param is_closed: Pass :code:`True` if the poll needs to be immediately closed. This can be useful for poll preview
        :param description: Description of the poll to be sent, 0-1024 characters after entities parsing
        :param description_parse_mode: Mode for parsing entities in the poll description. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param description_entities: A JSON-serialized list of special entities that appear in the poll description, which can be specified instead of *description_parse_mode*
        :param media: Media added to the poll description
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param correct_option_id: 0-based identifier of the correct answer option, required for polls in quiz mode
        :return: instance of method :class:`aiogram.methods.send_poll.SendPoll`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPoll

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendPoll(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            question=question,
            options=options,
            question_parse_mode=question_parse_mode,
            question_entities=question_entities,
            is_anonymous=is_anonymous,
            type=type,
            allows_multiple_answers=allows_multiple_answers,
            allows_revoting=allows_revoting,
            shuffle_options=shuffle_options,
            allow_adding_options=allow_adding_options,
            hide_results_until_closes=hide_results_until_closes,
            members_only=members_only,
            country_codes=country_codes,
            correct_option_ids=correct_option_ids,
            explanation=explanation,
            explanation_parse_mode=explanation_parse_mode,
            explanation_entities=explanation_entities,
            explanation_media=explanation_media,
            open_period=open_period,
            close_date=close_date,
            is_closed=is_closed,
            description=description,
            description_parse_mode=description_parse_mode,
            description_entities=description_entities,
            media=media,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            correct_option_id=correct_option_id,
            **kwargs,
        ).as_(self._bot)

    def answer_poll(
        self,
        question: str,
        options: list[InputPollOptionUnion],
        question_parse_mode: str | Default | None = Default("parse_mode"),
        question_entities: list[MessageEntity] | None = None,
        is_anonymous: bool | None = None,
        type: str | None = None,
        allows_multiple_answers: bool | None = None,
        allows_revoting: bool | None = None,
        shuffle_options: bool | None = None,
        allow_adding_options: bool | None = None,
        hide_results_until_closes: bool | None = None,
        members_only: bool | None = None,
        country_codes: list[str] | None = None,
        correct_option_ids: list[int] | None = None,
        explanation: str | None = None,
        explanation_parse_mode: str | Default | None = Default("parse_mode"),
        explanation_entities: list[MessageEntity] | None = None,
        explanation_media: InputPollMediaUnion | None = None,
        open_period: int | None = None,
        close_date: DateTimeUnion | None = None,
        is_closed: bool | None = None,
        description: str | None = None,
        description_parse_mode: str | Default | None = Default("parse_mode"),
        description_entities: list[MessageEntity] | None = None,
        media: InputPollMediaUnion | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        correct_option_id: int | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendPoll:
        """
        Shortcut for method :class:`aiogram.methods.send_poll.SendPoll`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send a native poll. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpoll

        :param question: Poll question, 1-300 characters
        :param options: A JSON-serialized list of 1-12 answer options
        :param question_parse_mode: Mode for parsing entities in the question. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Currently, only custom emoji entities are allowed
        :param question_entities: A JSON-serialized list of special entities that appear in the poll question. It can be specified instead of *question_parse_mode*
        :param is_anonymous: :code:`True`, if the poll needs to be anonymous, defaults to :code:`True`
        :param type: Poll type, 'quiz' or 'regular', defaults to 'regular'
        :param allows_multiple_answers: Pass :code:`True`, if the poll allows multiple answers, defaults to :code:`False`
        :param allows_revoting: Pass :code:`True`, if the poll allows to change chosen answer options, defaults to :code:`False` for quizzes and to :code:`True` for regular polls
        :param shuffle_options: Pass :code:`True`, if the poll options must be shown in random order
        :param allow_adding_options: Pass :code:`True`, if answer options can be added to the poll after creation; not supported for anonymous polls and quizzes
        :param hide_results_until_closes: Pass :code:`True`, if poll results must be shown only after the poll closes
        :param members_only: Pass :code:`True`, if voting is limited to users who have been members of the chat where the poll is being sent for more than 24 hours; for channel chats only
        :param country_codes: A JSON-serialized list of 0-12 two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which users can vote in the poll; for channel chats only. Use 'FT' as a country code to allow users with anonymous numbers to vote. If omitted or empty, then users from any country can participate in the poll
        :param correct_option_ids: A JSON-serialized list of monotonically increasing 0-based identifiers of the correct answer options, required for polls in quiz mode
        :param explanation: Text that is shown when a user chooses an incorrect answer or taps on the lamp icon in a quiz-style poll, 0-200 characters with at most 2 line feeds after entities parsing
        :param explanation_parse_mode: Mode for parsing entities in the explanation. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param explanation_entities: A JSON-serialized list of special entities that appear in the poll explanation. It can be specified instead of *explanation_parse_mode*
        :param explanation_media: Media added to the quiz explanation
        :param open_period: Amount of time in seconds the poll will be active after creation, 5-2628000. Can't be used together with *close_date*
        :param close_date: Point in time (Unix timestamp) when the poll will be automatically closed. Must be at least 5 and no more than 2628000 seconds in the future. Can't be used together with *open_period*
        :param is_closed: Pass :code:`True` if the poll needs to be immediately closed. This can be useful for poll preview
        :param description: Description of the poll to be sent, 0-1024 characters after entities parsing
        :param description_parse_mode: Mode for parsing entities in the poll description. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param description_entities: A JSON-serialized list of special entities that appear in the poll description, which can be specified instead of *description_parse_mode*
        :param media: Media added to the poll description
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param correct_option_id: 0-based identifier of the correct answer option, required for polls in quiz mode
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_poll.SendPoll`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPoll

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendPoll(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            question=question,
            options=options,
            question_parse_mode=question_parse_mode,
            question_entities=question_entities,
            is_anonymous=is_anonymous,
            type=type,
            allows_multiple_answers=allows_multiple_answers,
            allows_revoting=allows_revoting,
            shuffle_options=shuffle_options,
            allow_adding_options=allow_adding_options,
            hide_results_until_closes=hide_results_until_closes,
            members_only=members_only,
            country_codes=country_codes,
            correct_option_ids=correct_option_ids,
            explanation=explanation,
            explanation_parse_mode=explanation_parse_mode,
            explanation_entities=explanation_entities,
            explanation_media=explanation_media,
            open_period=open_period,
            close_date=close_date,
            is_closed=is_closed,
            description=description,
            description_parse_mode=description_parse_mode,
            description_entities=description_entities,
            media=media,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            correct_option_id=correct_option_id,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_dice(
        self,
        direct_messages_topic_id: int | None = None,
        emoji: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendDice:
        """
        Shortcut for method :class:`aiogram.methods.send_dice.SendDice`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send an animated emoji that will display a random value. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#senddice

        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji on which the dice throw animation is based. Currently, must be one of '🎲', '🎯', '🏀', '⚽', '🎳', or '🎰'. Dice can have values 1-6 for '🎲', '🎯' and '🎳', values 1-5 for '🏀' and '⚽', and values 1-64 for '🎰'. Defaults to '🎲'
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_dice.SendDice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendDice

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendDice(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            direct_messages_topic_id=direct_messages_topic_id,
            emoji=emoji,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_dice(
        self,
        direct_messages_topic_id: int | None = None,
        emoji: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendDice:
        """
        Shortcut for method :class:`aiogram.methods.send_dice.SendDice`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send an animated emoji that will display a random value. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#senddice

        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji on which the dice throw animation is based. Currently, must be one of '🎲', '🎯', '🏀', '⚽', '🎳', or '🎰'. Dice can have values 1-6 for '🎲', '🎯' and '🎳', values 1-5 for '🏀' and '⚽', and values 1-64 for '🎰'. Defaults to '🎲'
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_dice.SendDice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendDice

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendDice(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            direct_messages_topic_id=direct_messages_topic_id,
            emoji=emoji,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_sticker(
        self,
        sticker: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        emoji: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendSticker:
        """
        Shortcut for method :class:`aiogram.methods.send_sticker.SendSticker`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send static .WEBP, `animated <https://telegram.org/blog/animated-stickers>`_ .TGS, or `video <https://telegram.org/blog/video-stickers-better-reactions>`_ .WEBM stickers. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendsticker

        :param sticker: Sticker to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a .WEBP sticker from the Internet, or upload a new .WEBP, .TGS, or .WEBM sticker using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Video and animated stickers can't be sent via an HTTP URL
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji associated with the sticker; only for just uploaded stickers
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_sticker.SendSticker`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendSticker

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendSticker(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            sticker=sticker,
            direct_messages_topic_id=direct_messages_topic_id,
            emoji=emoji,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_sticker(
        self,
        sticker: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        emoji: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendSticker:
        """
        Shortcut for method :class:`aiogram.methods.send_sticker.SendSticker`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send static .WEBP, `animated <https://telegram.org/blog/animated-stickers>`_ .TGS, or `video <https://telegram.org/blog/video-stickers-better-reactions>`_ .WEBM stickers. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendsticker

        :param sticker: Sticker to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a .WEBP sticker from the Internet, or upload a new .WEBP, .TGS, or .WEBM sticker using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Video and animated stickers can't be sent via an HTTP URL
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji associated with the sticker; only for just uploaded stickers
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_sticker.SendSticker`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendSticker

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendSticker(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            sticker=sticker,
            direct_messages_topic_id=direct_messages_topic_id,
            emoji=emoji,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_venue(
        self,
        latitude: float,
        longitude: float,
        title: str,
        address: str,
        direct_messages_topic_id: int | None = None,
        foursquare_id: str | None = None,
        foursquare_type: str | None = None,
        google_place_id: str | None = None,
        google_place_type: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendVenue:
        """
        Shortcut for method :class:`aiogram.methods.send_venue.SendVenue`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send information about a venue. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvenue

        :param latitude: Latitude of the venue
        :param longitude: Longitude of the venue
        :param title: Name of the venue
        :param address: Address of the venue
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param foursquare_id: Foursquare identifier of the venue
        :param foursquare_type: Foursquare type of the venue, if known. (For example, 'arts_entertainment/default', 'arts_entertainment/aquarium' or 'food/icecream'.)
        :param google_place_id: Google Places identifier of the venue
        :param google_place_type: Google Places type of the venue. (See `supported types <https://developers.google.com/places/web-service/supported_types>`_.)
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_venue.SendVenue`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVenue

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVenue(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            latitude=latitude,
            longitude=longitude,
            title=title,
            address=address,
            direct_messages_topic_id=direct_messages_topic_id,
            foursquare_id=foursquare_id,
            foursquare_type=foursquare_type,
            google_place_id=google_place_id,
            google_place_type=google_place_type,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_venue(
        self,
        latitude: float,
        longitude: float,
        title: str,
        address: str,
        direct_messages_topic_id: int | None = None,
        foursquare_id: str | None = None,
        foursquare_type: str | None = None,
        google_place_id: str | None = None,
        google_place_type: str | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVenue:
        """
        Shortcut for method :class:`aiogram.methods.send_venue.SendVenue`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send information about a venue. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvenue

        :param latitude: Latitude of the venue
        :param longitude: Longitude of the venue
        :param title: Name of the venue
        :param address: Address of the venue
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param foursquare_id: Foursquare identifier of the venue
        :param foursquare_type: Foursquare type of the venue, if known. (For example, 'arts_entertainment/default', 'arts_entertainment/aquarium' or 'food/icecream'.)
        :param google_place_id: Google Places identifier of the venue
        :param google_place_type: Google Places type of the venue. (See `supported types <https://developers.google.com/places/web-service/supported_types>`_.)
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_venue.SendVenue`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVenue

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVenue(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            latitude=latitude,
            longitude=longitude,
            title=title,
            address=address,
            direct_messages_topic_id=direct_messages_topic_id,
            foursquare_id=foursquare_id,
            foursquare_type=foursquare_type,
            google_place_id=google_place_id,
            google_place_type=google_place_type,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_video(
        self,
        video: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        width: int | None = None,
        height: int | None = None,
        thumbnail: InputFile | None = None,
        cover: InputFileUnion | None = None,
        start_timestamp: DateTimeUnion | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        supports_streaming: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendVideo:
        """
        Shortcut for method :class:`aiogram.methods.send_video.SendVideo`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send video files, Telegram clients support MPEG4 videos (other formats may be sent as :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvideo

        :param video: Video to send. Pass a file_id as String to send a video that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a video from the Internet, or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param width: Video width
        :param height: Video height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param cover: Cover for the video in the message. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`
        :param start_timestamp: Start timestamp for the video in the message
        :param caption: Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the video needs to be covered with a spoiler animation
        :param supports_streaming: Pass :code:`True` if the uploaded video is suitable for streaming
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_video.SendVideo`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVideo

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVideo(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            video=video,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            width=width,
            height=height,
            thumbnail=thumbnail,
            cover=cover,
            start_timestamp=start_timestamp,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            supports_streaming=supports_streaming,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_video(
        self,
        video: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        width: int | None = None,
        height: int | None = None,
        thumbnail: InputFile | None = None,
        cover: InputFileUnion | None = None,
        start_timestamp: DateTimeUnion | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        has_spoiler: bool | None = None,
        supports_streaming: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVideo:
        """
        Shortcut for method :class:`aiogram.methods.send_video.SendVideo`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send video files, Telegram clients support MPEG4 videos (other formats may be sent as :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvideo

        :param video: Video to send. Pass a file_id as String to send a video that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a video from the Internet, or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param width: Video width
        :param height: Video height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param cover: Cover for the video in the message. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`
        :param start_timestamp: Start timestamp for the video in the message
        :param caption: Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the video needs to be covered with a spoiler animation
        :param supports_streaming: Pass :code:`True` if the uploaded video is suitable for streaming
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_video.SendVideo`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVideo

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVideo(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            video=video,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            width=width,
            height=height,
            thumbnail=thumbnail,
            cover=cover,
            start_timestamp=start_timestamp,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            has_spoiler=has_spoiler,
            supports_streaming=supports_streaming,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_video_note(
        self,
        video_note: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        length: int | None = None,
        thumbnail: InputFile | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendVideoNote:
        """
        Shortcut for method :class:`aiogram.methods.send_video_note.SendVideoNote`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        As of `v.4.0 <https://telegram.org/blog/video-messages-and-telescope>`_, Telegram clients support rounded square MPEG4 videos of up to 1 minute long. Use this method to send video messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvideonote

        :param video_note: Video note to send. Pass a file_id as String to send a video note that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending video notes by a URL is currently unsupported
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param length: Video width and height, i.e. diameter of the video message
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_video_note.SendVideoNote`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVideoNote

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVideoNote(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            video_note=video_note,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            length=length,
            thumbnail=thumbnail,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_video_note(
        self,
        video_note: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        duration: int | None = None,
        length: int | None = None,
        thumbnail: InputFile | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVideoNote:
        """
        Shortcut for method :class:`aiogram.methods.send_video_note.SendVideoNote`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        As of `v.4.0 <https://telegram.org/blog/video-messages-and-telescope>`_, Telegram clients support rounded square MPEG4 videos of up to 1 minute long. Use this method to send video messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvideonote

        :param video_note: Video note to send. Pass a file_id as String to send a video note that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending video notes by a URL is currently unsupported
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param length: Video width and height, i.e. diameter of the video message
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_video_note.SendVideoNote`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVideoNote

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVideoNote(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            video_note=video_note,
            direct_messages_topic_id=direct_messages_topic_id,
            duration=duration,
            length=length,
            thumbnail=thumbnail,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def reply_voice(
        self,
        voice: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        duration: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        **kwargs: Any,
    ) -> SendVoice:
        """
        Shortcut for method :class:`aiogram.methods.send_voice.SendVoice`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message. For this to work, your audio must be in an .OGG file encoded with OPUS, or in .MP3 format, or in .M4A format (other formats may be sent as :class:`aiogram.types.audio.Audio` or :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send voice messages of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvoice

        :param voice: Audio file to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Voice message caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the voice message caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the voice message in seconds
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :return: instance of method :class:`aiogram.methods.send_voice.SendVoice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVoice

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVoice(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            voice=voice,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            duration=duration,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            **kwargs,
        ).as_(self._bot)

    def answer_voice(
        self,
        voice: InputFileUnion,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        duration: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> SendVoice:
        """
        Shortcut for method :class:`aiogram.methods.send_voice.SendVoice`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message. For this to work, your audio must be in an .OGG file encoded with OPUS, or in .MP3 format, or in .M4A format (other formats may be sent as :class:`aiogram.types.audio.Audio` or :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send voice messages of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvoice

        :param voice: Audio file to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Voice message caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the voice message caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the voice message in seconds
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.send_voice.SendVoice`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendVoice

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendVoice(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            voice=voice,
            direct_messages_topic_id=direct_messages_topic_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            duration=duration,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def send_copy(  # noqa: C901
        self: Message,
        chat_id: ChatIdUnion,
        disable_notification: bool | None = None,
        reply_to_message_id: int | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | ReplyKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        message_thread_id: int | None = None,
        business_connection_id: str | None = None,
        parse_mode: str | None = None,
        message_effect_id: str | None = None,
        link_preview_options: LinkPreviewOptions | None = None,
    ) -> (
        ForwardMessage
        | SendAnimation
        | SendAudio
        | SendContact
        | SendDocument
        | SendLocation
        | SendMessage
        | SendPhoto
        | SendPoll
        | SendDice
        | SendSticker
        | SendVenue
        | SendVideo
        | SendVideoNote
        | SendVoice
    ):
        """
        Send copy of a message.

        Is similar to :meth:`aiogram.client.bot.Bot.copy_message`
        but returning the sent message instead of :class:`aiogram.types.message_id.MessageId`

        .. note::

            This method doesn't use the API method named `copyMessage` and
            historically implemented before the similar method is added to API

        :param chat_id:
        :param disable_notification:
        :param reply_to_message_id:
        :param reply_parameters:
        :param reply_markup:
        :param allow_sending_without_reply:
        :param message_thread_id:
        :param business_connection_id:
        :param parse_mode:
        :param message_effect_id:
        :param link_preview_options: Link preview generation options for the copied message.
            Only applied when the source message is a text message; falls back to
            the original message's ``link_preview_options`` when not provided.
        :return:
        """
        from ..methods import (
            ForwardMessage,
            SendAnimation,
            SendAudio,
            SendContact,
            SendDice,
            SendDocument,
            SendLocation,
            SendMessage,
            SendPhoto,
            SendPoll,
            SendSticker,
            SendVenue,
            SendVideo,
            SendVideoNote,
            SendVoice,
        )

        kwargs: dict[str, Any] = {
            "chat_id": chat_id,
            "reply_markup": reply_markup or self.reply_markup,
            "disable_notification": disable_notification,
            "reply_to_message_id": reply_to_message_id,
            "reply_parameters": reply_parameters,
            "message_thread_id": message_thread_id,
            "business_connection_id": business_connection_id,
            "allow_sending_without_reply": allow_sending_without_reply,
            # when sending a copy, we don't need any parse mode
            # because all entities are already prepared
            "parse_mode": parse_mode,
            "message_effect_id": message_effect_id or self.effect_id,
        }

        if self.text:
            return SendMessage(
                text=self.text,
                entities=self.entities,
                link_preview_options=link_preview_options or self.link_preview_options,
                **kwargs,
            ).as_(self._bot)
        if self.audio:
            return SendAudio(
                audio=self.audio.file_id,
                caption=self.caption,
                title=self.audio.title,
                performer=self.audio.performer,
                duration=self.audio.duration,
                caption_entities=self.caption_entities,
                **kwargs,
            ).as_(self._bot)
        if self.animation:
            return SendAnimation(
                animation=self.animation.file_id,
                caption=self.caption,
                caption_entities=self.caption_entities,
                **kwargs,
            ).as_(self._bot)
        if self.document:
            return SendDocument(
                document=self.document.file_id,
                caption=self.caption,
                caption_entities=self.caption_entities,
                **kwargs,
            ).as_(self._bot)
        if self.photo:
            return SendPhoto(
                photo=self.photo[-1].file_id,
                caption=self.caption,
                caption_entities=self.caption_entities,
                **kwargs,
            ).as_(self._bot)
        if self.sticker:
            return SendSticker(
                sticker=self.sticker.file_id,
                **kwargs,
            ).as_(self._bot)
        if self.video:
            return SendVideo(
                video=self.video.file_id,
                caption=self.caption,
                caption_entities=self.caption_entities,
                **kwargs,
            ).as_(self._bot)
        if self.video_note:
            return SendVideoNote(
                video_note=self.video_note.file_id,
                **kwargs,
            ).as_(self._bot)
        if self.voice:
            return SendVoice(
                voice=self.voice.file_id,
                **kwargs,
            ).as_(self._bot)
        if self.contact:
            return SendContact(
                phone_number=self.contact.phone_number,
                first_name=self.contact.first_name,
                last_name=self.contact.last_name,
                vcard=self.contact.vcard,
                **kwargs,
            ).as_(self._bot)
        if self.venue:
            return SendVenue(
                latitude=self.venue.location.latitude,
                longitude=self.venue.location.longitude,
                title=self.venue.title,
                address=self.venue.address,
                foursquare_id=self.venue.foursquare_id,
                foursquare_type=self.venue.foursquare_type,
                **kwargs,
            ).as_(self._bot)
        if self.location:
            return SendLocation(
                latitude=self.location.latitude,
                longitude=self.location.longitude,
                **kwargs,
            ).as_(self._bot)
        if self.poll:
            from .input_poll_option import InputPollOption

            return SendPoll(
                question=self.poll.question,
                options=[
                    InputPollOption(
                        text=option.text,
                        voter_count=option.voter_count,
                        text_entities=option.text_entities,
                        text_parse_mode=None,
                    )
                    for option in self.poll.options
                ],
                **kwargs,
            ).as_(self._bot)
        if self.dice:  # Dice value can't be controlled
            return SendDice(
                **kwargs,
            ).as_(self._bot)
        if self.story:
            return ForwardMessage(
                from_chat_id=self.chat.id,
                message_id=self.message_id,
                **kwargs,
            ).as_(self._bot)

        raise TypeError("This type of message can't be copied.")

    def copy_to(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        video_start_timestamp: DateTimeUnion | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        **kwargs: Any,
    ) -> CopyMessage:
        """
        Shortcut for method :class:`aiogram.methods.copy_message.CopyMessage`
        will automatically fill method attributes:

        - :code:`from_chat_id`
        - :code:`message_id`

        Use this method to copy messages of any kind. Service messages, paid media messages, giveaway messages, giveaway winners messages, and invoice messages can't be copied. A quiz :class:`aiogram.methods.poll.Poll` can be copied only if the value of the field *correct_option_id* is known to the bot. The method is analogous to the method :class:`aiogram.methods.forward_message.ForwardMessage`, but the copied message doesn't have a link to the original message. Returns the :class:`aiogram.types.message_id.MessageId` of the sent message on success.

        Source: https://core.telegram.org/bots/api#copymessage

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param video_start_timestamp: New start timestamp for the copied video in the message
        :param caption: New caption for media, 0-1024 characters after entities parsing. If not specified, the original caption is kept
        :param parse_mode: Mode for parsing entities in the new caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the new caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media. Ignored if a new caption isn't specified
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; only available when copying to private chats
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :return: instance of method :class:`aiogram.methods.copy_message.CopyMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import CopyMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return CopyMessage(
            from_chat_id=self.chat.id,
            message_id=self.message_id,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            video_start_timestamp=video_start_timestamp,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
            **kwargs,
        ).as_(self._bot)

    def edit_text(
        self,
        text: str | None = None,
        inline_message_id: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        entities: list[MessageEntity] | None = None,
        link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview"),
        reply_markup: InlineKeyboardMarkup | None = None,
        rich_message: InputRichMessage | None = None,
        disable_web_page_preview: bool | Default | None = Default("link_preview_is_disabled"),
        **kwargs: Any,
    ) -> EditMessageText:
        """
        Shortcut for method :class:`aiogram.methods.edit_message_text.EditMessageText`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to edit text, rich and `game <https://core.telegram.org/bots/api#games>`_ messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagetext

        :param text: New text of the message, 1-4096 characters after entity parsing; required if *rich_message* isn't specified
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param link_preview_options: Link preview generation options for the message
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param rich_message: New rich content of the message; required if *text* isn't specified
        :param disable_web_page_preview: Disables link previews for links in this message
        :return: instance of method :class:`aiogram.methods.edit_message_text.EditMessageText`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import EditMessageText

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return EditMessageText(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            text=text,
            inline_message_id=inline_message_id,
            parse_mode=parse_mode,
            entities=entities,
            link_preview_options=link_preview_options,
            reply_markup=reply_markup,
            rich_message=rich_message,
            disable_web_page_preview=disable_web_page_preview,
            **kwargs,
        ).as_(self._bot)

    def forward(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        video_start_timestamp: DateTimeUnion | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        **kwargs: Any,
    ) -> ForwardMessage:
        """
        Shortcut for method :class:`aiogram.methods.forward_message.ForwardMessage`
        will automatically fill method attributes:

        - :code:`from_chat_id`
        - :code:`message_id`

        Use this method to forward messages of any kind. Service messages and messages with protected content can't be forwarded. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#forwardmessage

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be forwarded; required if the message is forwarded to a direct messages chat
        :param video_start_timestamp: New start timestamp for the forwarded video in the message
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the forwarded message from forwarding and saving
        :param message_effect_id: Unique identifier of the message effect to be added to the message; only available when forwarding to private chats
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only
        :return: instance of method :class:`aiogram.methods.forward_message.ForwardMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import ForwardMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return ForwardMessage(
            from_chat_id=self.chat.id,
            message_id=self.message_id,
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            video_start_timestamp=video_start_timestamp,
            disable_notification=disable_notification,
            protect_content=protect_content,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            **kwargs,
        ).as_(self._bot)

    def edit_media(
        self,
        media: InputMediaUnion,
        inline_message_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        **kwargs: Any,
    ) -> EditMessageMedia:
        """
        Shortcut for method :class:`aiogram.methods.edit_message_media.EditMessageMedia`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to edit animation, audio, document, live photo, photo, or video messages, or to replace a text or a rich message with a media. If a message is part of a message album, then it can be edited only to an audio for audio albums, only to a document for document albums and to a photo, a live photo, or a video otherwise. When an inline message is edited, a new file can't be uploaded; use a previously uploaded file via its file_id or specify a URL. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagemedia

        :param media: A JSON-serialized object for a new media content of the message
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :param reply_markup: A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :return: instance of method :class:`aiogram.methods.edit_message_media.EditMessageMedia`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import EditMessageMedia

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return EditMessageMedia(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            media=media,
            inline_message_id=inline_message_id,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def edit_reply_markup(
        self,
        inline_message_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        **kwargs: Any,
    ) -> EditMessageReplyMarkup:
        """
        Shortcut for method :class:`aiogram.methods.edit_message_reply_markup.EditMessageReplyMarkup`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to edit only the reply markup of messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagereplymarkup

        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :return: instance of method :class:`aiogram.methods.edit_message_reply_markup.EditMessageReplyMarkup`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import EditMessageReplyMarkup

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return EditMessageReplyMarkup(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            inline_message_id=inline_message_id,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def delete_reply_markup(
        self,
        inline_message_id: str | None = None,
        **kwargs: Any,
    ) -> EditMessageReplyMarkup:
        """
        Shortcut for method :class:`aiogram.methods.edit_message_reply_markup.EditMessageReplyMarkup`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`
        - :code:`reply_markup`

        Use this method to edit only the reply markup of messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagereplymarkup

        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :return: instance of method :class:`aiogram.methods.edit_message_reply_markup.EditMessageReplyMarkup`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import EditMessageReplyMarkup

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return EditMessageReplyMarkup(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            reply_markup=None,
            inline_message_id=inline_message_id,
            **kwargs,
        ).as_(self._bot)

    def edit_live_location(
        self,
        latitude: float,
        longitude: float,
        inline_message_id: str | None = None,
        live_period: int | None = None,
        horizontal_accuracy: float | None = None,
        heading: int | None = None,
        proximity_alert_radius: int | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        **kwargs: Any,
    ) -> EditMessageLiveLocation:
        """
        Shortcut for method :class:`aiogram.methods.edit_message_live_location.EditMessageLiveLocation`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to edit live location messages. A location can be edited until its *live_period* expires or editing is explicitly disabled by a call to :class:`aiogram.methods.stop_message_live_location.StopMessageLiveLocation`. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.

        Source: https://core.telegram.org/bots/api#editmessagelivelocation

        :param latitude: Latitude of new location
        :param longitude: Longitude of new location
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :param live_period: New period in seconds during which the location can be updated, starting from the message send date. If 0x7FFFFFFF is specified, then the location can be updated forever. Otherwise, the new value must not exceed the current *live_period* by more than a day, and the live location expiration date must remain within the next 90 days. If not specified, then *live_period* remains unchanged
        :param horizontal_accuracy: The radius of uncertainty for the location, measured in meters; 0-1500
        :param heading: Direction in which the user is moving, in degrees. Must be between 1 and 360 if specified
        :param proximity_alert_radius: The maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified
        :param reply_markup: A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :return: instance of method :class:`aiogram.methods.edit_message_live_location.EditMessageLiveLocation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import EditMessageLiveLocation

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return EditMessageLiveLocation(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            latitude=latitude,
            longitude=longitude,
            inline_message_id=inline_message_id,
            live_period=live_period,
            horizontal_accuracy=horizontal_accuracy,
            heading=heading,
            proximity_alert_radius=proximity_alert_radius,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def stop_live_location(
        self,
        inline_message_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        **kwargs: Any,
    ) -> StopMessageLiveLocation:
        """
        Shortcut for method :class:`aiogram.methods.stop_message_live_location.StopMessageLiveLocation`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to stop updating a live location message before *live_period* expires. On success, if the message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.

        Source: https://core.telegram.org/bots/api#stopmessagelivelocation

        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :param reply_markup: A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :return: instance of method :class:`aiogram.methods.stop_message_live_location.StopMessageLiveLocation`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import StopMessageLiveLocation

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return StopMessageLiveLocation(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            inline_message_id=inline_message_id,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def edit_caption(
        self,
        inline_message_id: str | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        reply_markup: InlineKeyboardMarkup | None = None,
        **kwargs: Any,
    ) -> EditMessageCaption:
        """
        Shortcut for method :class:`aiogram.methods.edit_message_caption.EditMessageCaption`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to edit captions of messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagecaption

        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message
        :param caption: New caption of the message, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the message caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media. Supported only for animation, photo and video messages
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :return: instance of method :class:`aiogram.methods.edit_message_caption.EditMessageCaption`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import EditMessageCaption

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return EditMessageCaption(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            inline_message_id=inline_message_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def delete(
        self,
        **kwargs: Any,
    ) -> DeleteMessage:
        """
        Shortcut for method :class:`aiogram.methods.delete_message.DeleteMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to delete a message, including service messages, with the following limitations:

        - A message can only be deleted if it was sent less than 48 hours ago.

        - Service messages about a supergroup, channel, or forum topic creation can't be deleted.

        - A dice message in a private chat can only be deleted if it was sent more than 24 hours ago.

        - Bots can delete outgoing messages in private chats, groups, and supergroups.

        - Bots can delete incoming messages in private chats.

        - Bots granted *can_post_messages* permissions can delete outgoing messages in channels.

        - If the bot is an administrator of a group, it can delete any message there.

        - If the bot has *can_delete_messages* administrator right in a supergroup or a channel, it can delete any message there.

        - If the bot has *can_manage_direct_messages* administrator right in a channel, it can delete any message in the corresponding direct messages chat.

        Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletemessage

        :return: instance of method :class:`aiogram.methods.delete_message.DeleteMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import DeleteMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return DeleteMessage(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            **kwargs,
        ).as_(self._bot)

    def pin(
        self,
        disable_notification: bool | None = None,
        **kwargs: Any,
    ) -> PinChatMessage:
        """
        Shortcut for method :class:`aiogram.methods.pin_chat_message.PinChatMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to add a message to the list of pinned messages in a chat. In private chats and channel direct messages chats, all non-service messages can be pinned. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to pin messages in groups and channels respectively. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#pinchatmessage

        :param disable_notification: Pass :code:`True` if it is not necessary to send a notification to all chat members about the new pinned message. Notifications are always disabled in channels and private chats
        :return: instance of method :class:`aiogram.methods.pin_chat_message.PinChatMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import PinChatMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return PinChatMessage(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            disable_notification=disable_notification,
            **kwargs,
        ).as_(self._bot)

    def unpin(
        self,
        **kwargs: Any,
    ) -> UnpinChatMessage:
        """
        Shortcut for method :class:`aiogram.methods.unpin_chat_message.UnpinChatMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to remove a message from the list of pinned messages in a chat. In private chats and channel direct messages chats, all messages can be unpinned. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to unpin messages in groups and channels respectively. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unpinchatmessage

        :return: instance of method :class:`aiogram.methods.unpin_chat_message.UnpinChatMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import UnpinChatMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return UnpinChatMessage(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            **kwargs,
        ).as_(self._bot)

    def get_url(self, force_private: bool = False, include_thread_id: bool = False) -> str | None:
        """
        Returns message URL. Cannot be used in private (one-to-one) chats.
        If chat has a username, returns URL like https://t.me/username/message_id
        Otherwise (or if {force_private} flag is set), returns https://t.me/c/shifted_chat_id/message_id

        :param force_private: if set, a private URL is returned even for a public chat
        :param include_thread_id: if set, adds chat thread id to URL and returns like https://t.me/username/thread_id/message_id
        :return: string with full message URL
        """
        if self.chat.type in {"private", "group"}:
            return None

        chat_value = (
            f"c/{self.chat.shifted_id}"
            if not self.chat.username or force_private
            else self.chat.username
        )

        message_id_value = (
            f"{self.message_thread_id}/{self.message_id}"
            if include_thread_id and self.message_thread_id and self.is_topic_message
            else f"{self.message_id}"
        )

        return f"https://t.me/{chat_value}/{message_id_value}"

    def react(
        self,
        reaction: list[ReactionTypeUnion] | None = None,
        is_big: bool | None = None,
        **kwargs: Any,
    ) -> SetMessageReaction:
        """
        Shortcut for method :class:`aiogram.methods.set_message_reaction.SetMessageReaction`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_id`
        - :code:`business_connection_id`

        Use this method to change the chosen reactions on a message. Service messages of some types can't be reacted to. Automatically forwarded messages from a channel to its discussion group have the same available reactions as messages in the channel. Bots can't use paid reactions. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmessagereaction

        :param reaction: A JSON-serialized list of reaction types to set on the message. Currently, as non-premium users, bots can set up to one reaction per message. A custom emoji reaction can be used if it is either already present on the message or explicitly allowed by chat administrators. Paid reactions can't be used by bots
        :param is_big: Pass :code:`True` to set the reaction with a big animation
        :return: instance of method :class:`aiogram.methods.set_message_reaction.SetMessageReaction`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SetMessageReaction

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SetMessageReaction(
            chat_id=self.chat.id,
            message_id=self.message_id,
            business_connection_id=self.business_connection_id,
            reaction=reaction,
            is_big=is_big,
            **kwargs,
        ).as_(self._bot)

    def answer_paid_media(
        self,
        star_count: int,
        media: list[InputPaidMediaUnion],
        direct_messages_topic_id: int | None = None,
        payload: str | None = None,
        caption: str | None = None,
        parse_mode: str | None = None,
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        allow_paid_broadcast: bool | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        **kwargs: Any,
    ) -> SendPaidMedia:
        """
        Shortcut for method :class:`aiogram.methods.send_paid_media.SendPaidMedia`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send paid media. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpaidmedia

        :param star_count: The number of Telegram Stars that must be paid to buy access to the media; 1-25000
        :param media: A JSON-serialized array describing the media to be sent; up to 10 items
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param payload: Bot-defined paid media payload, 0-128 bytes. This will not be displayed to the user, use it for your internal processes
        :param caption: Media caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the media caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :return: instance of method :class:`aiogram.methods.send_paid_media.SendPaidMedia`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPaidMedia

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendPaidMedia(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            star_count=star_count,
            media=media,
            direct_messages_topic_id=direct_messages_topic_id,
            payload=payload,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def reply_paid_media(
        self,
        star_count: int,
        media: list[InputPaidMediaUnion],
        direct_messages_topic_id: int | None = None,
        payload: str | None = None,
        caption: str | None = None,
        parse_mode: str | None = None,
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        allow_paid_broadcast: bool | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        **kwargs: Any,
    ) -> SendPaidMedia:
        """
        Shortcut for method :class:`aiogram.methods.send_paid_media.SendPaidMedia`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send paid media. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpaidmedia

        :param star_count: The number of Telegram Stars that must be paid to buy access to the media; 1-25000
        :param media: A JSON-serialized array describing the media to be sent; up to 10 items
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param payload: Bot-defined paid media payload, 0-128 bytes. This will not be displayed to the user, use it for your internal processes
        :param caption: Media caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the media caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :return: instance of method :class:`aiogram.methods.send_paid_media.SendPaidMedia`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendPaidMedia

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendPaidMedia(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            star_count=star_count,
            media=media,
            direct_messages_topic_id=direct_messages_topic_id,
            payload=payload,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def answer_guest_query(
        self,
        result: InlineQueryResultUnion,
        **kwargs: Any,
    ) -> AnswerGuestQuery:
        """
        Shortcut for method :class:`aiogram.methods.answer_guest_query.AnswerGuestQuery`
        will automatically fill method attributes:

        - :code:`guest_query_id`

        Use this method to reply to a received guest message. On success, a :class:`aiogram.types.sent_guest_message.SentGuestMessage` object is returned.

        Source: https://core.telegram.org/bots/api#answerguestquery

        :param result: A JSON-serialized object describing the message to be sent
        :return: instance of method :class:`aiogram.methods.answer_guest_query.AnswerGuestQuery`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import AnswerGuestQuery

        assert self.guest_query_id is not None, (
            "This method can be used only if `guest_query_id` is present in the message."
        )

        return AnswerGuestQuery(
            guest_query_id=self.guest_query_id,
            result=result,
            **kwargs,
        ).as_(self._bot)

    def answer_rich(
        self,
        rich_message: InputRichMessage,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        **kwargs: Any,
    ) -> SendRichMessage:
        """
        Shortcut for method :class:`aiogram.methods.send_rich_message.SendRichMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`

        Use this method to send rich messages. If the message contains a block with a media element, then the bot must have the right to send the media to the chat. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendrichmessage

        :param rich_message: The message to be sent
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :return: instance of method :class:`aiogram.methods.send_rich_message.SendRichMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendRichMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendRichMessage(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            rich_message=rich_message,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)

    def reply_rich(
        self,
        rich_message: InputRichMessage,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        **kwargs: Any,
    ) -> SendRichMessage:
        """
        Shortcut for method :class:`aiogram.methods.send_rich_message.SendRichMessage`
        will automatically fill method attributes:

        - :code:`chat_id`
        - :code:`message_thread_id`
        - :code:`business_connection_id`
        - :code:`reply_parameters`

        Use this method to send rich messages. If the message contains a block with a media element, then the bot must have the right to send the media to the chat. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendrichmessage

        :param rich_message: The message to be sent
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user
        :return: instance of method :class:`aiogram.methods.send_rich_message.SendRichMessage`
        """
        # DO NOT EDIT MANUALLY!!!
        # This method was auto-generated via `butcher`

        from aiogram.methods import SendRichMessage

        assert self.chat is not None, (
            "This method can be used only if chat is present in the message."
        )

        return SendRichMessage(
            chat_id=self.chat.id,
            message_thread_id=self.message_thread_id if self.is_topic_message else None,
            business_connection_id=self.business_connection_id,
            reply_parameters=self.as_reply_parameters(),
            rich_message=rich_message,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_markup=reply_markup,
            **kwargs,
        ).as_(self._bot)
