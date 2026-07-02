from typing import Literal, Optional, Union

from .accepted_gift_types import AcceptedGiftTypes
from .affiliate_info import AffiliateInfo
from .animation import Animation
from .audio import Audio
from .background_fill import BackgroundFill
from .background_fill_freeform_gradient import BackgroundFillFreeformGradient
from .background_fill_gradient import BackgroundFillGradient
from .background_fill_solid import BackgroundFillSolid
from .background_fill_union import BackgroundFillUnion
from .background_type import BackgroundType
from .background_type_chat_theme import BackgroundTypeChatTheme
from .background_type_fill import BackgroundTypeFill
from .background_type_pattern import BackgroundTypePattern
from .background_type_union import BackgroundTypeUnion
from .background_type_wallpaper import BackgroundTypeWallpaper
from .base import UNSET_PARSE_MODE, TelegramObject
from .birthdate import Birthdate
from .bot_command import BotCommand
from .bot_command_scope import BotCommandScope
from .bot_command_scope_all_chat_administrators import (
    BotCommandScopeAllChatAdministrators,
)
from .bot_command_scope_all_group_chats import BotCommandScopeAllGroupChats
from .bot_command_scope_all_private_chats import BotCommandScopeAllPrivateChats
from .bot_command_scope_chat import BotCommandScopeChat
from .bot_command_scope_chat_administrators import BotCommandScopeChatAdministrators
from .bot_command_scope_chat_member import BotCommandScopeChatMember
from .bot_command_scope_default import BotCommandScopeDefault
from .bot_command_scope_union import BotCommandScopeUnion
from .bot_description import BotDescription
from .bot_name import BotName
from .bot_short_description import BotShortDescription
from .business_bot_rights import BusinessBotRights
from .business_connection import BusinessConnection
from .business_intro import BusinessIntro
from .business_location import BusinessLocation
from .business_messages_deleted import BusinessMessagesDeleted
from .business_opening_hours import BusinessOpeningHours
from .business_opening_hours_interval import BusinessOpeningHoursInterval
from .callback_game import CallbackGame
from .callback_query import CallbackQuery
from .chat import Chat
from .chat_administrator_rights import ChatAdministratorRights
from .chat_background import ChatBackground
from .chat_boost import ChatBoost
from .chat_boost_added import ChatBoostAdded
from .chat_boost_removed import ChatBoostRemoved
from .chat_boost_source import ChatBoostSource
from .chat_boost_source_gift_code import ChatBoostSourceGiftCode
from .chat_boost_source_giveaway import ChatBoostSourceGiveaway
from .chat_boost_source_premium import ChatBoostSourcePremium
from .chat_boost_source_union import ChatBoostSourceUnion
from .chat_boost_updated import ChatBoostUpdated
from .chat_full_info import ChatFullInfo
from .chat_id_union import ChatIdUnion
from .chat_invite_link import ChatInviteLink
from .chat_join_request import ChatJoinRequest
from .chat_location import ChatLocation
from .chat_member import ChatMember
from .chat_member_administrator import ChatMemberAdministrator
from .chat_member_banned import ChatMemberBanned
from .chat_member_left import ChatMemberLeft
from .chat_member_member import ChatMemberMember
from .chat_member_owner import ChatMemberOwner
from .chat_member_restricted import ChatMemberRestricted
from .chat_member_union import ChatMemberUnion
from .chat_member_updated import ChatMemberUpdated
from .chat_permissions import ChatPermissions
from .chat_photo import ChatPhoto
from .chat_shared import ChatShared
from .checklist import Checklist
from .checklist_task import ChecklistTask
from .checklist_tasks_added import ChecklistTasksAdded
from .checklist_tasks_done import ChecklistTasksDone
from .chosen_inline_result import ChosenInlineResult
from .contact import Contact
from .copy_text_button import CopyTextButton
from .custom import DateTime
from .date_time_union import DateTimeUnion
from .dice import Dice
from .direct_message_price_changed import DirectMessagePriceChanged
from .direct_messages_topic import DirectMessagesTopic
from .document import Document
from .downloadable import Downloadable
from .encrypted_credentials import EncryptedCredentials
from .encrypted_passport_element import EncryptedPassportElement
from .error_event import ErrorEvent
from .external_reply_info import ExternalReplyInfo
from .file import File
from .force_reply import ForceReply
from .forum_topic import ForumTopic
from .forum_topic_closed import ForumTopicClosed
from .forum_topic_created import ForumTopicCreated
from .forum_topic_edited import ForumTopicEdited
from .forum_topic_reopened import ForumTopicReopened
from .game import Game
from .game_high_score import GameHighScore
from .general_forum_topic_hidden import GeneralForumTopicHidden
from .general_forum_topic_unhidden import GeneralForumTopicUnhidden
from .gift import Gift
from .gift_background import GiftBackground
from .gift_info import GiftInfo
from .gifts import Gifts
from .giveaway import Giveaway
from .giveaway_completed import GiveawayCompleted
from .giveaway_created import GiveawayCreated
from .giveaway_winners import GiveawayWinners
from .inaccessible_message import InaccessibleMessage
from .inline_keyboard_button import InlineKeyboardButton
from .inline_keyboard_markup import InlineKeyboardMarkup
from .inline_query import InlineQuery
from .inline_query_result import InlineQueryResult
from .inline_query_result_article import InlineQueryResultArticle
from .inline_query_result_audio import InlineQueryResultAudio
from .inline_query_result_cached_audio import InlineQueryResultCachedAudio
from .inline_query_result_cached_document import InlineQueryResultCachedDocument
from .inline_query_result_cached_gif import InlineQueryResultCachedGif
from .inline_query_result_cached_mpeg4_gif import InlineQueryResultCachedMpeg4Gif
from .inline_query_result_cached_photo import InlineQueryResultCachedPhoto
from .inline_query_result_cached_sticker import InlineQueryResultCachedSticker
from .inline_query_result_cached_video import InlineQueryResultCachedVideo
from .inline_query_result_cached_voice import InlineQueryResultCachedVoice
from .inline_query_result_contact import InlineQueryResultContact
from .inline_query_result_document import InlineQueryResultDocument
from .inline_query_result_game import InlineQueryResultGame
from .inline_query_result_gif import InlineQueryResultGif
from .inline_query_result_location import InlineQueryResultLocation
from .inline_query_result_mpeg4_gif import InlineQueryResultMpeg4Gif
from .inline_query_result_photo import InlineQueryResultPhoto
from .inline_query_result_union import InlineQueryResultUnion
from .inline_query_result_venue import InlineQueryResultVenue
from .inline_query_result_video import InlineQueryResultVideo
from .inline_query_result_voice import InlineQueryResultVoice
from .inline_query_results_button import InlineQueryResultsButton
from .input_checklist import InputChecklist
from .input_checklist_task import InputChecklistTask
from .input_contact_message_content import InputContactMessageContent
from .input_file import BufferedInputFile, FSInputFile, InputFile, URLInputFile
from .input_file_union import InputFileUnion
from .input_invoice_message_content import InputInvoiceMessageContent
from .input_location_message_content import InputLocationMessageContent
from .input_media import InputMedia
from .input_media_animation import InputMediaAnimation
from .input_media_audio import InputMediaAudio
from .input_media_document import InputMediaDocument
from .input_media_photo import InputMediaPhoto
from .input_media_union import InputMediaUnion
from .input_media_video import InputMediaVideo
from .input_message_content import InputMessageContent
from .input_message_content_union import InputMessageContentUnion
from .input_paid_media import InputPaidMedia
from .input_paid_media_photo import InputPaidMediaPhoto
from .input_paid_media_union import InputPaidMediaUnion
from .input_paid_media_video import InputPaidMediaVideo
from .input_poll_option import InputPollOption
from .input_poll_option_union import InputPollOptionUnion
from .input_profile_photo import InputProfilePhoto
from .input_profile_photo_animated import InputProfilePhotoAnimated
from .input_profile_photo_static import InputProfilePhotoStatic
from .input_profile_photo_union import InputProfilePhotoUnion
from .input_sticker import InputSticker
from .input_story_content import InputStoryContent
from .input_story_content_photo import InputStoryContentPhoto
from .input_story_content_union import InputStoryContentUnion
from .input_story_content_video import InputStoryContentVideo
from .input_text_message_content import InputTextMessageContent
from .input_venue_message_content import InputVenueMessageContent
from .invoice import Invoice
from .keyboard_button import KeyboardButton
from .keyboard_button_poll_type import KeyboardButtonPollType
from .keyboard_button_request_chat import KeyboardButtonRequestChat
from .keyboard_button_request_user import KeyboardButtonRequestUser
from .keyboard_button_request_users import KeyboardButtonRequestUsers
from .labeled_price import LabeledPrice
from .link_preview_options import LinkPreviewOptions
from .location import Location
from .location_address import LocationAddress
from .login_url import LoginUrl
from .mask_position import MaskPosition
from .maybe_inaccessible_message import MaybeInaccessibleMessage
from .maybe_inaccessible_message_union import MaybeInaccessibleMessageUnion
from .media_union import MediaUnion
from .menu_button import MenuButton
from .menu_button_commands import MenuButtonCommands
from .menu_button_default import MenuButtonDefault
from .menu_button_union import MenuButtonUnion
from .menu_button_web_app import MenuButtonWebApp
from .message import ContentType, Message
from .message_auto_delete_timer_changed import MessageAutoDeleteTimerChanged
from .message_entity import MessageEntity
from .message_id import MessageId
from .message_origin import MessageOrigin
from .message_origin_channel import MessageOriginChannel
from .message_origin_chat import MessageOriginChat
from .message_origin_hidden_user import MessageOriginHiddenUser
from .message_origin_union import MessageOriginUnion
from .message_origin_user import MessageOriginUser
from .message_reaction_count_updated import MessageReactionCountUpdated
from .message_reaction_updated import MessageReactionUpdated
from .order_info import OrderInfo
from .owned_gift import OwnedGift
from .owned_gift_regular import OwnedGiftRegular
from .owned_gift_union import OwnedGiftUnion
from .owned_gift_unique import OwnedGiftUnique
from .owned_gifts import OwnedGifts
from .paid_media import PaidMedia
from .paid_media_info import PaidMediaInfo
from .paid_media_photo import PaidMediaPhoto
from .paid_media_preview import PaidMediaPreview
from .paid_media_purchased import PaidMediaPurchased
from .paid_media_union import PaidMediaUnion
from .paid_media_video import PaidMediaVideo
from .paid_message_price_changed import PaidMessagePriceChanged
from .passport_data import PassportData
from .passport_element_error import PassportElementError
from .passport_element_error_data_field import PassportElementErrorDataField
from .passport_element_error_file import PassportElementErrorFile
from .passport_element_error_files import PassportElementErrorFiles
from .passport_element_error_front_side import PassportElementErrorFrontSide
from .passport_element_error_reverse_side import PassportElementErrorReverseSide
from .passport_element_error_selfie import PassportElementErrorSelfie
from .passport_element_error_translation_file import PassportElementErrorTranslationFile
from .passport_element_error_translation_files import (
    PassportElementErrorTranslationFiles,
)
from .passport_element_error_union import PassportElementErrorUnion
from .passport_element_error_unspecified import PassportElementErrorUnspecified
from .passport_file import PassportFile
from .photo_size import PhotoSize
from .poll import Poll
from .poll_answer import PollAnswer
from .poll_option import PollOption
from .pre_checkout_query import PreCheckoutQuery
from .prepared_inline_message import PreparedInlineMessage
from .proximity_alert_triggered import ProximityAlertTriggered
from .reaction_count import ReactionCount
from .reaction_type import ReactionType
from .reaction_type_custom_emoji import ReactionTypeCustomEmoji
from .reaction_type_emoji import ReactionTypeEmoji
from .reaction_type_paid import ReactionTypePaid
from .reaction_type_union import ReactionTypeUnion
from .refunded_payment import RefundedPayment
from .reply_keyboard_markup import ReplyKeyboardMarkup
from .reply_keyboard_remove import ReplyKeyboardRemove
from .reply_markup_union import ReplyMarkupUnion
from .reply_parameters import ReplyParameters
from .response_parameters import ResponseParameters
from .result_chat_member_union import ResultChatMemberUnion
from .result_menu_button_union import ResultMenuButtonUnion
from .revenue_withdrawal_state import RevenueWithdrawalState
from .revenue_withdrawal_state_failed import RevenueWithdrawalStateFailed
from .revenue_withdrawal_state_pending import RevenueWithdrawalStatePending
from .revenue_withdrawal_state_succeeded import RevenueWithdrawalStateSucceeded
from .revenue_withdrawal_state_union import RevenueWithdrawalStateUnion
from .sent_web_app_message import SentWebAppMessage
from .shared_user import SharedUser
from .shipping_address import ShippingAddress
from .shipping_option import ShippingOption
from .shipping_query import ShippingQuery
from .star_amount import StarAmount
from .star_transaction import StarTransaction
from .star_transactions import StarTransactions
from .sticker import Sticker
from .sticker_set import StickerSet
from .story import Story
from .story_area import StoryArea
from .story_area_position import StoryAreaPosition
from .story_area_type import StoryAreaType
from .story_area_type_link import StoryAreaTypeLink
from .story_area_type_location import StoryAreaTypeLocation
from .story_area_type_suggested_reaction import StoryAreaTypeSuggestedReaction
from .story_area_type_union import StoryAreaTypeUnion
from .story_area_type_unique_gift import StoryAreaTypeUniqueGift
from .story_area_type_weather import StoryAreaTypeWeather
from .successful_payment import SuccessfulPayment
from .suggested_post_approval_failed import SuggestedPostApprovalFailed
from .suggested_post_approved import SuggestedPostApproved
from .suggested_post_declined import SuggestedPostDeclined
from .suggested_post_info import SuggestedPostInfo
from .suggested_post_paid import SuggestedPostPaid
from .suggested_post_parameters import SuggestedPostParameters
from .suggested_post_price import SuggestedPostPrice
from .suggested_post_refunded import SuggestedPostRefunded
from .switch_inline_query_chosen_chat import SwitchInlineQueryChosenChat
from .text_quote import TextQuote
from .transaction_partner import TransactionPartner
from .transaction_partner_affiliate_program import TransactionPartnerAffiliateProgram
from .transaction_partner_chat import TransactionPartnerChat
from .transaction_partner_fragment import TransactionPartnerFragment
from .transaction_partner_other import TransactionPartnerOther
from .transaction_partner_telegram_ads import TransactionPartnerTelegramAds
from .transaction_partner_telegram_api import TransactionPartnerTelegramApi
from .transaction_partner_union import TransactionPartnerUnion
from .transaction_partner_user import TransactionPartnerUser
from .unique_gift import UniqueGift
from .unique_gift_backdrop import UniqueGiftBackdrop
from .unique_gift_backdrop_colors import UniqueGiftBackdropColors
from .unique_gift_colors import UniqueGiftColors
from .unique_gift_info import UniqueGiftInfo
from .unique_gift_model import UniqueGiftModel
from .unique_gift_symbol import UniqueGiftSymbol
from .update import Update
from .user import User
from .user_chat_boosts import UserChatBoosts
from .user_profile_photos import UserProfilePhotos
from .user_rating import UserRating
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
from .web_app_info import WebAppInfo
from .webhook_info import WebhookInfo
from .write_access_allowed import WriteAccessAllowed

__all__ = (
    "AcceptedGiftTypes",
    "AffiliateInfo",
    "Animation",
    "Audio",
    "BackgroundFill",
    "BackgroundFillFreeformGradient",
    "BackgroundFillGradient",
    "BackgroundFillSolid",
    "BackgroundFillUnion",
    "BackgroundType",
    "BackgroundTypeChatTheme",
    "BackgroundTypeFill",
    "BackgroundTypePattern",
    "BackgroundTypeUnion",
    "BackgroundTypeWallpaper",
    "Birthdate",
    "BotAccessSettings",
    "BotCommand",
    "BotCommandScope",
    "BotCommandScopeAllChatAdministrators",
    "BotCommandScopeAllGroupChats",
    "BotCommandScopeAllPrivateChats",
    "BotCommandScopeChat",
    "BotCommandScopeChatAdministrators",
    "BotCommandScopeChatMember",
    "BotCommandScopeDefault",
    "BotCommandScopeUnion",
    "BotDescription",
    "BotName",
    "BotShortDescription",
    "BufferedInputFile",
    "BusinessBotRights",
    "BusinessConnection",
    "BusinessIntro",
    "BusinessLocation",
    "BusinessMessagesDeleted",
    "BusinessOpeningHours",
    "BusinessOpeningHoursInterval",
    "CallbackGame",
    "CallbackQuery",
    "Chat",
    "ChatAdministratorRights",
    "ChatBackground",
    "ChatBoost",
    "ChatBoostAdded",
    "ChatBoostRemoved",
    "ChatBoostSource",
    "ChatBoostSourceGiftCode",
    "ChatBoostSourceGiveaway",
    "ChatBoostSourcePremium",
    "ChatBoostSourceUnion",
    "ChatBoostUpdated",
    "ChatFullInfo",
    "ChatIdUnion",
    "ChatInviteLink",
    "ChatJoinRequest",
    "ChatLocation",
    "ChatMember",
    "ChatMemberAdministrator",
    "ChatMemberBanned",
    "ChatMemberLeft",
    "ChatMemberMember",
    "ChatMemberOwner",
    "ChatMemberRestricted",
    "ChatMemberUnion",
    "ChatMemberUpdated",
    "ChatOwnerChanged",
    "ChatOwnerLeft",
    "ChatPermissions",
    "ChatPhoto",
    "ChatShared",
    "Checklist",
    "ChecklistTask",
    "ChecklistTasksAdded",
    "ChecklistTasksDone",
    "ChosenInlineResult",
    "Contact",
    "ContentType",
    "CopyTextButton",
    "DateTime",
    "DateTimeUnion",
    "Dice",
    "DirectMessagePriceChanged",
    "DirectMessagesTopic",
    "Document",
    "Downloadable",
    "EncryptedCredentials",
    "EncryptedPassportElement",
    "ErrorEvent",
    "ExternalReplyInfo",
    "FSInputFile",
    "File",
    "ForceReply",
    "ForumTopic",
    "ForumTopicClosed",
    "ForumTopicCreated",
    "ForumTopicEdited",
    "ForumTopicReopened",
    "Game",
    "GameHighScore",
    "GeneralForumTopicHidden",
    "GeneralForumTopicUnhidden",
    "Gift",
    "GiftBackground",
    "GiftInfo",
    "Gifts",
    "Giveaway",
    "GiveawayCompleted",
    "GiveawayCreated",
    "GiveawayWinners",
    "InaccessibleMessage",
    "InlineKeyboardButton",
    "InlineKeyboardMarkup",
    "InlineQuery",
    "InlineQueryResult",
    "InlineQueryResultArticle",
    "InlineQueryResultAudio",
    "InlineQueryResultCachedAudio",
    "InlineQueryResultCachedDocument",
    "InlineQueryResultCachedGif",
    "InlineQueryResultCachedMpeg4Gif",
    "InlineQueryResultCachedPhoto",
    "InlineQueryResultCachedSticker",
    "InlineQueryResultCachedVideo",
    "InlineQueryResultCachedVoice",
    "InlineQueryResultContact",
    "InlineQueryResultDocument",
    "InlineQueryResultGame",
    "InlineQueryResultGif",
    "InlineQueryResultLocation",
    "InlineQueryResultMpeg4Gif",
    "InlineQueryResultPhoto",
    "InlineQueryResultUnion",
    "InlineQueryResultVenue",
    "InlineQueryResultVideo",
    "InlineQueryResultVoice",
    "InlineQueryResultsButton",
    "InputChecklist",
    "InputChecklistTask",
    "InputContactMessageContent",
    "InputFile",
    "InputFileUnion",
    "InputInvoiceMessageContent",
    "InputLocationMessageContent",
    "InputMedia",
    "InputMediaAnimation",
    "InputMediaAudio",
    "InputMediaDocument",
    "InputMediaLink",
    "InputMediaLivePhoto",
    "InputMediaLocation",
    "InputMediaPhoto",
    "InputMediaSticker",
    "InputMediaUnion",
    "InputMediaVenue",
    "InputMediaVideo",
    "InputMessageContent",
    "InputMessageContentUnion",
    "InputPaidMedia",
    "InputPaidMediaLivePhoto",
    "InputPaidMediaPhoto",
    "InputPaidMediaUnion",
    "InputPaidMediaVideo",
    "InputPollMedia",
    "InputPollMediaUnion",
    "InputPollOption",
    "InputPollOptionMedia",
    "InputPollOptionMediaUnion",
    "InputPollOptionUnion",
    "InputProfilePhoto",
    "InputProfilePhotoAnimated",
    "InputProfilePhotoStatic",
    "InputProfilePhotoUnion",
    "InputRichMessage",
    "InputRichMessageContent",
    "InputSticker",
    "InputStoryContent",
    "InputStoryContentPhoto",
    "InputStoryContentUnion",
    "InputStoryContentVideo",
    "InputTextMessageContent",
    "InputVenueMessageContent",
    "Invoice",
    "KeyboardButton",
    "KeyboardButtonPollType",
    "KeyboardButtonRequestChat",
    "KeyboardButtonRequestManagedBot",
    "KeyboardButtonRequestUser",
    "KeyboardButtonRequestUsers",
    "LabeledPrice",
    "Link",
    "LinkPreviewOptions",
    "LivePhoto",
    "Location",
    "LocationAddress",
    "LoginUrl",
    "ManagedBotCreated",
    "ManagedBotUpdated",
    "MaskPosition",
    "MaybeInaccessibleMessage",
    "MaybeInaccessibleMessageUnion",
    "MediaUnion",
    "MenuButton",
    "MenuButtonCommands",
    "MenuButtonDefault",
    "MenuButtonUnion",
    "MenuButtonWebApp",
    "Message",
    "MessageAutoDeleteTimerChanged",
    "MessageEntity",
    "MessageId",
    "MessageOrigin",
    "MessageOriginChannel",
    "MessageOriginChat",
    "MessageOriginHiddenUser",
    "MessageOriginUnion",
    "MessageOriginUser",
    "MessageReactionCountUpdated",
    "MessageReactionUpdated",
    "OrderInfo",
    "OwnedGift",
    "OwnedGiftRegular",
    "OwnedGiftUnion",
    "OwnedGiftUnique",
    "OwnedGifts",
    "PaidMedia",
    "PaidMediaInfo",
    "PaidMediaLivePhoto",
    "PaidMediaPhoto",
    "PaidMediaPreview",
    "PaidMediaPurchased",
    "PaidMediaUnion",
    "PaidMediaVideo",
    "PaidMessagePriceChanged",
    "PassportData",
    "PassportElementError",
    "PassportElementErrorDataField",
    "PassportElementErrorFile",
    "PassportElementErrorFiles",
    "PassportElementErrorFrontSide",
    "PassportElementErrorReverseSide",
    "PassportElementErrorSelfie",
    "PassportElementErrorTranslationFile",
    "PassportElementErrorTranslationFiles",
    "PassportElementErrorUnion",
    "PassportElementErrorUnspecified",
    "PassportFile",
    "PhotoSize",
    "Poll",
    "PollAnswer",
    "PollMedia",
    "PollOption",
    "PollOptionAdded",
    "PollOptionDeleted",
    "PreCheckoutQuery",
    "PreparedInlineMessage",
    "PreparedKeyboardButton",
    "ProximityAlertTriggered",
    "ReactionCount",
    "ReactionType",
    "ReactionTypeCustomEmoji",
    "ReactionTypeEmoji",
    "ReactionTypePaid",
    "ReactionTypeUnion",
    "RefundedPayment",
    "ReplyKeyboardMarkup",
    "ReplyKeyboardRemove",
    "ReplyMarkupUnion",
    "ReplyParameters",
    "ResponseParameters",
    "ResultChatMemberUnion",
    "ResultMenuButtonUnion",
    "RevenueWithdrawalState",
    "RevenueWithdrawalStateFailed",
    "RevenueWithdrawalStatePending",
    "RevenueWithdrawalStateSucceeded",
    "RevenueWithdrawalStateUnion",
    "RichBlock",
    "RichBlockAnchor",
    "RichBlockAnimation",
    "RichBlockAudio",
    "RichBlockBlockQuotation",
    "RichBlockCaption",
    "RichBlockCollage",
    "RichBlockDetails",
    "RichBlockDivider",
    "RichBlockFooter",
    "RichBlockList",
    "RichBlockListItem",
    "RichBlockMap",
    "RichBlockMathematicalExpression",
    "RichBlockParagraph",
    "RichBlockPhoto",
    "RichBlockPreformatted",
    "RichBlockPullQuotation",
    "RichBlockSectionHeading",
    "RichBlockSlideshow",
    "RichBlockTable",
    "RichBlockTableCell",
    "RichBlockThinking",
    "RichBlockUnion",
    "RichBlockVideo",
    "RichBlockVoiceNote",
    "RichMessage",
    "RichText",
    "RichTextAnchor",
    "RichTextAnchorLink",
    "RichTextBankCardNumber",
    "RichTextBold",
    "RichTextBotCommand",
    "RichTextCashtag",
    "RichTextCode",
    "RichTextCustomEmoji",
    "RichTextDateTime",
    "RichTextEmailAddress",
    "RichTextHashtag",
    "RichTextItalic",
    "RichTextMarked",
    "RichTextMathematicalExpression",
    "RichTextMention",
    "RichTextPhoneNumber",
    "RichTextReference",
    "RichTextReferenceLink",
    "RichTextSpoiler",
    "RichTextStrikethrough",
    "RichTextSubscript",
    "RichTextSuperscript",
    "RichTextTextMention",
    "RichTextUnderline",
    "RichTextUnion",
    "RichTextUrl",
    "SentGuestMessage",
    "SentWebAppMessage",
    "SharedUser",
    "ShippingAddress",
    "ShippingOption",
    "ShippingQuery",
    "StarAmount",
    "StarTransaction",
    "StarTransactions",
    "Sticker",
    "StickerSet",
    "Story",
    "StoryArea",
    "StoryAreaPosition",
    "StoryAreaType",
    "StoryAreaTypeLink",
    "StoryAreaTypeLocation",
    "StoryAreaTypeSuggestedReaction",
    "StoryAreaTypeUnion",
    "StoryAreaTypeUniqueGift",
    "StoryAreaTypeWeather",
    "SuccessfulPayment",
    "SuggestedPostApprovalFailed",
    "SuggestedPostApproved",
    "SuggestedPostDeclined",
    "SuggestedPostInfo",
    "SuggestedPostPaid",
    "SuggestedPostParameters",
    "SuggestedPostPrice",
    "SuggestedPostRefunded",
    "SwitchInlineQueryChosenChat",
    "TelegramObject",
    "TextQuote",
    "TransactionPartner",
    "TransactionPartnerAffiliateProgram",
    "TransactionPartnerChat",
    "TransactionPartnerFragment",
    "TransactionPartnerOther",
    "TransactionPartnerTelegramAds",
    "TransactionPartnerTelegramApi",
    "TransactionPartnerUnion",
    "TransactionPartnerUser",
    "UNSET_PARSE_MODE",
    "URLInputFile",
    "UniqueGift",
    "UniqueGiftBackdrop",
    "UniqueGiftBackdropColors",
    "UniqueGiftColors",
    "UniqueGiftInfo",
    "UniqueGiftModel",
    "UniqueGiftSymbol",
    "Update",
    "User",
    "UserChatBoosts",
    "UserProfileAudios",
    "UserProfilePhotos",
    "UserRating",
    "UserShared",
    "UsersShared",
    "Venue",
    "Video",
    "VideoChatEnded",
    "VideoChatParticipantsInvited",
    "VideoChatScheduled",
    "VideoChatStarted",
    "VideoNote",
    "VideoQuality",
    "Voice",
    "WebAppData",
    "WebAppInfo",
    "WebhookInfo",
    "WriteAccessAllowed",
)

from ..client.default import Default as _Default
from .bot_access_settings import BotAccessSettings
from .chat_owner_changed import ChatOwnerChanged
from .chat_owner_left import ChatOwnerLeft
from .input_media_link import InputMediaLink
from .input_media_live_photo import InputMediaLivePhoto
from .input_media_location import InputMediaLocation
from .input_media_sticker import InputMediaSticker
from .input_media_venue import InputMediaVenue
from .input_paid_media_live_photo import InputPaidMediaLivePhoto
from .input_poll_media import InputPollMedia
from .input_poll_media_union import InputPollMediaUnion
from .input_poll_option_media import InputPollOptionMedia
from .input_poll_option_media_union import InputPollOptionMediaUnion
from .input_rich_message import InputRichMessage
from .input_rich_message_content import InputRichMessageContent
from .keyboard_button_request_managed_bot import KeyboardButtonRequestManagedBot
from .link import Link
from .live_photo import LivePhoto
from .managed_bot_created import ManagedBotCreated
from .managed_bot_updated import ManagedBotUpdated
from .paid_media_live_photo import PaidMediaLivePhoto
from .poll_media import PollMedia
from .poll_option_added import PollOptionAdded
from .poll_option_deleted import PollOptionDeleted
from .prepared_keyboard_button import PreparedKeyboardButton
from .rich_block import RichBlock
from .rich_block_anchor import RichBlockAnchor
from .rich_block_animation import RichBlockAnimation
from .rich_block_audio import RichBlockAudio
from .rich_block_block_quotation import RichBlockBlockQuotation
from .rich_block_caption import RichBlockCaption
from .rich_block_collage import RichBlockCollage
from .rich_block_details import RichBlockDetails
from .rich_block_divider import RichBlockDivider
from .rich_block_footer import RichBlockFooter
from .rich_block_list import RichBlockList
from .rich_block_list_item import RichBlockListItem
from .rich_block_map import RichBlockMap
from .rich_block_mathematical_expression import RichBlockMathematicalExpression
from .rich_block_paragraph import RichBlockParagraph
from .rich_block_photo import RichBlockPhoto
from .rich_block_preformatted import RichBlockPreformatted
from .rich_block_pull_quotation import RichBlockPullQuotation
from .rich_block_section_heading import RichBlockSectionHeading
from .rich_block_slideshow import RichBlockSlideshow
from .rich_block_table import RichBlockTable
from .rich_block_table_cell import RichBlockTableCell
from .rich_block_thinking import RichBlockThinking
from .rich_block_union import RichBlockUnion
from .rich_block_video import RichBlockVideo
from .rich_block_voice_note import RichBlockVoiceNote
from .rich_message import RichMessage
from .rich_text import RichText
from .rich_text_anchor import RichTextAnchor
from .rich_text_anchor_link import RichTextAnchorLink
from .rich_text_bank_card_number import RichTextBankCardNumber
from .rich_text_bold import RichTextBold
from .rich_text_bot_command import RichTextBotCommand
from .rich_text_cashtag import RichTextCashtag
from .rich_text_code import RichTextCode
from .rich_text_custom_emoji import RichTextCustomEmoji
from .rich_text_date_time import RichTextDateTime
from .rich_text_email_address import RichTextEmailAddress
from .rich_text_hashtag import RichTextHashtag
from .rich_text_italic import RichTextItalic
from .rich_text_marked import RichTextMarked
from .rich_text_mathematical_expression import RichTextMathematicalExpression
from .rich_text_mention import RichTextMention
from .rich_text_phone_number import RichTextPhoneNumber
from .rich_text_reference import RichTextReference
from .rich_text_reference_link import RichTextReferenceLink
from .rich_text_spoiler import RichTextSpoiler
from .rich_text_strikethrough import RichTextStrikethrough
from .rich_text_subscript import RichTextSubscript
from .rich_text_superscript import RichTextSuperscript
from .rich_text_text_mention import RichTextTextMention
from .rich_text_underline import RichTextUnderline
from .rich_text_union import RichTextUnion
from .rich_text_url import RichTextUrl
from .sent_guest_message import SentGuestMessage
from .user_profile_audios import UserProfileAudios
from .video_quality import VideoQuality

# Load typing forward refs for every TelegramObject
for _entity_name in __all__:
    _entity = globals()[_entity_name]
    if not hasattr(_entity, "model_rebuild"):
        continue
    _entity.model_rebuild(
        _types_namespace={
            "List": list,
            "Optional": Optional,
            "Union": Union,
            "Literal": Literal,
            "Default": _Default,
            **{k: v for k, v in globals().items() if k in __all__},
        }
    )

del _entity
del _entity_name
