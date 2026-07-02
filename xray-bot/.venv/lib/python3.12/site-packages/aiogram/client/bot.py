from __future__ import annotations

import io
import pathlib
from collections.abc import AsyncGenerator, AsyncIterator
from contextlib import asynccontextmanager
from types import TracebackType
from typing import (
    Any,
    BinaryIO,
    TypeVar,
    cast,
)

import aiofiles

from aiogram.utils.token import extract_bot_id, validate_token

from ..methods import (
    AddStickerToSet,
    AnswerCallbackQuery,
    AnswerChatJoinRequestQuery,
    AnswerGuestQuery,
    AnswerInlineQuery,
    AnswerPreCheckoutQuery,
    AnswerShippingQuery,
    AnswerWebAppQuery,
    ApproveChatJoinRequest,
    ApproveSuggestedPost,
    BanChatMember,
    BanChatSenderChat,
    Close,
    CloseForumTopic,
    CloseGeneralForumTopic,
    ConvertGiftToStars,
    CopyMessage,
    CopyMessages,
    CreateChatInviteLink,
    CreateChatSubscriptionInviteLink,
    CreateForumTopic,
    CreateInvoiceLink,
    CreateNewStickerSet,
    DeclineChatJoinRequest,
    DeclineSuggestedPost,
    DeleteAllMessageReactions,
    DeleteBusinessMessages,
    DeleteChatPhoto,
    DeleteChatStickerSet,
    DeleteForumTopic,
    DeleteMessage,
    DeleteMessageReaction,
    DeleteMessages,
    DeleteMyCommands,
    DeleteStickerFromSet,
    DeleteStickerSet,
    DeleteStory,
    DeleteWebhook,
    EditChatInviteLink,
    EditChatSubscriptionInviteLink,
    EditForumTopic,
    EditGeneralForumTopic,
    EditMessageCaption,
    EditMessageChecklist,
    EditMessageLiveLocation,
    EditMessageMedia,
    EditMessageReplyMarkup,
    EditMessageText,
    EditStory,
    EditUserStarSubscription,
    ExportChatInviteLink,
    ForwardMessage,
    ForwardMessages,
    GetAvailableGifts,
    GetBusinessAccountGifts,
    GetBusinessAccountStarBalance,
    GetBusinessConnection,
    GetChat,
    GetChatAdministrators,
    GetChatGifts,
    GetChatMember,
    GetChatMemberCount,
    GetChatMenuButton,
    GetCustomEmojiStickers,
    GetFile,
    GetForumTopicIconStickers,
    GetGameHighScores,
    GetManagedBotAccessSettings,
    GetManagedBotToken,
    GetMe,
    GetMyCommands,
    GetMyDefaultAdministratorRights,
    GetMyDescription,
    GetMyName,
    GetMyShortDescription,
    GetMyStarBalance,
    GetStarTransactions,
    GetStickerSet,
    GetUpdates,
    GetUserChatBoosts,
    GetUserGifts,
    GetUserPersonalChatMessages,
    GetUserProfileAudios,
    GetUserProfilePhotos,
    GetWebhookInfo,
    GiftPremiumSubscription,
    HideGeneralForumTopic,
    LeaveChat,
    LogOut,
    PinChatMessage,
    PostStory,
    PromoteChatMember,
    ReadBusinessMessage,
    RefundStarPayment,
    RemoveBusinessAccountProfilePhoto,
    RemoveChatVerification,
    RemoveMyProfilePhoto,
    RemoveUserVerification,
    ReopenForumTopic,
    ReopenGeneralForumTopic,
    ReplaceManagedBotToken,
    ReplaceStickerInSet,
    RepostStory,
    RestrictChatMember,
    RevokeChatInviteLink,
    SavePreparedInlineMessage,
    SavePreparedKeyboardButton,
    SendAnimation,
    SendAudio,
    SendChatAction,
    SendChatJoinRequestWebApp,
    SendChecklist,
    SendContact,
    SendDice,
    SendDocument,
    SendGame,
    SendGift,
    SendInvoice,
    SendLivePhoto,
    SendLocation,
    SendMediaGroup,
    SendMessage,
    SendMessageDraft,
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
    SetBusinessAccountBio,
    SetBusinessAccountGiftSettings,
    SetBusinessAccountName,
    SetBusinessAccountProfilePhoto,
    SetBusinessAccountUsername,
    SetChatAdministratorCustomTitle,
    SetChatDescription,
    SetChatMemberTag,
    SetChatMenuButton,
    SetChatPermissions,
    SetChatPhoto,
    SetChatStickerSet,
    SetChatTitle,
    SetCustomEmojiStickerSetThumbnail,
    SetGameScore,
    SetManagedBotAccessSettings,
    SetMessageReaction,
    SetMyCommands,
    SetMyDefaultAdministratorRights,
    SetMyDescription,
    SetMyName,
    SetMyProfilePhoto,
    SetMyShortDescription,
    SetPassportDataErrors,
    SetStickerEmojiList,
    SetStickerKeywords,
    SetStickerMaskPosition,
    SetStickerPositionInSet,
    SetStickerSetThumbnail,
    SetStickerSetTitle,
    SetUserEmojiStatus,
    SetWebhook,
    StopMessageLiveLocation,
    StopPoll,
    TelegramMethod,
    TransferBusinessAccountStars,
    TransferGift,
    UnbanChatMember,
    UnbanChatSenderChat,
    UnhideGeneralForumTopic,
    UnpinAllChatMessages,
    UnpinAllForumTopicMessages,
    UnpinAllGeneralForumTopicMessages,
    UnpinChatMessage,
    UpgradeGift,
    UploadStickerFile,
    VerifyChat,
    VerifyUser,
)
from ..types import (
    AcceptedGiftTypes,
    BotAccessSettings,
    BotCommand,
    BotCommandScopeUnion,
    BotDescription,
    BotName,
    BotShortDescription,
    BusinessConnection,
    ChatAdministratorRights,
    ChatFullInfo,
    ChatIdUnion,
    ChatInviteLink,
    ChatPermissions,
    DateTimeUnion,
    Downloadable,
    File,
    ForumTopic,
    GameHighScore,
    Gifts,
    InlineKeyboardMarkup,
    InlineQueryResultsButton,
    InlineQueryResultUnion,
    InputChecklist,
    InputFile,
    InputFileUnion,
    InputMediaUnion,
    InputPaidMediaUnion,
    InputPollMedia,
    InputPollMediaUnion,
    InputPollOptionUnion,
    InputProfilePhotoUnion,
    InputRichMessage,
    InputSticker,
    InputStoryContentUnion,
    KeyboardButton,
    LabeledPrice,
    LinkPreviewOptions,
    MaskPosition,
    MediaUnion,
    MenuButtonUnion,
    Message,
    MessageEntity,
    MessageId,
    OwnedGifts,
    PassportElementErrorUnion,
    Poll,
    PreparedInlineMessage,
    PreparedKeyboardButton,
    ReactionTypeUnion,
    ReplyMarkupUnion,
    ReplyParameters,
    ResultChatMemberUnion,
    ResultMenuButtonUnion,
    SentGuestMessage,
    SentWebAppMessage,
    ShippingOption,
    StarAmount,
    StarTransactions,
    Sticker,
    StickerSet,
    Story,
    StoryArea,
    SuggestedPostParameters,
    Update,
    User,
    UserChatBoosts,
    UserProfileAudios,
    UserProfilePhotos,
    WebhookInfo,
)
from .default import Default, DefaultBotProperties
from .session.aiohttp import AiohttpSession
from .session.base import BaseSession

T = TypeVar("T")


class Bot:
    def __init__(
        self,
        token: str,
        session: BaseSession | None = None,
        default: DefaultBotProperties | None = None,
        **kwargs: Any,
    ) -> None:
        """
        Bot class

        :param token: Telegram Bot token `Obtained from @BotFather <https://t.me/BotFather>`_
        :param session: HTTP Client session (For example AiohttpSession).
            If not specified it will be automatically created.
        :param default: Default bot properties.
            If specified it will be propagated into the API methods at runtime.
        :raise TokenValidationError: When token has invalid format this exception will be raised
        """

        validate_token(token)

        if session is None:
            session = AiohttpSession()
        if default is None:
            default = DefaultBotProperties()

        self.session = session

        # Few arguments are completely removed in 3.7.0 version
        # Temporary solution to raise an error if user passed these arguments
        # with explanation how to fix it
        parse_mode = kwargs.get("parse_mode")
        link_preview_is_disabled = kwargs.get("disable_web_page_preview")
        protect_content = kwargs.get("protect_content")
        if (
            parse_mode is not None
            or link_preview_is_disabled is not None
            or protect_content is not None
        ):
            example_kwargs = {
                "parse_mode": parse_mode,
                "link_preview_is_disabled": link_preview_is_disabled,
                "protect_content": protect_content,
            }
            replacement_spec = ", ".join(
                f"{k}={v!r}" for k, v in example_kwargs.items() if v is not None
            )
            raise TypeError(
                "Passing `parse_mode`, `disable_web_page_preview` or `protect_content` "
                "to Bot initializer is not supported anymore. These arguments have been removed "
                f"in 3.7.0 version. Use `default=DefaultBotProperties({replacement_spec})` argument instead."
            )

        self.default = default

        self.__token = token
        self._me: User | None = None

    async def __aenter__(self) -> Bot:
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc_value: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        await self.session.close()

    @property
    def token(self) -> str:
        return self.__token

    @property
    def id(self) -> int:
        """
        Get bot ID from token

        :return:
        """
        return extract_bot_id(self.__token)

    @asynccontextmanager
    async def context(self, auto_close: bool = True) -> AsyncIterator[Bot]:
        """
        Generate bot context

        :param auto_close: close session on exit
        :return:
        """
        try:
            yield self
        finally:
            if auto_close:
                await self.session.close()

    async def me(self) -> User:
        """
        Cached alias for getMe method

        :return:
        """
        if self._me is None:  # pragma: no cover
            self._me = await self.get_me()
        return self._me

    @classmethod
    async def __download_file_binary_io(
        cls, destination: BinaryIO, seek: bool, stream: AsyncGenerator[bytes, None]
    ) -> BinaryIO:
        async for chunk in stream:
            destination.write(chunk)
            destination.flush()
        if seek is True:
            destination.seek(0)
        return destination

    @classmethod
    async def __download_file(
        cls, destination: str | pathlib.Path, stream: AsyncGenerator[bytes, None]
    ) -> None:
        async with aiofiles.open(destination, "wb") as f:
            async for chunk in stream:
                await f.write(chunk)

    @classmethod
    async def __aiofiles_reader(
        cls, file: str | pathlib.Path, chunk_size: int = 65536
    ) -> AsyncGenerator[bytes, None]:
        async with aiofiles.open(file, "rb") as f:
            while chunk := await f.read(chunk_size):
                yield chunk

    async def download_file(
        self,
        file_path: str | pathlib.Path,
        destination: BinaryIO | pathlib.Path | str | None = None,
        timeout: int = 30,
        chunk_size: int = 65536,
        seek: bool = True,
    ) -> BinaryIO | None:
        """
        Download file by file_path to destination.

        If you want to automatically create destination (:class:`io.BytesIO`) use default
        value of destination and handle result of this method.

        :param file_path: File path on Telegram server (You can get it from :obj:`aiogram.types.File`)
        :param destination: Filename, file path or instance of :class:`io.IOBase`. For e.g. :class:`io.BytesIO`, defaults to None
        :param timeout: Total timeout in seconds, defaults to 30
        :param chunk_size: File chunks size, defaults to 64 kb
        :param seek: Go to start of file when downloading is finished. Used only for destination with :class:`typing.BinaryIO` type, defaults to True
        """
        if destination is None:
            destination = io.BytesIO()

        close_stream = False
        if self.session.api.is_local:
            stream = self.__aiofiles_reader(
                self.session.api.wrap_local_file.to_local(file_path), chunk_size=chunk_size
            )
            close_stream = True
        else:
            url = self.session.api.file_url(self.__token, file_path)
            stream = self.session.stream_content(
                url=url,
                timeout=timeout,
                chunk_size=chunk_size,
                raise_for_status=True,
            )

        try:
            if isinstance(destination, (str, pathlib.Path)):
                await self.__download_file(destination=destination, stream=stream)
                return None
            return await self.__download_file_binary_io(
                destination=destination, seek=seek, stream=stream
            )
        finally:
            if close_stream:
                await stream.aclose()

    async def download(
        self,
        file: str | Downloadable,
        destination: BinaryIO | pathlib.Path | str | None = None,
        timeout: int = 30,
        chunk_size: int = 65536,
        seek: bool = True,
    ) -> BinaryIO | None:
        """
        Download file by file_id or Downloadable object to destination.

        If you want to automatically create destination (:class:`io.BytesIO`) use default
        value of destination and handle result of this method.

        :param file: file_id or Downloadable object
        :param destination: Filename, file path or instance of :class:`io.IOBase`. For e.g. :class:`io.BytesIO`, defaults to None
        :param timeout: Total timeout in seconds, defaults to 30
        :param chunk_size: File chunks size, defaults to 64 kb
        :param seek: Go to start of file when downloading is finished. Used only for destination with :class:`typing.BinaryIO` type, defaults to True
        """
        if isinstance(file, str):
            file_id = file
        else:
            # type is ignored in due to:
            # Incompatible types in assignment (expression has type "Any | None", variable has type "str")
            file_id = getattr(file, "file_id", None)  # type: ignore
            if file_id is None:
                raise TypeError("file can only be of the string or Downloadable type")

        file_ = await self.get_file(file_id)

        # `file_path` can be None for large files but this files can't be downloaded
        # So we need to do type-cast
        # https://github.com/aiogram/aiogram/pull/282/files#r394110017
        file_path = cast(str, file_.file_path)

        return await self.download_file(
            file_path, destination=destination, timeout=timeout, chunk_size=chunk_size, seek=seek
        )

    async def __call__(self, method: TelegramMethod[T], request_timeout: int | None = None) -> T:
        """
        Call API method

        :param method:
        :return:
        """
        return await self.session(self, method, timeout=request_timeout)

    def __hash__(self) -> int:
        """
        Get hash for the token

        :return:
        """
        return hash(self.__token)

    def __eq__(self, other: Any) -> bool:
        """
        Compare current bot with another bot instance

        :param other:
        :return:
        """
        if not isinstance(other, Bot):
            return False
        return hash(self) == hash(other)

    async def add_sticker_to_set(
        self,
        user_id: int,
        name: str,
        sticker: InputSticker,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to add a new sticker to a set created by the bot. Emoji sticker sets can have up to 200 stickers. Other sticker sets can have up to 120 stickers. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#addstickertoset

        :param user_id: User identifier of sticker set owner
        :param name: Sticker set name
        :param sticker: A JSON-serialized object with information about the added sticker. If exactly the same sticker had already been added to the set, then the set isn't changed.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = AddStickerToSet(
            user_id=user_id,
            name=name,
            sticker=sticker,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_callback_query(
        self,
        callback_query_id: str,
        text: str | None = None,
        show_alert: bool | None = None,
        url: str | None = None,
        cache_time: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to send answers to callback queries sent from `inline keyboards <https://core.telegram.org/bots/features#inline-keyboards>`_. The answer will be displayed to the user as a notification at the top of the chat screen or as an alert. On success, :code:`True` is returned.

         Alternatively, the user can be redirected to the specified Game URL. For this option to work, you must first create a game for your bot via `@BotFather <https://t.me/botfather>`_ and accept the terms. Otherwise, you may use links like :code:`t.me/your_bot?start=XXXX` that open your bot with a parameter.

        Source: https://core.telegram.org/bots/api#answercallbackquery

        :param callback_query_id: Unique identifier for the query to be answered
        :param text: Text of the notification. If not specified, nothing will be shown to the user, 0-200 characters.
        :param show_alert: If :code:`True`, an alert will be shown by the client instead of a notification at the top of the chat screen. Defaults to *false*.
        :param url: URL that will be opened by the user's client. If you have created a :class:`aiogram.types.game.Game` and accepted the conditions via `@BotFather <https://t.me/botfather>`_, specify the URL that opens your game - note that this will only work if the query comes from a `https://core.telegram.org/bots/api#inlinekeyboardbutton <https://core.telegram.org/bots/api#inlinekeyboardbutton>`_ *callback_game* button.
        :param cache_time: The maximum amount of time in seconds that the result of the callback query may be cached client-side. Telegram apps will support caching starting in version 3.14. Defaults to 0.
        :param request_timeout: Request timeout
        :return: Otherwise, you may use links like :code:`t.me/your_bot?start=XXXX` that open your bot with a parameter.
        """

        call = AnswerCallbackQuery(
            callback_query_id=callback_query_id,
            text=text,
            show_alert=show_alert,
            url=url,
            cache_time=cache_time,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_inline_query(
        self,
        inline_query_id: str,
        results: list[InlineQueryResultUnion],
        cache_time: int | None = None,
        is_personal: bool | None = None,
        next_offset: str | None = None,
        button: InlineQueryResultsButton | None = None,
        switch_pm_parameter: str | None = None,
        switch_pm_text: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to send answers to an inline query. On success, :code:`True` is returned.

        No more than **50** results per query are allowed.

        Source: https://core.telegram.org/bots/api#answerinlinequery

        :param inline_query_id: Unique identifier for the answered query
        :param results: A JSON-serialized array of results for the inline query
        :param cache_time: The maximum amount of time in seconds that the result of the inline query may be cached on the server. Defaults to 300.
        :param is_personal: Pass :code:`True` if results may be cached on the server side only for the user that sent the query. By default, results may be returned to any user who sends the same query.
        :param next_offset: Pass the offset that a client should send in the next query with the same text to receive more results. Pass an empty string if there are no more results or if you don't support pagination. Offset length can't exceed 64 bytes.
        :param button: A JSON-serialized object describing a button to be shown above inline query results
        :param switch_pm_parameter: `Deep-linking <https://core.telegram.org/bots/features#deep-linking>`_ parameter for the /start message sent to the bot when user presses the switch button. 1-64 characters, only :code:`A-Z`, :code:`a-z`, :code:`0-9`, :code:`_` and :code:`-` are allowed.
        :param switch_pm_text: If passed, clients will display a button with specified text that switches the user to a private chat with the bot and sends the bot a start message with the parameter *switch_pm_parameter*
        :param request_timeout: Request timeout
        :return: On success, :code:`True` is returned.
        """

        call = AnswerInlineQuery(
            inline_query_id=inline_query_id,
            results=results,
            cache_time=cache_time,
            is_personal=is_personal,
            next_offset=next_offset,
            button=button,
            switch_pm_parameter=switch_pm_parameter,
            switch_pm_text=switch_pm_text,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_pre_checkout_query(
        self,
        pre_checkout_query_id: str,
        ok: bool,
        error_message: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Once the user has confirmed their payment and shipping details, the Bot API sends the final confirmation in the form of an :class:`aiogram.types.update.Update` with the field *pre_checkout_query*. Use this method to respond to such pre-checkout queries. On success, :code:`True` is returned. **Note:** The Bot API must receive an answer within 10 seconds after the pre-checkout query was sent.

        Source: https://core.telegram.org/bots/api#answerprecheckoutquery

        :param pre_checkout_query_id: Unique identifier for the query to be answered
        :param ok: Specify :code:`True` if everything is alright (goods are available, etc.) and the bot is ready to proceed with the order. Use :code:`False` if there are any problems.
        :param error_message: Required if *ok* is :code:`False`. Error message in human readable form that explains the reason for failure to proceed with the checkout (e.g. "Sorry, somebody just bought the last of our amazing black T-shirts while you were busy filling out your payment details. Please choose a different color or garment!"). Telegram will display this message to the user.
        :param request_timeout: Request timeout
        :return: **Note:** The Bot API must receive an answer within 10 seconds after the pre-checkout query was sent.
        """

        call = AnswerPreCheckoutQuery(
            pre_checkout_query_id=pre_checkout_query_id,
            ok=ok,
            error_message=error_message,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_shipping_query(
        self,
        shipping_query_id: str,
        ok: bool,
        shipping_options: list[ShippingOption] | None = None,
        error_message: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        If you sent an invoice requesting a shipping address and the parameter *is_flexible* was specified, the Bot API will send an :class:`aiogram.types.update.Update` with a *shipping_query* field to the bot. Use this method to reply to shipping queries. On success, :code:`True` is returned.

        Source: https://core.telegram.org/bots/api#answershippingquery

        :param shipping_query_id: Unique identifier for the query to be answered
        :param ok: Pass :code:`True` if delivery to the specified address is possible and :code:`False` if there are any problems (for example, if delivery to the specified address is not possible)
        :param shipping_options: Required if *ok* is :code:`True`. A JSON-serialized array of available shipping options.
        :param error_message: Required if *ok* is :code:`False`. Error message in human readable form that explains why it is impossible to complete the order (e.g. 'Sorry, delivery to your desired address is unavailable'). Telegram will display this message to the user.
        :param request_timeout: Request timeout
        :return: On success, :code:`True` is returned.
        """

        call = AnswerShippingQuery(
            shipping_query_id=shipping_query_id,
            ok=ok,
            shipping_options=shipping_options,
            error_message=error_message,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_web_app_query(
        self,
        web_app_query_id: str,
        result: InlineQueryResultUnion,
        request_timeout: int | None = None,
    ) -> SentWebAppMessage:
        """
        Use this method to set the result of an interaction with a `Web App <https://core.telegram.org/bots/webapps>`_ and send a corresponding message on behalf of the user to the chat from which the query originated. On success, a :class:`aiogram.types.sent_web_app_message.SentWebAppMessage` object is returned.

        Source: https://core.telegram.org/bots/api#answerwebappquery

        :param web_app_query_id: Unique identifier for the query to be answered
        :param result: A JSON-serialized object describing the message to be sent
        :param request_timeout: Request timeout
        :return: On success, a :class:`aiogram.types.sent_web_app_message.SentWebAppMessage` object is returned.
        """

        call = AnswerWebAppQuery(
            web_app_query_id=web_app_query_id,
            result=result,
        )
        return await self(call, request_timeout=request_timeout)

    async def approve_chat_join_request(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to approve a chat join request. The bot must be an administrator in the chat for this to work and must have the *can_invite_users* administrator right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#approvechatjoinrequest

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ApproveChatJoinRequest(
            chat_id=chat_id,
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def ban_chat_member(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        until_date: DateTimeUnion | None = None,
        revoke_messages: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to ban a user in a group, a supergroup or a channel. In the case of supergroups and channels, the user will not be able to return to the chat on their own using invite links, etc., unless `unbanned <https://core.telegram.org/bots/api#unbanchatmember>`_ first. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#banchatmember

        :param chat_id: Unique identifier for the target group or username of the target supergroup or channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param until_date: Date when the user will be unbanned; Unix time. If user is banned for more than 366 days or less than 30 seconds from the current time they are considered to be banned forever. Applied for supergroups and channels only.
        :param revoke_messages: Pass :code:`True` to delete all messages from the chat for the user that is being removed. If :code:`False`, the user will be able to see messages in the group that were sent before the user was removed. Always :code:`True` for supergroups and channels.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = BanChatMember(
            chat_id=chat_id,
            user_id=user_id,
            until_date=until_date,
            revoke_messages=revoke_messages,
        )
        return await self(call, request_timeout=request_timeout)

    async def ban_chat_sender_chat(
        self,
        chat_id: ChatIdUnion,
        sender_chat_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to ban a channel chat in a supergroup or a channel. Until the chat is `unbanned <https://core.telegram.org/bots/api#unbanchatsenderchat>`_, the owner of the banned chat won't be able to send messages on behalf of **any of their channels**. The bot must be an administrator in the supergroup or channel for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#banchatsenderchat

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param sender_chat_id: Unique identifier of the target sender chat
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = BanChatSenderChat(
            chat_id=chat_id,
            sender_chat_id=sender_chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def close(
        self,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to close the bot instance before moving it from one local server to another. You need to delete the webhook before calling this method to ensure that the bot isn't launched again after server restart. The method will return error 429 in the first 10 minutes after the bot is launched. Returns :code:`True` on success. Requires no parameters.

        Source: https://core.telegram.org/bots/api#close

        :param request_timeout: Request timeout
        :return: Requires no parameters.
        """

        call = Close()
        return await self(call, request_timeout=request_timeout)

    async def close_forum_topic(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to close an open topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights, unless it is the creator of the topic. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#closeforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread of the forum topic
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = CloseForumTopic(
            chat_id=chat_id,
            message_thread_id=message_thread_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def copy_message(
        self,
        chat_id: ChatIdUnion,
        from_chat_id: ChatIdUnion,
        message_id: int,
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
        request_timeout: int | None = None,
    ) -> MessageId:
        """
        Use this method to copy messages of any kind. Service messages, paid media messages, giveaway messages, giveaway winners messages, and invoice messages can't be copied. A quiz :class:`aiogram.methods.poll.Poll` can be copied only if the value of the field *correct_option_id* is known to the bot. The method is analogous to the method :class:`aiogram.methods.forward_message.ForwardMessage`, but the copied message doesn't have a link to the original message. Returns the :class:`aiogram.types.message_id.MessageId` of the sent message on success.

        Source: https://core.telegram.org/bots/api#copymessage

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param from_chat_id: Unique identifier for the chat where the original message was sent (or username of the target bot, supergroup or channel in the format :code:`@username`)
        :param message_id: Message identifier in the chat specified in *from_chat_id*
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param video_start_timestamp: New start timestamp for the copied video in the message
        :param caption: New caption for media, 0-1024 characters after entities parsing. If not specified, the original caption is kept.
        :param parse_mode: Mode for parsing entities in the new caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the new caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media. Ignored if a new caption isn't specified.
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; only available when copying to private chats
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: Returns the :class:`aiogram.types.message_id.MessageId` of the sent message on success.
        """

        call = CopyMessage(
            chat_id=chat_id,
            from_chat_id=from_chat_id,
            message_id=message_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def create_chat_invite_link(
        self,
        chat_id: ChatIdUnion,
        name: str | None = None,
        expire_date: DateTimeUnion | None = None,
        member_limit: int | None = None,
        creates_join_request: bool | None = None,
        request_timeout: int | None = None,
    ) -> ChatInviteLink:
        """
        Use this method to create an additional invite link for a chat. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. The link can be revoked using the method :class:`aiogram.methods.revoke_chat_invite_link.RevokeChatInviteLink`. Returns the new invite link as :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

        Source: https://core.telegram.org/bots/api#createchatinvitelink

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param name: Invite link name; 0-32 characters
        :param expire_date: Point in time (Unix timestamp) when the link will expire
        :param member_limit: The maximum number of users that can be members of the chat simultaneously after joining the chat via this invite link; 1-99999
        :param creates_join_request: :code:`True`, if users joining the chat via the link need to be approved by chat administrators. If :code:`True`, *member_limit* can't be specified.
        :param request_timeout: Request timeout
        :return: Returns the new invite link as :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.
        """

        call = CreateChatInviteLink(
            chat_id=chat_id,
            name=name,
            expire_date=expire_date,
            member_limit=member_limit,
            creates_join_request=creates_join_request,
        )
        return await self(call, request_timeout=request_timeout)

    async def create_forum_topic(
        self,
        chat_id: ChatIdUnion,
        name: str,
        icon_color: int | None = None,
        icon_custom_emoji_id: str | None = None,
        request_timeout: int | None = None,
    ) -> ForumTopic:
        """
        Use this method to create a topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator right. Returns information about the created topic as a :class:`aiogram.types.forum_topic.ForumTopic` object.

        Source: https://core.telegram.org/bots/api#createforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param name: Topic name, 1-128 characters
        :param icon_color: Color of the topic icon in RGB format. Currently, must be one of 7322096 (0x6FB9F0), 16766590 (0xFFD67E), 13338331 (0xCB86DB), 9367192 (0x8EEE98), 16749490 (0xFF93B2), or 16478047 (0xFB6F5F).
        :param icon_custom_emoji_id: Unique identifier of the custom emoji shown as the topic icon. Use :class:`aiogram.methods.get_forum_topic_icon_stickers.GetForumTopicIconStickers` to get all allowed custom emoji identifiers.
        :param request_timeout: Request timeout
        :return: Returns information about the created topic as a :class:`aiogram.types.forum_topic.ForumTopic` object.
        """

        call = CreateForumTopic(
            chat_id=chat_id,
            name=name,
            icon_color=icon_color,
            icon_custom_emoji_id=icon_custom_emoji_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def create_invoice_link(
        self,
        title: str,
        description: str,
        payload: str,
        currency: str,
        prices: list[LabeledPrice],
        business_connection_id: str | None = None,
        provider_token: str | None = None,
        subscription_period: int | None = None,
        max_tip_amount: int | None = None,
        suggested_tip_amounts: list[int] | None = None,
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
        request_timeout: int | None = None,
    ) -> str:
        """
        Use this method to create a link for an invoice. Returns the created invoice link as *String* on success.

        Source: https://core.telegram.org/bots/api#createinvoicelink

        :param title: Product name, 1-32 characters
        :param description: Product description, 1-255 characters
        :param payload: Bot-defined invoice payload, 1-128 bytes. This will not be displayed to the user, use it for your internal processes.
        :param currency: Three-letter ISO 4217 currency code, see `more on currencies <https://core.telegram.org/bots/payments#supported-currencies>`_. Pass 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param prices: Price breakdown, a JSON-serialized list of components (e.g. product price, tax, discount, delivery cost, delivery tax, bonus, etc.). Must contain exactly one item for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param business_connection_id: Unique identifier of the business connection on behalf of which the link will be created. For payments in `Telegram Stars <https://t.me/BotNews/90>`_ only.
        :param provider_token: Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param subscription_period: The number of seconds the subscription will be active for before the next payment. The currency must be set to 'XTR' (Telegram Stars) if the parameter is used. Currently, it must always be 2592000 (30 days) if specified. Any number of subscriptions can be active for a given bot at the same time, including multiple concurrent subscriptions from the same user. Subscription price must no exceed 10000 Telegram Stars.
        :param max_tip_amount: The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param suggested_tip_amounts: A JSON-serialized array of suggested amounts of tips in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*.
        :param provider_data: JSON-serialized data about the invoice, which will be shared with the payment provider. A detailed description of required fields should be provided by the payment provider.
        :param photo_url: URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service.
        :param photo_size: Photo size in bytes
        :param photo_width: Photo width
        :param photo_height: Photo height
        :param need_name: Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param need_phone_number: Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param need_email: Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param need_shipping_address: Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param send_phone_number_to_provider: Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param send_email_to_provider: Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param is_flexible: Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param request_timeout: Request timeout
        :return: Returns the created invoice link as *String* on success.
        """

        call = CreateInvoiceLink(
            title=title,
            description=description,
            payload=payload,
            currency=currency,
            prices=prices,
            business_connection_id=business_connection_id,
            provider_token=provider_token,
            subscription_period=subscription_period,
            max_tip_amount=max_tip_amount,
            suggested_tip_amounts=suggested_tip_amounts,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def create_new_sticker_set(
        self,
        user_id: int,
        name: str,
        title: str,
        stickers: list[InputSticker],
        sticker_type: str | None = None,
        needs_repainting: bool | None = None,
        sticker_format: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to create a new sticker set owned by a user. The bot will be able to edit the sticker set thus created. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#createnewstickerset

        :param user_id: User identifier of created sticker set owner
        :param name: Short name of sticker set, to be used in :code:`t.me/addstickers/` URLs (e.g., *animals*). Can contain only English letters, digits and underscores. Must begin with a letter, can't contain consecutive underscores and must end in :code:`"_by_<bot_username>"`. :code:`<bot_username>` is case insensitive. 1-64 characters.
        :param title: Sticker set title, 1-64 characters
        :param stickers: A JSON-serialized list of 1-50 initial stickers to be added to the sticker set
        :param sticker_type: Type of stickers in the set, pass 'regular', 'mask', or 'custom_emoji'. By default, a regular sticker set is created.
        :param needs_repainting: Pass :code:`True` if stickers in the sticker set must be repainted to the color of text when used in messages, the accent color if used as emoji status, white on chat photos, or another appropriate color based on context; for custom emoji sticker sets only
        :param sticker_format: Format of stickers in the set, must be one of 'static', 'animated', 'video'
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = CreateNewStickerSet(
            user_id=user_id,
            name=name,
            title=title,
            stickers=stickers,
            sticker_type=sticker_type,
            needs_repainting=needs_repainting,
            sticker_format=sticker_format,
        )
        return await self(call, request_timeout=request_timeout)

    async def decline_chat_join_request(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to decline a chat join request. The bot must be an administrator in the chat for this to work and must have the *can_invite_users* administrator right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#declinechatjoinrequest

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeclineChatJoinRequest(
            chat_id=chat_id,
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_chat_photo(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete a chat photo. Photos can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletechatphoto

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteChatPhoto(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_chat_sticker_set(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete a group sticker set from a supergroup. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Use the field *can_set_sticker_set* optionally returned in :class:`aiogram.methods.get_chat.GetChat` requests to check if the bot can use this method. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletechatstickerset

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteChatStickerSet(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_forum_topic(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete a forum topic along with all its messages in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_delete_messages* administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deleteforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread of the forum topic
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteForumTopic(
            chat_id=chat_id,
            message_thread_id=message_thread_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_message(
        self,
        chat_id: ChatIdUnion,
        message_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
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

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param message_id: Identifier of the message to delete
        :param request_timeout: Request timeout
        :return: Use this method to delete a message, including service messages, with the following limitations:
        """

        call = DeleteMessage(
            chat_id=chat_id,
            message_id=message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_my_commands(
        self,
        scope: BotCommandScopeUnion | None = None,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete the list of the bot's commands for the given scope and user language. After deletion, `higher level commands <https://core.telegram.org/bots/api#determining-list-of-commands>`_ will be shown to affected users. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletemycommands

        :param scope: A JSON-serialized object, describing scope of users for which the commands are relevant. Defaults to :class:`aiogram.types.bot_command_scope_default.BotCommandScopeDefault`.
        :param language_code: A two-letter ISO 639-1 language code. If empty, commands will be applied to all users from the given scope, for whose language there are no dedicated commands.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteMyCommands(
            scope=scope,
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_sticker_from_set(
        self,
        sticker: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete a sticker from a set created by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletestickerfromset

        :param sticker: File identifier of the sticker
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteStickerFromSet(
            sticker=sticker,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_webhook(
        self,
        drop_pending_updates: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to remove webhook integration if you decide to switch back to :class:`aiogram.methods.get_updates.GetUpdates`. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletewebhook

        :param drop_pending_updates: Pass :code:`True` to drop all pending updates
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteWebhook(
            drop_pending_updates=drop_pending_updates,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_chat_invite_link(
        self,
        chat_id: ChatIdUnion,
        invite_link: str,
        name: str | None = None,
        expire_date: DateTimeUnion | None = None,
        member_limit: int | None = None,
        creates_join_request: bool | None = None,
        request_timeout: int | None = None,
    ) -> ChatInviteLink:
        """
        Use this method to edit a non-primary invite link created by the bot. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns the edited invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

        Source: https://core.telegram.org/bots/api#editchatinvitelink

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param invite_link: The invite link to edit
        :param name: Invite link name; 0-32 characters
        :param expire_date: Point in time (Unix timestamp) when the link will expire
        :param member_limit: The maximum number of users that can be members of the chat simultaneously after joining the chat via this invite link; 1-99999
        :param creates_join_request: :code:`True`, if users joining the chat via the link need to be approved by chat administrators. If :code:`True`, *member_limit* can't be specified.
        :param request_timeout: Request timeout
        :return: Returns the edited invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.
        """

        call = EditChatInviteLink(
            chat_id=chat_id,
            invite_link=invite_link,
            name=name,
            expire_date=expire_date,
            member_limit=member_limit,
            creates_join_request=creates_join_request,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_forum_topic(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int,
        name: str | None = None,
        icon_custom_emoji_id: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to edit name and icon of a topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights, unless it is the creator of the topic. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#editforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread of the forum topic
        :param name: New topic name, 0-128 characters. If not specified or empty, the current name of the topic will be kept.
        :param icon_custom_emoji_id: New unique identifier of the custom emoji shown as the topic icon. Use :class:`aiogram.methods.get_forum_topic_icon_stickers.GetForumTopicIconStickers` to get all allowed custom emoji identifiers. Pass an empty string to remove the icon. If not specified, the current icon will be kept.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = EditForumTopic(
            chat_id=chat_id,
            message_thread_id=message_thread_id,
            name=name,
            icon_custom_emoji_id=icon_custom_emoji_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_message_caption(
        self,
        business_connection_id: str | None = None,
        chat_id: ChatIdUnion | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        caption: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | Default | None = Default("show_caption_above_media"),
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to edit captions of messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagecaption

        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the message to edit.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param caption: New caption of the message, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the message caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media. Supported only for animation, photo and video messages.
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.
        """

        call = EditMessageCaption(
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            show_caption_above_media=show_caption_above_media,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_message_live_location(
        self,
        latitude: float,
        longitude: float,
        business_connection_id: str | None = None,
        chat_id: ChatIdUnion | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        live_period: int | None = None,
        horizontal_accuracy: float | None = None,
        heading: int | None = None,
        proximity_alert_radius: int | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to edit live location messages. A location can be edited until its *live_period* expires or editing is explicitly disabled by a call to :class:`aiogram.methods.stop_message_live_location.StopMessageLiveLocation`. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.

        Source: https://core.telegram.org/bots/api#editmessagelivelocation

        :param latitude: Latitude of new location
        :param longitude: Longitude of new location
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the message to edit.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param live_period: New period in seconds during which the location can be updated, starting from the message send date. If 0x7FFFFFFF is specified, then the location can be updated forever. Otherwise, the new value must not exceed the current *live_period* by more than a day, and the live location expiration date must remain within the next 90 days. If not specified, then *live_period* remains unchanged.
        :param horizontal_accuracy: The radius of uncertainty for the location, measured in meters; 0-1500
        :param heading: Direction in which the user is moving, in degrees. Must be between 1 and 360 if specified.
        :param proximity_alert_radius: The maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified.
        :param reply_markup: A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.
        """

        call = EditMessageLiveLocation(
            latitude=latitude,
            longitude=longitude,
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
            live_period=live_period,
            horizontal_accuracy=horizontal_accuracy,
            heading=heading,
            proximity_alert_radius=proximity_alert_radius,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_message_media(
        self,
        media: InputMediaUnion,
        business_connection_id: str | None = None,
        chat_id: ChatIdUnion | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to edit animation, audio, document, live photo, photo, or video messages, or to replace a text or a rich message with a media. If a message is part of a message album, then it can be edited only to an audio for audio albums, only to a document for document albums and to a photo, a live photo, or a video otherwise. When an inline message is edited, a new file can't be uploaded; use a previously uploaded file via its file_id or specify a URL. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagemedia

        :param media: A JSON-serialized object for a new media content of the message
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the message to edit.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param reply_markup: A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.
        """

        call = EditMessageMedia(
            media=media,
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_message_reply_markup(
        self,
        business_connection_id: str | None = None,
        chat_id: ChatIdUnion | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to edit only the reply markup of messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagereplymarkup

        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the message to edit.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.
        """

        call = EditMessageReplyMarkup(
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_message_text(
        self,
        text: str | None = None,
        business_connection_id: str | None = None,
        chat_id: ChatIdUnion | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        parse_mode: str | Default | None = Default("parse_mode"),
        entities: list[MessageEntity] | None = None,
        link_preview_options: LinkPreviewOptions | Default | None = Default("link_preview"),
        reply_markup: InlineKeyboardMarkup | None = None,
        rich_message: InputRichMessage | None = None,
        disable_web_page_preview: bool | Default | None = Default("link_preview_is_disabled"),
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to edit text, rich and `game <https://core.telegram.org/bots/api#games>`_ messages. On success, if the edited message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.

        Source: https://core.telegram.org/bots/api#editmessagetext

        :param text: New text of the message, 1-4096 characters after entity parsing; required if *rich_message* isn't specified
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the message to edit.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param link_preview_options: Link preview generation options for the message
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param rich_message: New rich content of the message; required if *text* isn't specified
        :param disable_web_page_preview: Disables link previews for links in this message
        :param request_timeout: Request timeout
        :return: Note that business messages that were not sent by the bot and do not contain an inline keyboard can only be edited within **48 hours** from the time they were sent.
        """

        call = EditMessageText(
            text=text,
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
            parse_mode=parse_mode,
            entities=entities,
            link_preview_options=link_preview_options,
            reply_markup=reply_markup,
            rich_message=rich_message,
            disable_web_page_preview=disable_web_page_preview,
        )
        return await self(call, request_timeout=request_timeout)

    async def export_chat_invite_link(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> str:
        """
        Use this method to generate a new primary invite link for a chat; any previously generated primary link is revoked. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns the new invite link as *String* on success.

         Note: Each administrator in a chat generates their own invite links. Bots can't use invite links generated by other administrators. If you want your bot to work with invite links, it will need to generate its own link using :class:`aiogram.methods.export_chat_invite_link.ExportChatInviteLink` or by calling the :class:`aiogram.methods.get_chat.GetChat` method. If your bot needs to generate a new primary invite link replacing its previous one, use :class:`aiogram.methods.export_chat_invite_link.ExportChatInviteLink` again.

        Source: https://core.telegram.org/bots/api#exportchatinvitelink

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: If your bot needs to generate a new primary invite link replacing its previous one, use :class:`aiogram.methods.export_chat_invite_link.ExportChatInviteLink` again.
        """

        call = ExportChatInviteLink(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def forward_message(
        self,
        chat_id: ChatIdUnion,
        from_chat_id: ChatIdUnion,
        message_id: int,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        video_start_timestamp: DateTimeUnion | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to forward messages of any kind. Service messages and messages with protected content can't be forwarded. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#forwardmessage

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param from_chat_id: Unique identifier for the chat where the original message was sent (or username of the target bot, supergroup or channel in the format :code:`@username`)
        :param message_id: Message identifier in the chat specified in *from_chat_id*
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be forwarded; required if the message is forwarded to a direct messages chat
        :param video_start_timestamp: New start timestamp for the forwarded video in the message
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the forwarded message from forwarding and saving
        :param message_effect_id: Unique identifier of the message effect to be added to the message; only available when forwarding to private chats
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = ForwardMessage(
            chat_id=chat_id,
            from_chat_id=from_chat_id,
            message_id=message_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            video_start_timestamp=video_start_timestamp,
            disable_notification=disable_notification,
            protect_content=protect_content,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_chat(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> ChatFullInfo:
        """
        Use this method to get up-to-date information about the chat. Returns a :class:`aiogram.types.chat_full_info.ChatFullInfo` object on success.

        Source: https://core.telegram.org/bots/api#getchat

        :param chat_id: Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.chat_full_info.ChatFullInfo` object on success.
        """

        call = GetChat(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_chat_administrators(
        self,
        chat_id: ChatIdUnion,
        return_bots: bool | None = None,
        request_timeout: int | None = None,
    ) -> list[ResultChatMemberUnion]:
        """
        Use this method to get a list of administrators in a chat. Returns an Array of :class:`aiogram.types.chat_member.ChatMember` objects.

        Source: https://core.telegram.org/bots/api#getchatadministrators

        :param chat_id: Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`
        :param return_bots: Pass :code:`True` to additionally receive all bots that are administrators of the chat. By default, bots other than the current bot are omitted.
        :param request_timeout: Request timeout
        :return: Returns an Array of :class:`aiogram.types.chat_member.ChatMember` objects.
        """

        call = GetChatAdministrators(
            chat_id=chat_id,
            return_bots=return_bots,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_chat_member(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        request_timeout: int | None = None,
    ) -> ResultChatMemberUnion:
        """
        Use this method to get information about a member of a chat. The method is only guaranteed to work for other users if the bot is an administrator in the chat. Returns a :class:`aiogram.types.chat_member.ChatMember` object on success.

        Source: https://core.telegram.org/bots/api#getchatmember

        :param chat_id: Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.chat_member.ChatMember` object on success.
        """

        call = GetChatMember(
            chat_id=chat_id,
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_chat_member_count(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> int:
        """
        Use this method to get the number of members in a chat. Returns *Int* on success.

        Source: https://core.telegram.org/bots/api#getchatmembercount

        :param chat_id: Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns *Int* on success.
        """

        call = GetChatMemberCount(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_chat_menu_button(
        self,
        chat_id: int | None = None,
        request_timeout: int | None = None,
    ) -> ResultMenuButtonUnion:
        """
        Use this method to get the current value of the bot's menu button in a private chat, or the default menu button. Returns :class:`aiogram.types.menu_button.MenuButton` on success.

        Source: https://core.telegram.org/bots/api#getchatmenubutton

        :param chat_id: Unique identifier for the target private chat. If not specified, the bot's default menu button will be returned.
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.menu_button.MenuButton` on success.
        """

        call = GetChatMenuButton(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_custom_emoji_stickers(
        self,
        custom_emoji_ids: list[str],
        request_timeout: int | None = None,
    ) -> list[Sticker]:
        """
        Use this method to get information about custom emoji stickers by their identifiers. Returns an Array of :class:`aiogram.types.sticker.Sticker` objects.

        Source: https://core.telegram.org/bots/api#getcustomemojistickers

        :param custom_emoji_ids: A JSON-serialized list of custom emoji identifiers. At most 200 custom emoji identifiers can be specified.
        :param request_timeout: Request timeout
        :return: Returns an Array of :class:`aiogram.types.sticker.Sticker` objects.
        """

        call = GetCustomEmojiStickers(
            custom_emoji_ids=custom_emoji_ids,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_file(
        self,
        file_id: str,
        request_timeout: int | None = None,
    ) -> File:
        """
        Use this method to get basic information about a file and prepare it for downloading. For the moment, bots can download files of up to 20MB in size. On success, a :class:`aiogram.types.file.File` object is returned. The file can then be downloaded via the link :code:`https://api.telegram.org/file/bot<token>/<file_path>`, where :code:`<file_path>` is taken from the response. It is guaranteed that the link will be valid for at least 1 hour. When the link expires, a new one can be requested by calling :class:`aiogram.methods.get_file.GetFile` again.
        **Note:** This function may not preserve the original file name and MIME type. You should save the file's MIME type and name (if available) when the File object is received.

        Source: https://core.telegram.org/bots/api#getfile

        :param file_id: File identifier to get information about
        :param request_timeout: Request timeout
        :return: You should save the file's MIME type and name (if available) when the File object is received.
        """

        call = GetFile(
            file_id=file_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_forum_topic_icon_stickers(
        self,
        request_timeout: int | None = None,
    ) -> list[Sticker]:
        """
        Use this method to get custom emoji stickers, which can be used as a forum topic icon by any user. Requires no parameters. Returns an Array of :class:`aiogram.types.sticker.Sticker` objects.

        Source: https://core.telegram.org/bots/api#getforumtopiciconstickers

        :param request_timeout: Request timeout
        :return: Returns an Array of :class:`aiogram.types.sticker.Sticker` objects.
        """

        call = GetForumTopicIconStickers()
        return await self(call, request_timeout=request_timeout)

    async def get_game_high_scores(
        self,
        user_id: int,
        chat_id: int | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        request_timeout: int | None = None,
    ) -> list[GameHighScore]:
        """
        Use this method to get data for high score tables. Will return the score of the specified user and several of their neighbors in a game. Returns an Array of :class:`aiogram.types.game_high_score.GameHighScore` objects.

         This method will currently return scores for the target user, plus two of their closest neighbors on each side. Will also return the top three users if the user and their neighbors are not among them. Please note that this behavior is subject to change.

        Source: https://core.telegram.org/bots/api#getgamehighscores

        :param user_id: Target user id
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the sent message.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param request_timeout: Request timeout
        :return: Please note that this behavior is subject to change.
        """

        call = GetGameHighScores(
            user_id=user_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_me(
        self,
        request_timeout: int | None = None,
    ) -> User:
        """
        A simple method for testing your bot's authentication token. Requires no parameters. Returns basic information about the bot in form of a :class:`aiogram.types.user.User` object.

        Source: https://core.telegram.org/bots/api#getme

        :param request_timeout: Request timeout
        :return: Returns basic information about the bot in form of a :class:`aiogram.types.user.User` object.
        """

        call = GetMe()
        return await self(call, request_timeout=request_timeout)

    async def get_my_commands(
        self,
        scope: BotCommandScopeUnion | None = None,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> list[BotCommand]:
        """
        Use this method to get the current list of the bot's commands for the given scope and user language. Returns an Array of :class:`aiogram.types.bot_command.BotCommand` objects. If commands aren't set, an empty list is returned.

        Source: https://core.telegram.org/bots/api#getmycommands

        :param scope: A JSON-serialized object, describing scope of users. Defaults to :class:`aiogram.types.bot_command_scope_default.BotCommandScopeDefault`.
        :param language_code: A two-letter ISO 639-1 language code or an empty string
        :param request_timeout: Request timeout
        :return: If commands aren't set, an empty list is returned.
        """

        call = GetMyCommands(
            scope=scope,
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_my_default_administrator_rights(
        self,
        for_channels: bool | None = None,
        request_timeout: int | None = None,
    ) -> ChatAdministratorRights:
        """
        Use this method to get the current default administrator rights of the bot. Returns :class:`aiogram.types.chat_administrator_rights.ChatAdministratorRights` on success.

        Source: https://core.telegram.org/bots/api#getmydefaultadministratorrights

        :param for_channels: Pass :code:`True` to get default administrator rights of the bot in channels. Otherwise, default administrator rights of the bot for groups and supergroups will be returned.
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.chat_administrator_rights.ChatAdministratorRights` on success.
        """

        call = GetMyDefaultAdministratorRights(
            for_channels=for_channels,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_sticker_set(
        self,
        name: str,
        request_timeout: int | None = None,
    ) -> StickerSet:
        """
        Use this method to get a sticker set. On success, a :class:`aiogram.types.sticker_set.StickerSet` object is returned.

        Source: https://core.telegram.org/bots/api#getstickerset

        :param name: Name of the sticker set
        :param request_timeout: Request timeout
        :return: On success, a :class:`aiogram.types.sticker_set.StickerSet` object is returned.
        """

        call = GetStickerSet(
            name=name,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_updates(
        self,
        offset: int | None = None,
        limit: int | None = None,
        timeout: int | None = None,
        allowed_updates: list[str] | None = None,
        request_timeout: int | None = None,
    ) -> list[Update]:
        """
        Use this method to receive incoming updates using long polling (`wiki <https://en.wikipedia.org/wiki/Push_technology#Long_polling>`_). Returns an Array of :class:`aiogram.types.update.Update` objects.

         **Notes**

         **1.** This method will not work if an outgoing webhook is set up.

         **2.** In order to avoid getting duplicate updates, recalculate *offset* after each server response.

        Source: https://core.telegram.org/bots/api#getupdates

        :param offset: Identifier of the first update to be returned. Must be greater by one than the highest among the identifiers of previously received updates. By default, updates starting with the earliest unconfirmed update are returned. An update is considered confirmed as soon as :class:`aiogram.methods.get_updates.GetUpdates` is called with an *offset* higher than its *update_id*. The negative offset can be specified to retrieve updates starting from *-offset* update from the end of the updates queue. All previous updates will be forgotten.
        :param limit: Limits the number of updates to be retrieved. Values between 1-100 are accepted. Defaults to 100.
        :param timeout: Timeout in seconds for long polling. Defaults to 0, i.e. usual short polling. Should be positive, short polling should be used for testing purposes only.
        :param allowed_updates: A JSON-serialized list of the update types you want your bot to receive. For example, specify :code:`["message", "edited_channel_post", "callback_query"]` to only receive updates of these types. See :class:`aiogram.types.update.Update` for a complete list of available update types. Specify an empty list to receive all update types except *chat_member*, *message_reaction*, and *message_reaction_count* (default). If not specified, the previous setting will be used.
        :param request_timeout: Request timeout
        :return: Returns an Array of :class:`aiogram.types.update.Update` objects.
        """

        call = GetUpdates(
            offset=offset,
            limit=limit,
            timeout=timeout,
            allowed_updates=allowed_updates,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_user_profile_photos(
        self,
        user_id: int,
        offset: int | None = None,
        limit: int | None = None,
        request_timeout: int | None = None,
    ) -> UserProfilePhotos:
        """
        Use this method to get a list of profile pictures for a user. Returns a :class:`aiogram.types.user_profile_photos.UserProfilePhotos` object.

        Source: https://core.telegram.org/bots/api#getuserprofilephotos

        :param user_id: Unique identifier of the target user
        :param offset: Sequential number of the first photo to be returned. By default, all photos are returned.
        :param limit: Limits the number of photos to be retrieved. Values between 1-100 are accepted. Defaults to 100.
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.user_profile_photos.UserProfilePhotos` object.
        """

        call = GetUserProfilePhotos(
            user_id=user_id,
            offset=offset,
            limit=limit,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_webhook_info(
        self,
        request_timeout: int | None = None,
    ) -> WebhookInfo:
        """
        Use this method to get current webhook status. Requires no parameters. On success, returns a :class:`aiogram.types.webhook_info.WebhookInfo` object. If the bot is using :class:`aiogram.methods.get_updates.GetUpdates`, will return an object with the *url* field empty.

        Source: https://core.telegram.org/bots/api#getwebhookinfo

        :param request_timeout: Request timeout
        :return: If the bot is using :class:`aiogram.methods.get_updates.GetUpdates`, will return an object with the *url* field empty.
        """

        call = GetWebhookInfo()
        return await self(call, request_timeout=request_timeout)

    async def leave_chat(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method for your bot to leave a group, supergroup or channel. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#leavechat

        :param chat_id: Unique identifier for the target chat or username of the target supergroup or channel in the format :code:`@username`. Channel direct messages chats aren't supported; leave the corresponding channel instead.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = LeaveChat(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def log_out(
        self,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to log out from the cloud Bot API server before launching the bot locally. You **must** log out the bot before running it locally, otherwise there is no guarantee that the bot will receive updates. After a successful call, you can immediately log in on a local server, but will not be able to log in back to the cloud Bot API server for 10 minutes. Returns :code:`True` on success. Requires no parameters.

        Source: https://core.telegram.org/bots/api#logout

        :param request_timeout: Request timeout
        :return: Requires no parameters.
        """

        call = LogOut()
        return await self(call, request_timeout=request_timeout)

    async def pin_chat_message(
        self,
        chat_id: ChatIdUnion,
        message_id: int,
        business_connection_id: str | None = None,
        disable_notification: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to add a message to the list of pinned messages in a chat. In private chats and channel direct messages chats, all non-service messages can be pinned. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to pin messages in groups and channels respectively. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#pinchatmessage

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param message_id: Identifier of a message to pin
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be pinned
        :param disable_notification: Pass :code:`True` if it is not necessary to send a notification to all chat members about the new pinned message. Notifications are always disabled in channels and private chats.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = PinChatMessage(
            chat_id=chat_id,
            message_id=message_id,
            business_connection_id=business_connection_id,
            disable_notification=disable_notification,
        )
        return await self(call, request_timeout=request_timeout)

    async def promote_chat_member(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        is_anonymous: bool | None = None,
        can_manage_chat: bool | None = None,
        can_delete_messages: bool | None = None,
        can_manage_video_chats: bool | None = None,
        can_restrict_members: bool | None = None,
        can_promote_members: bool | None = None,
        can_change_info: bool | None = None,
        can_invite_users: bool | None = None,
        can_post_stories: bool | None = None,
        can_edit_stories: bool | None = None,
        can_delete_stories: bool | None = None,
        can_post_messages: bool | None = None,
        can_edit_messages: bool | None = None,
        can_pin_messages: bool | None = None,
        can_manage_topics: bool | None = None,
        can_manage_direct_messages: bool | None = None,
        can_manage_tags: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to promote or demote a user in a supergroup or a channel. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Pass :code:`False` for all boolean parameters to demote a user. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#promotechatmember

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param is_anonymous: Pass :code:`True` if the administrator's presence in the chat is hidden
        :param can_manage_chat: Pass :code:`True` if the administrator can access the chat event log, get boost list, see hidden supergroup and channel members, report spam messages, ignore slow mode, and send messages to the chat without paying Telegram Stars. Implied by any other administrator privilege.
        :param can_delete_messages: Pass :code:`True` if the administrator can delete messages of other users
        :param can_manage_video_chats: Pass :code:`True` if the administrator can manage video chats
        :param can_restrict_members: Pass :code:`True` if the administrator can restrict, ban or unban chat members, or access supergroup statistics. For backward compatibility, defaults to :code:`True` for promotions of channel administrators.
        :param can_promote_members: Pass :code:`True` if the administrator can add new administrators with a subset of their own privileges or demote administrators that they have promoted, directly or indirectly (promoted by administrators that were appointed by him)
        :param can_change_info: Pass :code:`True` if the administrator can change chat title, photo and other settings
        :param can_invite_users: Pass :code:`True` if the administrator can invite new users to the chat
        :param can_post_stories: Pass :code:`True` if the administrator can post stories to the chat
        :param can_edit_stories: Pass :code:`True` if the administrator can edit stories posted by other users, post stories to the chat page, pin chat stories, and access the chat's story archive
        :param can_delete_stories: Pass :code:`True` if the administrator can delete stories posted by other users
        :param can_post_messages: Pass :code:`True` if the administrator can post messages in the channel, approve suggested posts, or access channel statistics; for channels only
        :param can_edit_messages: Pass :code:`True` if the administrator can edit messages of other users and can pin messages; for channels only
        :param can_pin_messages: Pass :code:`True` if the administrator can pin messages; for supergroups only
        :param can_manage_topics: Pass :code:`True` if the user is allowed to create, rename, close, and reopen forum topics; for supergroups only
        :param can_manage_direct_messages: Pass :code:`True` if the administrator can manage direct messages within the channel and decline suggested posts; for channels only
        :param can_manage_tags: Pass :code:`True` if the administrator can edit the tags of regular members; for groups and supergroups only
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = PromoteChatMember(
            chat_id=chat_id,
            user_id=user_id,
            is_anonymous=is_anonymous,
            can_manage_chat=can_manage_chat,
            can_delete_messages=can_delete_messages,
            can_manage_video_chats=can_manage_video_chats,
            can_restrict_members=can_restrict_members,
            can_promote_members=can_promote_members,
            can_change_info=can_change_info,
            can_invite_users=can_invite_users,
            can_post_stories=can_post_stories,
            can_edit_stories=can_edit_stories,
            can_delete_stories=can_delete_stories,
            can_post_messages=can_post_messages,
            can_edit_messages=can_edit_messages,
            can_pin_messages=can_pin_messages,
            can_manage_topics=can_manage_topics,
            can_manage_direct_messages=can_manage_direct_messages,
            can_manage_tags=can_manage_tags,
        )
        return await self(call, request_timeout=request_timeout)

    async def reopen_forum_topic(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to reopen a closed topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights, unless it is the creator of the topic. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#reopenforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread of the forum topic
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ReopenForumTopic(
            chat_id=chat_id,
            message_thread_id=message_thread_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def restrict_chat_member(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        permissions: ChatPermissions,
        use_independent_chat_permissions: bool | None = None,
        until_date: DateTimeUnion | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to restrict a user in a supergroup. The bot must be an administrator in the supergroup for this to work and must have the appropriate administrator rights. Pass :code:`True` for all permissions to lift restrictions from a user. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#restrictchatmember

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param permissions: A JSON-serialized object for new user permissions
        :param use_independent_chat_permissions: Pass :code:`True` if chat permissions are set independently. Otherwise, the *can_send_other_messages* and *can_add_web_page_previews* permissions will imply the *can_send_messages*, *can_send_audios*, *can_send_documents*, *can_send_photos*, *can_send_videos*, *can_send_video_notes*, and *can_send_voice_notes* permissions; the *can_send_polls* permission will imply the *can_send_messages* permission.
        :param until_date: Date when restrictions will be lifted for the user; Unix time. If user is restricted for more than 366 days or less than 30 seconds from the current time, they are considered to be restricted forever.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = RestrictChatMember(
            chat_id=chat_id,
            user_id=user_id,
            permissions=permissions,
            use_independent_chat_permissions=use_independent_chat_permissions,
            until_date=until_date,
        )
        return await self(call, request_timeout=request_timeout)

    async def revoke_chat_invite_link(
        self,
        chat_id: ChatIdUnion,
        invite_link: str,
        request_timeout: int | None = None,
    ) -> ChatInviteLink:
        """
        Use this method to revoke an invite link created by the bot. If the primary link is revoked, a new link is automatically generated. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns the revoked invite link as :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

        Source: https://core.telegram.org/bots/api#revokechatinvitelink

        :param chat_id: Unique identifier of the target chat or username of the target channel in the format :code:`@username`
        :param invite_link: The invite link to revoke
        :param request_timeout: Request timeout
        :return: Returns the revoked invite link as :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.
        """

        call = RevokeChatInviteLink(
            chat_id=chat_id,
            invite_link=invite_link,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_animation(
        self,
        chat_id: ChatIdUnion,
        animation: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send animation files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendanimation

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param animation: Animation to send. Pass a file_id as String to send an animation that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an animation from the Internet, or upload a new animation using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent animation in seconds
        :param width: Animation width
        :param height: Animation height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Animation caption (may also be used when resending animation by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the animation caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the animation needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: Bots can currently send animation files of up to 50 MB in size, this limit may be changed in the future.
        """

        call = SendAnimation(
            chat_id=chat_id,
            animation=animation,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_audio(
        self,
        chat_id: ChatIdUnion,
        audio: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio must be in the .MP3 or .M4A format. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send audio files of up to 50 MB in size, this limit may be changed in the future.
        For sending voice messages, use the :class:`aiogram.methods.send_voice.SendVoice` method instead.

        Source: https://core.telegram.org/bots/api#sendaudio

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param audio: Audio file to send. Pass a file_id as String to send an audio file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get an audio file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Audio caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the audio caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the audio in seconds
        :param performer: Performer
        :param title: Track name
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: Bots can currently send audio files of up to 50 MB in size, this limit may be changed in the future.
        """

        call = SendAudio(
            chat_id=chat_id,
            audio=audio,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_chat_action(
        self,
        chat_id: ChatIdUnion,
        action: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method when you need to tell the user that something is happening on the bot's side. The status is set for 5 seconds or less (when a message arrives from your bot, Telegram clients clear its typing status). Returns :code:`True` on success.

         Example: The `ImageBot <https://t.me/imagebot>`_ needs some time to process a request and upload the image. Instead of sending a text message along the lines of 'Retrieving image, please wait…', the bot may use :class:`aiogram.methods.send_chat_action.SendChatAction` with *action* = *upload_photo*. The user will see a 'sending photo' status for the bot.

        We only recommend using this method when a response from the bot will take a **noticeable** amount of time to arrive.

        Source: https://core.telegram.org/bots/api#sendchataction

        :param chat_id: Unique identifier for the target chat or username of the target bot or supergroup in the format :code:`@username`. Channel chats and channel direct messages chats aren't supported.
        :param action: Type of action to broadcast. Choose one, depending on what the user is about to receive: *typing* for `text messages <https://core.telegram.org/bots/api#sendmessage>`_, *upload_photo* for `photos <https://core.telegram.org/bots/api#sendphoto>`_, *record_video* or *upload_video* for `videos <https://core.telegram.org/bots/api#sendvideo>`_, *record_voice* or *upload_voice* for `voice notes <https://core.telegram.org/bots/api#sendvoice>`_, *upload_document* for `general files <https://core.telegram.org/bots/api#senddocument>`_, *choose_sticker* for `stickers <https://core.telegram.org/bots/api#sendsticker>`_, *find_location* for `location data <https://core.telegram.org/bots/api#sendlocation>`_, *record_video_note* or *upload_video_note* for `video notes <https://core.telegram.org/bots/api#sendvideonote>`_.
        :param business_connection_id: Unique identifier of the business connection on behalf of which the action will be sent
        :param message_thread_id: Unique identifier for the target message thread or topic of a forum; for supergroups and private chats of bots with forum topic mode enabled only
        :param request_timeout: Request timeout
        :return: The user will see a 'sending photo' status for the bot.
        """

        call = SendChatAction(
            chat_id=chat_id,
            action=action,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_contact(
        self,
        chat_id: ChatIdUnion,
        phone_number: str,
        first_name: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send phone contacts. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendcontact

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param phone_number: Contact's phone number
        :param first_name: Contact's first name
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param last_name: Contact's last name
        :param vcard: Additional data about the contact in the form of a `vCard <https://en.wikipedia.org/wiki/VCard>`_, 0-2048 bytes
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendContact(
            chat_id=chat_id,
            phone_number=phone_number,
            first_name=first_name,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_dice(
        self,
        chat_id: ChatIdUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send an animated emoji that will display a random value. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#senddice

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji on which the dice throw animation is based. Currently, must be one of '🎲', '🎯', '🏀', '⚽', '🎳', or '🎰'. Dice can have values 1-6 for '🎲', '🎯' and '🎳', values 1-5 for '🏀' and '⚽', and values 1-64 for '🎰'. Defaults to '🎲'.
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendDice(
            chat_id=chat_id,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_document(
        self,
        chat_id: ChatIdUnion,
        document: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send general files. On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send files of any type of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#senddocument

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param document: File to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param caption: Document caption (may also be used when resending documents by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the document caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param disable_content_type_detection: Disables automatic server-side content type detection for files uploaded using multipart/form-data
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: Bots can currently send files of any type of up to 50 MB in size, this limit may be changed in the future.
        """

        call = SendDocument(
            chat_id=chat_id,
            document=document,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_game(
        self,
        chat_id: ChatIdUnion,
        game_short_name: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send a game. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendgame

        :param chat_id: Unique identifier for the target chat or username of the target bot in the format :code:`@username`. Games can't be sent to channel direct messages chats and channel chats.
        :param game_short_name: Short name of the game, serves as the unique identifier for the game. Set up your games via `@BotFather <https://t.me/botfather>`_.
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Play game_title' button will be shown. If not empty, the first button must launch the game.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendGame(
            chat_id=chat_id,
            game_short_name=game_short_name,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_invoice(
        self,
        chat_id: ChatIdUnion,
        title: str,
        description: str,
        payload: str,
        currency: str,
        prices: list[LabeledPrice],
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send invoices. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendinvoice

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param title: Product name, 1-32 characters
        :param description: Product description, 1-255 characters
        :param payload: Bot-defined invoice payload, 1-128 bytes. This will not be displayed to the user, use it for your internal processes.
        :param currency: Three-letter ISO 4217 currency code, see `more on currencies <https://core.telegram.org/bots/payments#supported-currencies>`_. Pass 'XTR' for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param prices: Price breakdown, a JSON-serialized list of components (e.g. product price, tax, discount, delivery cost, delivery tax, bonus, etc.). Must contain exactly one item for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param provider_token: Payment provider token, obtained via `@BotFather <https://t.me/botfather>`_. Pass an empty string for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param max_tip_amount: The maximum accepted amount for tips in the *smallest units* of the currency (integer, **not** float/double). For example, for a maximum tip of :code:`US$ 1.45` pass :code:`max_tip_amount = 145`. See the *exp* parameter in `currencies.json <https://core.telegram.org/bots/payments/currencies.json>`_, it shows the number of digits past the decimal point for each currency (2 for the majority of currencies). Defaults to 0. Not supported for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param suggested_tip_amounts: A JSON-serialized array of suggested amounts of tips in the *smallest units* of the currency (integer, **not** float/double). At most 4 suggested tip amounts can be specified. The suggested tip amounts must be positive, passed in a strictly increased order and must not exceed *max_tip_amount*.
        :param start_parameter: Unique deep-linking parameter. If left empty, **forwarded copies** of the sent message will have a *Pay* button, allowing multiple users to pay directly from the forwarded message, using the same invoice. If non-empty, forwarded copies of the sent message will have a *URL* button with a deep link to the bot (instead of a *Pay* button), with the value used as the start parameter.
        :param provider_data: JSON-serialized data about the invoice, which will be shared with the payment provider. A detailed description of required fields should be provided by the payment provider.
        :param photo_url: URL of the product photo for the invoice. Can be a photo of the goods or a marketing image for a service. People like it better when they see what they are paying for.
        :param photo_size: Photo size in bytes
        :param photo_width: Photo width
        :param photo_height: Photo height
        :param need_name: Pass :code:`True` if you require the user's full name to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param need_phone_number: Pass :code:`True` if you require the user's phone number to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param need_email: Pass :code:`True` if you require the user's email address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param need_shipping_address: Pass :code:`True` if you require the user's shipping address to complete the order. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param send_phone_number_to_provider: Pass :code:`True` if the user's phone number should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param send_email_to_provider: Pass :code:`True` if the user's email address should be sent to the provider. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param is_flexible: Pass :code:`True` if the final price depends on the shipping method. Ignored for payments in `Telegram Stars <https://t.me/BotNews/90>`_.
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_. If empty, one 'Pay :code:`total price`' button will be shown. If not empty, the first button must be a Pay button.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendInvoice(
            chat_id=chat_id,
            title=title,
            description=description,
            payload=payload,
            currency=currency,
            prices=prices,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_location(
        self,
        chat_id: ChatIdUnion,
        latitude: float,
        longitude: float,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send point on the map. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendlocation

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param latitude: Latitude of the location
        :param longitude: Longitude of the location
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param horizontal_accuracy: The radius of uncertainty for the location, measured in meters; 0-1500
        :param live_period: Period in seconds during which the location will be updated (see `Live Locations <https://telegram.org/blog/live-locations>`_, should be between 60 and 86400, or 0x7FFFFFFF for live locations that can be edited indefinitely
        :param heading: For live locations, a direction in which the user is moving, in degrees. Must be between 1 and 360 if specified.
        :param proximity_alert_radius: For live locations, a maximum distance for proximity alerts about approaching another chat member, in meters. Must be between 1 and 100000 if specified.
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendLocation(
            chat_id=chat_id,
            latitude=latitude,
            longitude=longitude,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_media_group(
        self,
        chat_id: ChatIdUnion,
        media: list[MediaUnion],
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | Default | None = Default("protect_content"),
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        allow_sending_without_reply: bool | None = None,
        reply_to_message_id: int | None = None,
        request_timeout: int | None = None,
    ) -> list[Message]:
        """
        Use this method to send a group of photos, live photos, videos, documents or audios as an album. Documents and audio files can be only grouped in an album with messages of the same type. On success, an array of :class:`aiogram.types.message.Message` objects that were sent is returned.

        Source: https://core.telegram.org/bots/api#sendmediagroup

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param media: A JSON-serialized array describing messages to be sent, must include 2-10 items
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the messages will be sent; required if the messages are sent to a direct messages chat
        :param disable_notification: Sends messages `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent messages from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the messages are a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, an array of :class:`aiogram.types.message.Message` objects that were sent is returned.
        """

        call = SendMediaGroup(
            chat_id=chat_id,
            media=media,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            allow_sending_without_reply=allow_sending_without_reply,
            reply_to_message_id=reply_to_message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_message(
        self,
        chat_id: ChatIdUnion,
        text: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send text messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendmessage

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param text: Text of the message to be sent, 1-4096 characters after entities parsing
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param link_preview_options: Link preview generation options for the message
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param disable_web_page_preview: Disables link previews for links in this message
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendMessage(
            chat_id=chat_id,
            text=text,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_photo(
        self,
        chat_id: ChatIdUnion,
        photo: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send photos. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendphoto

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param photo: Photo to send. Pass a file_id as String to send a photo that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a photo from the Internet, or upload a new photo using multipart/form-data. The photo must be at most 10 MB in size. The photo's width and height must not exceed 10000 in total. Width and height ratio must be at most 20. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Photo caption (may also be used when resending photos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the photo caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the photo needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendPhoto(
            chat_id=chat_id,
            photo=photo,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_poll(
        self,
        chat_id: ChatIdUnion,
        question: str,
        options: list[InputPollOptionUnion],
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send a native poll. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpoll

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`. Polls can't be sent to channel direct messages chats.
        :param question: Poll question, 1-300 characters
        :param options: A JSON-serialized list of 1-12 answer options
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param question_parse_mode: Mode for parsing entities in the question. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Currently, only custom emoji entities are allowed.
        :param question_entities: A JSON-serialized list of special entities that appear in the poll question. It can be specified instead of *question_parse_mode*.
        :param is_anonymous: :code:`True`, if the poll needs to be anonymous, defaults to :code:`True`
        :param type: Poll type, 'quiz' or 'regular', defaults to 'regular'
        :param allows_multiple_answers: Pass :code:`True`, if the poll allows multiple answers, defaults to :code:`False`
        :param allows_revoting: Pass :code:`True`, if the poll allows to change chosen answer options, defaults to :code:`False` for quizzes and to :code:`True` for regular polls
        :param shuffle_options: Pass :code:`True`, if the poll options must be shown in random order
        :param allow_adding_options: Pass :code:`True`, if answer options can be added to the poll after creation; not supported for anonymous polls and quizzes
        :param hide_results_until_closes: Pass :code:`True`, if poll results must be shown only after the poll closes
        :param members_only: Pass :code:`True`, if voting is limited to users who have been members of the chat where the poll is being sent for more than 24 hours; for channel chats only
        :param country_codes: A JSON-serialized list of 0-12 two-letter `ISO 3166-1 alpha-2 <https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2>`_ country codes indicating the countries from which users can vote in the poll; for channel chats only. Use 'FT' as a country code to allow users with anonymous numbers to vote. If omitted or empty, then users from any country can participate in the poll.
        :param correct_option_ids: A JSON-serialized list of monotonically increasing 0-based identifiers of the correct answer options, required for polls in quiz mode
        :param explanation: Text that is shown when a user chooses an incorrect answer or taps on the lamp icon in a quiz-style poll, 0-200 characters with at most 2 line feeds after entities parsing
        :param explanation_parse_mode: Mode for parsing entities in the explanation. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param explanation_entities: A JSON-serialized list of special entities that appear in the poll explanation. It can be specified instead of *explanation_parse_mode*.
        :param explanation_media: Media added to the quiz explanation
        :param open_period: Amount of time in seconds the poll will be active after creation, 5-2628000. Can't be used together with *close_date*.
        :param close_date: Point in time (Unix timestamp) when the poll will be automatically closed. Must be at least 5 and no more than 2628000 seconds in the future. Can't be used together with *open_period*.
        :param is_closed: Pass :code:`True` if the poll needs to be immediately closed. This can be useful for poll preview.
        :param description: Description of the poll to be sent, 0-1024 characters after entities parsing
        :param description_parse_mode: Mode for parsing entities in the poll description. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param description_entities: A JSON-serialized list of special entities that appear in the poll description, which can be specified instead of *description_parse_mode*
        :param media: Media added to the poll description
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param correct_option_id: 0-based identifier of the correct answer option, required for polls in quiz mode
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendPoll(
            chat_id=chat_id,
            question=question,
            options=options,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_sticker(
        self,
        chat_id: ChatIdUnion,
        sticker: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send static .WEBP, `animated <https://telegram.org/blog/animated-stickers>`_ .TGS, or `video <https://telegram.org/blog/video-stickers-better-reactions>`_ .WEBM stickers. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendsticker

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param sticker: Sticker to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a .WEBP sticker from the Internet, or upload a new .WEBP, .TGS, or .WEBM sticker using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Video and animated stickers can't be sent via an HTTP URL.
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param emoji: Emoji associated with the sticker; only for just uploaded stickers
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendSticker(
            chat_id=chat_id,
            sticker=sticker,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_venue(
        self,
        chat_id: ChatIdUnion,
        latitude: float,
        longitude: float,
        title: str,
        address: str,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send information about a venue. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvenue

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param latitude: Latitude of the venue
        :param longitude: Longitude of the venue
        :param title: Name of the venue
        :param address: Address of the venue
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param foursquare_id: Foursquare identifier of the venue
        :param foursquare_type: Foursquare type of the venue, if known. (For example, 'arts_entertainment/default', 'arts_entertainment/aquarium' or 'food/icecream'.)
        :param google_place_id: Google Places identifier of the venue
        :param google_place_type: Google Places type of the venue. (See `supported types <https://developers.google.com/places/web-service/supported_types>`_.)
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendVenue(
            chat_id=chat_id,
            latitude=latitude,
            longitude=longitude,
            title=title,
            address=address,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_video(
        self,
        chat_id: ChatIdUnion,
        video: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send video files, Telegram clients support MPEG4 videos (other formats may be sent as :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvideo

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param video: Video to send. Pass a file_id as String to send a video that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a video from the Internet, or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param width: Video width
        :param height: Video height
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param cover: Cover for the video in the message. Pass a file_id to send a file that exists on the Telegram servers (recommended), pass an HTTP URL for Telegram to get a file from the Internet, or pass 'attach://<file_attach_name>' to upload a new one using multipart/form-data under <file_attach_name> name. :ref:`More information on Sending Files » <sending-files>`
        :param start_timestamp: Start timestamp for the video in the message
        :param caption: Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the video needs to be covered with a spoiler animation
        :param supports_streaming: Pass :code:`True` if the uploaded video is suitable for streaming
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: Bots can currently send video files of up to 50 MB in size, this limit may be changed in the future.
        """

        call = SendVideo(
            chat_id=chat_id,
            video=video,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_video_note(
        self,
        chat_id: ChatIdUnion,
        video_note: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        As of `v.4.0 <https://telegram.org/blog/video-messages-and-telescope>`_, Telegram clients support rounded square MPEG4 videos of up to 1 minute long. Use this method to send video messages. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendvideonote

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param video_note: Video note to send. Pass a file_id as String to send a video note that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending video notes by a URL is currently unsupported.
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param duration: Duration of sent video in seconds
        :param length: Video width and height, i.e. diameter of the video message
        :param thumbnail: Thumbnail of the file sent; can be ignored if thumbnail generation for the file is supported server-side. The thumbnail should be in JPEG format and less than 200 kB in size. A thumbnail's width and height should not exceed 320. Ignored if the file is not uploaded using multipart/form-data. Thumbnails can't be reused and can be only uploaded as a new file, so you can pass 'attach://<file_attach_name>' if the thumbnail was uploaded using multipart/form-data under <file_attach_name>. :ref:`More information on Sending Files » <sending-files>`
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendVideoNote(
            chat_id=chat_id,
            video_note=video_note,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def send_voice(
        self,
        chat_id: ChatIdUnion,
        voice: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message. For this to work, your audio must be in an .OGG file encoded with OPUS, or in .MP3 format, or in .M4A format (other formats may be sent as :class:`aiogram.types.audio.Audio` or :class:`aiogram.types.document.Document`). On success, the sent :class:`aiogram.types.message.Message` is returned. Bots can currently send voice messages of up to 50 MB in size, this limit may be changed in the future.

        Source: https://core.telegram.org/bots/api#sendvoice

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param voice: Audio file to send. Pass a file_id as String to send a file that exists on the Telegram servers (recommended), pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Voice message caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the voice message caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param duration: Duration of the voice message in seconds
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param allow_sending_without_reply: Pass :code:`True` if the message should be sent even if the specified replied-to message is not found
        :param reply_to_message_id: If the message is a reply, ID of the original message
        :param request_timeout: Request timeout
        :return: Bots can currently send voice messages of up to 50 MB in size, this limit may be changed in the future.
        """

        call = SendVoice(
            chat_id=chat_id,
            voice=voice,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_administrator_custom_title(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        custom_title: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set a custom title for an administrator in a supergroup promoted by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatadministratorcustomtitle

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param custom_title: New custom title for the administrator; 0-16 characters, emoji are not allowed
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatAdministratorCustomTitle(
            chat_id=chat_id,
            user_id=user_id,
            custom_title=custom_title,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_description(
        self,
        chat_id: ChatIdUnion,
        description: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the description of a group, a supergroup or a channel. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatdescription

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param description: New chat description, 0-255 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatDescription(
            chat_id=chat_id,
            description=description,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_menu_button(
        self,
        chat_id: int | None = None,
        menu_button: MenuButtonUnion | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the bot's menu button in a private chat, or the default menu button. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatmenubutton

        :param chat_id: Unique identifier for the target private chat. If not specified, the bot's default menu button will be changed.
        :param menu_button: A JSON-serialized object for the bot's new menu button. Defaults to :class:`aiogram.types.menu_button_default.MenuButtonDefault`.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatMenuButton(
            chat_id=chat_id,
            menu_button=menu_button,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_permissions(
        self,
        chat_id: ChatIdUnion,
        permissions: ChatPermissions,
        use_independent_chat_permissions: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set default chat permissions for all members. The bot must be an administrator in the group or a supergroup for this to work and must have the *can_restrict_members* administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatpermissions

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param permissions: A JSON-serialized object for new default chat permissions
        :param use_independent_chat_permissions: Pass :code:`True` if chat permissions are set independently. Otherwise, the *can_send_other_messages* and *can_add_web_page_previews* permissions will imply the *can_send_messages*, *can_send_audios*, *can_send_documents*, *can_send_photos*, *can_send_videos*, *can_send_video_notes*, and *can_send_voice_notes* permissions; the *can_send_polls* permission will imply the *can_send_messages* permission.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatPermissions(
            chat_id=chat_id,
            permissions=permissions,
            use_independent_chat_permissions=use_independent_chat_permissions,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_photo(
        self,
        chat_id: ChatIdUnion,
        photo: InputFile,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set a new profile photo for the chat. Photos can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatphoto

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param photo: New chat photo, uploaded using multipart/form-data
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatPhoto(
            chat_id=chat_id,
            photo=photo,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_sticker_set(
        self,
        chat_id: ChatIdUnion,
        sticker_set_name: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set a new group sticker set for a supergroup. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Use the field *can_set_sticker_set* optionally returned in :class:`aiogram.methods.get_chat.GetChat` requests to check if the bot can use this method. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatstickerset

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param sticker_set_name: Name of the sticker set to be set as the group sticker set
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatStickerSet(
            chat_id=chat_id,
            sticker_set_name=sticker_set_name,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_title(
        self,
        chat_id: ChatIdUnion,
        title: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the title of a chat. Titles can't be changed for private chats. The bot must be an administrator in the chat for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchattitle

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param title: New chat title, 1-128 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatTitle(
            chat_id=chat_id,
            title=title,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_game_score(
        self,
        user_id: int,
        score: int,
        force: bool | None = None,
        disable_edit_message: bool | None = None,
        chat_id: int | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to set the score of the specified user in a game message. On success, if the message is not an inline message, the :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned. Returns an error, if the new score is not greater than the user's current score in the chat and *force* is :code:`False`.

        Source: https://core.telegram.org/bots/api#setgamescore

        :param user_id: User identifier
        :param score: New score, must be non-negative
        :param force: Pass :code:`True` if the high score is allowed to decrease. This can be useful when fixing mistakes or banning cheaters.
        :param disable_edit_message: Pass :code:`True` if the game message should not be automatically edited to include the current scoreboard
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the sent message.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param request_timeout: Request timeout
        :return: Returns an error, if the new score is not greater than the user's current score in the chat and *force* is :code:`False`.
        """

        call = SetGameScore(
            user_id=user_id,
            score=score,
            force=force,
            disable_edit_message=disable_edit_message,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_my_commands(
        self,
        commands: list[BotCommand],
        scope: BotCommandScopeUnion | None = None,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the list of the bot's commands. See `this manual <https://core.telegram.org/bots/features#commands>`_ for more details about bot commands. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmycommands

        :param commands: A JSON-serialized list of bot commands to be set as the list of the bot's commands. At most 100 commands can be specified.
        :param scope: A JSON-serialized object, describing scope of users for which the commands are relevant. Defaults to :class:`aiogram.types.bot_command_scope_default.BotCommandScopeDefault`.
        :param language_code: A two-letter ISO 639-1 language code. If empty, commands will be applied to all users from the given scope, for whose language there are no dedicated commands.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMyCommands(
            commands=commands,
            scope=scope,
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_my_default_administrator_rights(
        self,
        rights: ChatAdministratorRights | None = None,
        for_channels: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the default administrator rights requested by the bot when it's added as an administrator to groups or channels. These rights will be suggested to users, but they are free to modify the list before adding the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmydefaultadministratorrights

        :param rights: A JSON-serialized object describing new default administrator rights. If not specified, the default administrator rights will be cleared.
        :param for_channels: Pass :code:`True` to change the default administrator rights of the bot in channels. Otherwise, the default administrator rights of the bot for groups and supergroups will be changed.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMyDefaultAdministratorRights(
            rights=rights,
            for_channels=for_channels,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_passport_data_errors(
        self,
        user_id: int,
        errors: list[PassportElementErrorUnion],
        request_timeout: int | None = None,
    ) -> bool:
        """
        Informs a user that some of the Telegram Passport elements they provided contains errors. The user will not be able to re-submit their Passport to you until the errors are fixed (the contents of the field for which you returned the error must change). Returns :code:`True` on success.
        Use this if the data submitted by the user doesn't satisfy the standards your service requires for any reason. For example, if a birthday date seems invalid, a submitted document is blurry, a scan shows evidence of tampering, etc. Supply some details in the error message to make sure the user knows how to correct the issues.

        Source: https://core.telegram.org/bots/api#setpassportdataerrors

        :param user_id: User identifier
        :param errors: A JSON-serialized array describing the errors
        :param request_timeout: Request timeout
        :return: Supply some details in the error message to make sure the user knows how to correct the issues.
        """

        call = SetPassportDataErrors(
            user_id=user_id,
            errors=errors,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_sticker_position_in_set(
        self,
        sticker: str,
        position: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to move a sticker in a set created by the bot to a specific position. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickerpositioninset

        :param sticker: File identifier of the sticker
        :param position: New sticker position in the set, zero-based
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetStickerPositionInSet(
            sticker=sticker,
            position=position,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_webhook(
        self,
        url: str,
        certificate: InputFile | None = None,
        ip_address: str | None = None,
        max_connections: int | None = None,
        allowed_updates: list[str] | None = None,
        drop_pending_updates: bool | None = None,
        secret_token: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to specify a URL and receive incoming updates via an outgoing webhook. Whenever there is an update for the bot, we will send an HTTPS POST request to the specified URL, containing a JSON-serialized :class:`aiogram.types.update.Update`. In case of an unsuccessful request (a request with response `HTTP status code <https://en.wikipedia.org/wiki/List_of_HTTP_status_codes>`_ different from :code:`2XY`), we will repeat the request and give up after a reasonable amount of attempts. Returns :code:`True` on success.
        If you'd like to make sure that the webhook was set by you, you can specify secret data in the parameter *secret_token*. If specified, the request will contain a header 'X-Telegram-Bot-Api-Secret-Token' with the secret token as content.

         **Notes**

         **1.** You will not be able to receive updates using :class:`aiogram.methods.get_updates.GetUpdates` for as long as an outgoing webhook is set up.

         **2.** To use a self-signed certificate, you need to upload your `public key certificate <https://core.telegram.org/bots/self-signed>`_ using *certificate* parameter. Please upload as InputFile, sending a String will not work.

         **3.** Ports currently supported *for webhooks*: **443, 80, 88, 8443**.
         If you're having any trouble setting up webhooks, please check out this `amazing guide to webhooks <https://core.telegram.org/bots/webhooks>`_.

        Source: https://core.telegram.org/bots/api#setwebhook

        :param url: HTTPS URL to send updates to. Use an empty string to remove webhook integration.
        :param certificate: Upload your public key certificate so that the root certificate in use can be checked. See our `self-signed guide <https://core.telegram.org/bots/self-signed>`_ for details.
        :param ip_address: The fixed IP address which will be used to send webhook requests instead of the IP address resolved through DNS
        :param max_connections: The maximum allowed number of simultaneous HTTPS connections to the webhook for update delivery, 1-100. Defaults to *40*. Use lower values to limit the load on your bot's server, and higher values to increase your bot's throughput.
        :param allowed_updates: A JSON-serialized list of the update types you want your bot to receive. For example, specify :code:`["message", "edited_channel_post", "callback_query"]` to only receive updates of these types. See :class:`aiogram.types.update.Update` for a complete list of available update types. Specify an empty list to receive all update types except *chat_member*, *message_reaction*, and *message_reaction_count* (default). If not specified, the previous setting will be used.
        :param drop_pending_updates: Pass :code:`True` to drop all pending updates
        :param secret_token: A secret token to be sent in a header 'X-Telegram-Bot-Api-Secret-Token' in every webhook request, 1-256 characters. Only characters :code:`A-Z`, :code:`a-z`, :code:`0-9`, :code:`_` and :code:`-` are allowed. The header is useful to ensure that the request comes from a webhook set by you.
        :param request_timeout: Request timeout
        :return: Please upload as InputFile, sending a String will not work.
        """

        call = SetWebhook(
            url=url,
            certificate=certificate,
            ip_address=ip_address,
            max_connections=max_connections,
            allowed_updates=allowed_updates,
            drop_pending_updates=drop_pending_updates,
            secret_token=secret_token,
        )
        return await self(call, request_timeout=request_timeout)

    async def stop_message_live_location(
        self,
        business_connection_id: str | None = None,
        chat_id: ChatIdUnion | None = None,
        message_id: int | None = None,
        inline_message_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message | bool:
        """
        Use this method to stop updating a live location message before *live_period* expires. On success, if the message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.

        Source: https://core.telegram.org/bots/api#stopmessagelivelocation

        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param chat_id: Required if *inline_message_id* is not specified. Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`.
        :param message_id: Required if *inline_message_id* is not specified. Identifier of the message with live location to stop.
        :param inline_message_id: Required if *chat_id* and *message_id* are not specified. Identifier of the inline message.
        :param reply_markup: A JSON-serialized object for a new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: On success, if the message is not an inline message, the edited :class:`aiogram.types.message.Message` is returned, otherwise :code:`True` is returned.
        """

        call = StopMessageLiveLocation(
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            inline_message_id=inline_message_id,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def stop_poll(
        self,
        chat_id: ChatIdUnion,
        message_id: int,
        business_connection_id: str | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Poll:
        """
        Use this method to stop a poll which was sent by the bot. On success, the stopped :class:`aiogram.types.poll.Poll` is returned.

        Source: https://core.telegram.org/bots/api#stoppoll

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param message_id: Identifier of the original message with the poll
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message to be edited was sent
        :param reply_markup: A JSON-serialized object for a new message `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: On success, the stopped :class:`aiogram.types.poll.Poll` is returned.
        """

        call = StopPoll(
            chat_id=chat_id,
            message_id=message_id,
            business_connection_id=business_connection_id,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def unban_chat_member(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        only_if_banned: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to unban a previously banned user in a supergroup or channel. The user will **not** return to the group or channel automatically, but will be able to join via link, etc. The bot must be an administrator for this to work. By default, this method guarantees that after the call the user is not a member of the chat, but will be able to join it. So if the user is a member of the chat they will also be **removed** from the chat. If you don't want this, use the parameter *only_if_banned*. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unbanchatmember

        :param chat_id: Unique identifier for the target group or username of the target supergroup or channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param only_if_banned: Do nothing if the user is not banned
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnbanChatMember(
            chat_id=chat_id,
            user_id=user_id,
            only_if_banned=only_if_banned,
        )
        return await self(call, request_timeout=request_timeout)

    async def unban_chat_sender_chat(
        self,
        chat_id: ChatIdUnion,
        sender_chat_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to unban a previously banned channel chat in a supergroup or channel. The bot must be an administrator for this to work and must have the appropriate administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unbanchatsenderchat

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param sender_chat_id: Unique identifier of the target sender chat
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnbanChatSenderChat(
            chat_id=chat_id,
            sender_chat_id=sender_chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def unpin_all_chat_messages(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to clear the list of pinned messages in a chat. In private chats and channel direct messages chats, no additional rights are required to unpin all pinned messages. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to unpin all pinned messages in groups and channels respectively. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unpinallchatmessages

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnpinAllChatMessages(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def unpin_all_forum_topic_messages(
        self,
        chat_id: ChatIdUnion,
        message_thread_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to clear the list of pinned messages in a forum topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an administrator in the chat for this to work and must have the *can_pin_messages* administrator right in the supergroup. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unpinallforumtopicmessages

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param message_thread_id: Unique identifier for the target message thread of the forum topic
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnpinAllForumTopicMessages(
            chat_id=chat_id,
            message_thread_id=message_thread_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def unpin_chat_message(
        self,
        chat_id: ChatIdUnion,
        business_connection_id: str | None = None,
        message_id: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to remove a message from the list of pinned messages in a chat. In private chats and channel direct messages chats, all messages can be unpinned. Conversely, the bot must be an administrator with the 'can_pin_messages' right or the 'can_edit_messages' right to unpin messages in groups and channels respectively. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unpinchatmessage

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be unpinned
        :param message_id: Identifier of the message to unpin. Required if *business_connection_id* is specified. If not specified, the most recent pinned message (by sending date) will be unpinned.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnpinChatMessage(
            chat_id=chat_id,
            business_connection_id=business_connection_id,
            message_id=message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def upload_sticker_file(
        self,
        user_id: int,
        sticker: InputFile,
        sticker_format: str,
        request_timeout: int | None = None,
    ) -> File:
        """
        Use this method to upload a file with a sticker for later use in the :class:`aiogram.methods.create_new_sticker_set.CreateNewStickerSet`, :class:`aiogram.methods.add_sticker_to_set.AddStickerToSet`, or :class:`aiogram.methods.replace_sticker_in_set.ReplaceStickerInSet` methods (the file can be used multiple times). Returns the uploaded :class:`aiogram.types.file.File` on success.

        Source: https://core.telegram.org/bots/api#uploadstickerfile

        :param user_id: User identifier of sticker file owner
        :param sticker: A file with the sticker in .WEBP, .PNG, .TGS, or .WEBM format. See `https://core.telegram.org/stickers <https://core.telegram.org/stickers>`_`https://core.telegram.org/stickers <https://core.telegram.org/stickers>`_ for technical requirements. :ref:`More information on Sending Files » <sending-files>`
        :param sticker_format: Format of the sticker, must be one of 'static', 'animated', 'video'
        :param request_timeout: Request timeout
        :return: Returns the uploaded :class:`aiogram.types.file.File` on success.
        """

        call = UploadStickerFile(
            user_id=user_id,
            sticker=sticker,
            sticker_format=sticker_format,
        )
        return await self(call, request_timeout=request_timeout)

    async def close_general_forum_topic(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to close an open 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#closegeneralforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = CloseGeneralForumTopic(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_general_forum_topic(
        self,
        chat_id: ChatIdUnion,
        name: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to edit the name of the 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#editgeneralforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param name: New topic name, 1-128 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = EditGeneralForumTopic(
            chat_id=chat_id,
            name=name,
        )
        return await self(call, request_timeout=request_timeout)

    async def hide_general_forum_topic(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to hide the 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights. The topic will be automatically closed if it was open. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#hidegeneralforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = HideGeneralForumTopic(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def reopen_general_forum_topic(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to reopen a closed 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights. The topic will be automatically unhidden if it was hidden. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#reopengeneralforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ReopenGeneralForumTopic(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def unhide_general_forum_topic(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to unhide the 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the *can_manage_topics* administrator rights. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unhidegeneralforumtopic

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnhideGeneralForumTopic(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_sticker_set(
        self,
        name: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete a sticker set that was created by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletestickerset

        :param name: Sticker set name
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteStickerSet(
            name=name,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_my_description(
        self,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> BotDescription:
        """
        Use this method to get the current bot description for the given user language. Returns :class:`aiogram.types.bot_description.BotDescription` on success.

        Source: https://core.telegram.org/bots/api#getmydescription

        :param language_code: A two-letter ISO 639-1 language code or an empty string
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.bot_description.BotDescription` on success.
        """

        call = GetMyDescription(
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_my_short_description(
        self,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> BotShortDescription:
        """
        Use this method to get the current bot short description for the given user language. Returns :class:`aiogram.types.bot_short_description.BotShortDescription` on success.

        Source: https://core.telegram.org/bots/api#getmyshortdescription

        :param language_code: A two-letter ISO 639-1 language code or an empty string
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.bot_short_description.BotShortDescription` on success.
        """

        call = GetMyShortDescription(
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_custom_emoji_sticker_set_thumbnail(
        self,
        name: str,
        custom_emoji_id: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set the thumbnail of a custom emoji sticker set. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setcustomemojistickersetthumbnail

        :param name: Sticker set name
        :param custom_emoji_id: Custom emoji identifier of a sticker from the sticker set; pass an empty string to drop the thumbnail and use the first sticker as the thumbnail
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetCustomEmojiStickerSetThumbnail(
            name=name,
            custom_emoji_id=custom_emoji_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_my_description(
        self,
        description: str | None = None,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the bot's description, which is shown in the chat with the bot if the chat is empty. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmydescription

        :param description: New bot description; 0-512 characters. Pass an empty string to remove the dedicated description for the given language.
        :param language_code: A two-letter ISO 639-1 language code. If empty, the description will be applied to all users for whose language there is no dedicated description.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMyDescription(
            description=description,
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_my_short_description(
        self,
        short_description: str | None = None,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the bot's short description, which is shown on the bot's profile page and is sent together with the link when users share the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmyshortdescription

        :param short_description: New short description for the bot; 0-120 characters. Pass an empty string to remove the dedicated short description for the given language.
        :param language_code: A two-letter ISO 639-1 language code. If empty, the short description will be applied to all users for whose language there is no dedicated short description.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMyShortDescription(
            short_description=short_description,
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_sticker_emoji_list(
        self,
        sticker: str,
        emoji_list: list[str],
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the list of emoji assigned to a regular or custom emoji sticker. The sticker must belong to a sticker set created by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickeremojilist

        :param sticker: File identifier of the sticker
        :param emoji_list: A JSON-serialized list of 1-20 emoji associated with the sticker
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetStickerEmojiList(
            sticker=sticker,
            emoji_list=emoji_list,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_sticker_keywords(
        self,
        sticker: str,
        keywords: list[str] | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change search keywords assigned to a regular or custom emoji sticker. The sticker must belong to a sticker set created by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickerkeywords

        :param sticker: File identifier of the sticker
        :param keywords: A JSON-serialized list of 0-20 search keywords for the sticker with total length of up to 64 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetStickerKeywords(
            sticker=sticker,
            keywords=keywords,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_sticker_mask_position(
        self,
        sticker: str,
        mask_position: MaskPosition | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the `mask position <https://core.telegram.org/bots/api#maskposition>`_ of a mask sticker. The sticker must belong to a sticker set that was created by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickermaskposition

        :param sticker: File identifier of the sticker
        :param mask_position: A JSON-serialized object with the position where the mask should be placed on faces. Omit the parameter to remove the mask position.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetStickerMaskPosition(
            sticker=sticker,
            mask_position=mask_position,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_sticker_set_thumbnail(
        self,
        name: str,
        user_id: int,
        format: str,
        thumbnail: InputFileUnion | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set the thumbnail of a regular or mask sticker set. The format of the thumbnail file must match the format of the stickers in the set. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickersetthumbnail

        :param name: Sticker set name
        :param user_id: User identifier of the sticker set owner
        :param format: Format of the thumbnail, must be one of 'static' for a **.WEBP** or **.PNG** image, 'animated' for a **.TGS** animation, or 'video' for a **.WEBM** video
        :param thumbnail: A **.WEBP** or **.PNG** image with the thumbnail, must be up to 128 kilobytes in size and have a width and height of exactly 100px, or a **.TGS** animation with a thumbnail up to 32 kilobytes in size (see `https://core.telegram.org/stickers#animation-requirements <https://core.telegram.org/stickers#animation-requirements>`_`https://core.telegram.org/stickers#animation-requirements <https://core.telegram.org/stickers#animation-requirements>`_ for animated sticker technical requirements), or a **.WEBM** video with the thumbnail up to 32 kilobytes in size; see `https://core.telegram.org/stickers#video-requirements <https://core.telegram.org/stickers#video-requirements>`_`https://core.telegram.org/stickers#video-requirements <https://core.telegram.org/stickers#video-requirements>`_ for video sticker technical requirements. Pass a *file_id* as a String to send a file that already exists on the Telegram servers, pass an HTTP URL as a String for Telegram to get a file from the Internet, or upload a new one using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Animated and video sticker set thumbnails can't be uploaded via HTTP URL. If omitted, then the thumbnail is dropped and the first sticker is used as the thumbnail.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetStickerSetThumbnail(
            name=name,
            user_id=user_id,
            format=format,
            thumbnail=thumbnail,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_sticker_set_title(
        self,
        name: str,
        title: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set the title of a created sticker set. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setstickersettitle

        :param name: Sticker set name
        :param title: Sticker set title, 1-64 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetStickerSetTitle(
            name=name,
            title=title,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_my_name(
        self,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> BotName:
        """
        Use this method to get the current bot name for the given user language. Returns :class:`aiogram.types.bot_name.BotName` on success.

        Source: https://core.telegram.org/bots/api#getmyname

        :param language_code: A two-letter ISO 639-1 language code or an empty string
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.bot_name.BotName` on success.
        """

        call = GetMyName(
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_my_name(
        self,
        name: str | None = None,
        language_code: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the bot's name. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmyname

        :param name: New bot name; 0-64 characters. Pass an empty string to remove the dedicated name for the given language.
        :param language_code: A two-letter ISO 639-1 language code. If empty, the name will be shown to all users for whose language there is no dedicated name.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMyName(
            name=name,
            language_code=language_code,
        )
        return await self(call, request_timeout=request_timeout)

    async def unpin_all_general_forum_topic_messages(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to clear the list of pinned messages in a General forum topic. The bot must be an administrator in the chat for this to work and must have the *can_pin_messages* administrator right in the supergroup. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#unpinallgeneralforumtopicmessages

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UnpinAllGeneralForumTopicMessages(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def copy_messages(
        self,
        chat_id: ChatIdUnion,
        from_chat_id: ChatIdUnion,
        message_ids: list[int],
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        remove_caption: bool | None = None,
        request_timeout: int | None = None,
    ) -> list[MessageId]:
        """
        Use this method to copy messages of any kind. If some of the specified messages can't be found or copied, they are skipped. Service messages, paid media messages, giveaway messages, giveaway winners messages, and invoice messages can't be copied. A quiz :class:`aiogram.methods.poll.Poll` can be copied only if the value of the field *correct_option_id* is known to the bot. The method is analogous to the method :class:`aiogram.methods.forward_messages.ForwardMessages`, but the copied messages don't have a link to the original message. Album grouping is kept for copied messages. On success, an array of :class:`aiogram.types.message_id.MessageId` of the sent messages is returned.

        Source: https://core.telegram.org/bots/api#copymessages

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param from_chat_id: Unique identifier for the chat where the original messages were sent (or username of the target bot, supergroup or channel in the format :code:`@username`)
        :param message_ids: A JSON-serialized list of 1-100 identifiers of messages in the chat *from_chat_id* to copy. The identifiers must be specified in a strictly increasing order.
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the messages will be sent; required if the messages are sent to a direct messages chat
        :param disable_notification: Sends the messages `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent messages from forwarding and saving
        :param remove_caption: Pass :code:`True` to copy the messages without their captions
        :param request_timeout: Request timeout
        :return: On success, an array of :class:`aiogram.types.message_id.MessageId` of the sent messages is returned.
        """

        call = CopyMessages(
            chat_id=chat_id,
            from_chat_id=from_chat_id,
            message_ids=message_ids,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            remove_caption=remove_caption,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_messages(
        self,
        chat_id: ChatIdUnion,
        message_ids: list[int],
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to delete multiple messages simultaneously. If some of the specified messages can't be found, they are skipped. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletemessages

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param message_ids: A JSON-serialized list of 1-100 identifiers of messages to delete. See :class:`aiogram.methods.delete_message.DeleteMessage` for limitations on which messages can be deleted.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteMessages(
            chat_id=chat_id,
            message_ids=message_ids,
        )
        return await self(call, request_timeout=request_timeout)

    async def forward_messages(
        self,
        chat_id: ChatIdUnion,
        from_chat_id: ChatIdUnion,
        message_ids: list[int],
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        request_timeout: int | None = None,
    ) -> list[MessageId]:
        """
        Use this method to forward multiple messages of any kind. If some of the specified messages can't be found or forwarded, they are skipped. Service messages and messages with protected content can't be forwarded. Album grouping is kept for forwarded messages. On success, an array of :class:`aiogram.types.message_id.MessageId` of the sent messages is returned.

        Source: https://core.telegram.org/bots/api#forwardmessages

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param from_chat_id: Unique identifier for the chat where the original messages were sent (or username of the target bot, supergroup or channel in the format :code:`@username`)
        :param message_ids: A JSON-serialized list of 1-100 identifiers of messages in the chat *from_chat_id* to forward. The identifiers must be specified in a strictly increasing order.
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the messages will be forwarded; required if the messages are forwarded to a direct messages chat
        :param disable_notification: Sends the messages `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the forwarded messages from forwarding and saving
        :param request_timeout: Request timeout
        :return: On success, an array of :class:`aiogram.types.message_id.MessageId` of the sent messages is returned.
        """

        call = ForwardMessages(
            chat_id=chat_id,
            from_chat_id=from_chat_id,
            message_ids=message_ids,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_user_chat_boosts(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        request_timeout: int | None = None,
    ) -> UserChatBoosts:
        """
        Use this method to get the list of boosts added to a chat by a user. Requires administrator rights in the chat. Returns a :class:`aiogram.types.user_chat_boosts.UserChatBoosts` object.

        Source: https://core.telegram.org/bots/api#getuserchatboosts

        :param chat_id: Unique identifier for the chat or username of the channel in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.user_chat_boosts.UserChatBoosts` object.
        """

        call = GetUserChatBoosts(
            chat_id=chat_id,
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_message_reaction(
        self,
        chat_id: ChatIdUnion,
        message_id: int,
        reaction: list[ReactionTypeUnion] | None = None,
        is_big: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the chosen reactions on a message. Service messages of some types can't be reacted to. Automatically forwarded messages from a channel to its discussion group have the same available reactions as messages in the channel. Bots can't use paid reactions. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmessagereaction

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param message_id: Identifier of the target message. If the message belongs to a media group, the reaction is set to the first non-deleted message in the group instead.
        :param reaction: A JSON-serialized list of reaction types to set on the message. Currently, as non-premium users, bots can set up to one reaction per message. A custom emoji reaction can be used if it is either already present on the message or explicitly allowed by chat administrators. Paid reactions can't be used by bots.
        :param is_big: Pass :code:`True` to set the reaction with a big animation
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMessageReaction(
            chat_id=chat_id,
            message_id=message_id,
            reaction=reaction,
            is_big=is_big,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_business_connection(
        self,
        business_connection_id: str,
        request_timeout: int | None = None,
    ) -> BusinessConnection:
        """
        Use this method to get information about the connection of the bot with a business account. Returns a :class:`aiogram.types.business_connection.BusinessConnection` object on success.

        Source: https://core.telegram.org/bots/api#getbusinessconnection

        :param business_connection_id: Unique identifier of the business connection
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.business_connection.BusinessConnection` object on success.
        """

        call = GetBusinessConnection(
            business_connection_id=business_connection_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def replace_sticker_in_set(
        self,
        user_id: int,
        name: str,
        old_sticker: str,
        sticker: InputSticker,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to replace an existing sticker in a sticker set with a new one. The method is equivalent to calling :class:`aiogram.methods.delete_sticker_from_set.DeleteStickerFromSet`, then :class:`aiogram.methods.add_sticker_to_set.AddStickerToSet`, then :class:`aiogram.methods.set_sticker_position_in_set.SetStickerPositionInSet`. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#replacestickerinset

        :param user_id: User identifier of the sticker set owner
        :param name: Sticker set name
        :param old_sticker: File identifier of the replaced sticker
        :param sticker: A JSON-serialized object with information about the added sticker. If exactly the same sticker had already been added to the set, then the set remains unchanged.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ReplaceStickerInSet(
            user_id=user_id,
            name=name,
            old_sticker=old_sticker,
            sticker=sticker,
        )
        return await self(call, request_timeout=request_timeout)

    async def refund_star_payment(
        self,
        user_id: int,
        telegram_payment_charge_id: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Refunds a successful payment in `Telegram Stars <https://t.me/BotNews/90>`_. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#refundstarpayment

        :param user_id: Identifier of the user whose payment will be refunded
        :param telegram_payment_charge_id: Telegram payment identifier
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = RefundStarPayment(
            user_id=user_id,
            telegram_payment_charge_id=telegram_payment_charge_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_star_transactions(
        self,
        offset: int | None = None,
        limit: int | None = None,
        request_timeout: int | None = None,
    ) -> StarTransactions:
        """
        Returns the bot's Telegram Star transactions in chronological order. On success, returns a :class:`aiogram.types.star_transactions.StarTransactions` object.

        Source: https://core.telegram.org/bots/api#getstartransactions

        :param offset: Number of transactions to skip in the response
        :param limit: The maximum number of transactions to be retrieved. Values between 1-100 are accepted. Defaults to 100.
        :param request_timeout: Request timeout
        :return: On success, returns a :class:`aiogram.types.star_transactions.StarTransactions` object.
        """

        call = GetStarTransactions(
            offset=offset,
            limit=limit,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_paid_media(
        self,
        chat_id: ChatIdUnion,
        star_count: int,
        media: list[InputPaidMediaUnion],
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
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
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send paid media. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendpaidmedia

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`. If the chat is a channel, all Telegram Star proceeds from this media will be credited to the chat's balance. Otherwise, they will be credited to the bot's balance.
        :param star_count: The number of Telegram Stars that must be paid to buy access to the media; 1-25000
        :param media: A JSON-serialized array describing the media to be sent; up to 10 items
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param payload: Bot-defined paid media payload, 0-128 bytes. This will not be displayed to the user, use it for your internal processes.
        :param caption: Media caption, 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the media caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendPaidMedia(
            chat_id=chat_id,
            star_count=star_count,
            media=media,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def create_chat_subscription_invite_link(
        self,
        chat_id: ChatIdUnion,
        subscription_period: DateTimeUnion,
        subscription_price: int,
        name: str | None = None,
        request_timeout: int | None = None,
    ) -> ChatInviteLink:
        """
        Use this method to create a `subscription invite link <https://telegram.org/blog/superchannels-star-reactions-subscriptions#star-subscriptions>`_ for a channel chat. The bot must have the *can_invite_users* administrator rights. The link can be edited using the method :class:`aiogram.methods.edit_chat_subscription_invite_link.EditChatSubscriptionInviteLink` or revoked using the method :class:`aiogram.methods.revoke_chat_invite_link.RevokeChatInviteLink`. Returns the new invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

        Source: https://core.telegram.org/bots/api#createchatsubscriptioninvitelink

        :param chat_id: Unique identifier for the target channel chat or username of the target channel in the format :code:`@username`
        :param subscription_period: The number of seconds the subscription will be active for before the next payment. Currently, it must always be 2592000 (30 days).
        :param subscription_price: The amount of Telegram Stars a user must pay initially and after each subsequent subscription period to be a member of the chat; 1-10000
        :param name: Invite link name; 0-32 characters
        :param request_timeout: Request timeout
        :return: Returns the new invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.
        """

        call = CreateChatSubscriptionInviteLink(
            chat_id=chat_id,
            subscription_period=subscription_period,
            subscription_price=subscription_price,
            name=name,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_chat_subscription_invite_link(
        self,
        chat_id: ChatIdUnion,
        invite_link: str,
        name: str | None = None,
        request_timeout: int | None = None,
    ) -> ChatInviteLink:
        """
        Use this method to edit a subscription invite link created by the bot. The bot must have the *can_invite_users* administrator rights. Returns the edited invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.

        Source: https://core.telegram.org/bots/api#editchatsubscriptioninvitelink

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param invite_link: The invite link to edit
        :param name: Invite link name; 0-32 characters
        :param request_timeout: Request timeout
        :return: Returns the edited invite link as a :class:`aiogram.types.chat_invite_link.ChatInviteLink` object.
        """

        call = EditChatSubscriptionInviteLink(
            chat_id=chat_id,
            invite_link=invite_link,
            name=name,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_user_star_subscription(
        self,
        user_id: int,
        telegram_payment_charge_id: str,
        is_canceled: bool,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Allows the bot to cancel or re-enable extension of a subscription paid in Telegram Stars. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#edituserstarsubscription

        :param user_id: Identifier of the user whose subscription will be edited
        :param telegram_payment_charge_id: Telegram payment identifier for the subscription
        :param is_canceled: Pass :code:`True` to cancel extension of the user subscription; the subscription must be active up to the end of the current subscription period. Pass :code:`False` to allow the user to re-enable a subscription that was previously canceled by the bot.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = EditUserStarSubscription(
            user_id=user_id,
            telegram_payment_charge_id=telegram_payment_charge_id,
            is_canceled=is_canceled,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_available_gifts(
        self,
        request_timeout: int | None = None,
    ) -> Gifts:
        """
        Returns the list of gifts that can be sent by the bot to users and channel chats. Requires no parameters. Returns a :class:`aiogram.types.gifts.Gifts` object.

        Source: https://core.telegram.org/bots/api#getavailablegifts

        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.gifts.Gifts` object.
        """

        call = GetAvailableGifts()
        return await self(call, request_timeout=request_timeout)

    async def save_prepared_inline_message(
        self,
        user_id: int,
        result: InlineQueryResultUnion,
        allow_user_chats: bool | None = None,
        allow_bot_chats: bool | None = None,
        allow_group_chats: bool | None = None,
        allow_channel_chats: bool | None = None,
        request_timeout: int | None = None,
    ) -> PreparedInlineMessage:
        """
        Stores a message that can be sent by a user of a Mini App. Returns a :class:`aiogram.types.prepared_inline_message.PreparedInlineMessage` object.

        Source: https://core.telegram.org/bots/api#savepreparedinlinemessage

        :param user_id: Unique identifier of the target user that can use the prepared message
        :param result: A JSON-serialized object describing the message to be sent
        :param allow_user_chats: Pass :code:`True` if the message can be sent to private chats with users
        :param allow_bot_chats: Pass :code:`True` if the message can be sent to private chats with bots
        :param allow_group_chats: Pass :code:`True` if the message can be sent to group and supergroup chats
        :param allow_channel_chats: Pass :code:`True` if the message can be sent to channel chats
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.prepared_inline_message.PreparedInlineMessage` object.
        """

        call = SavePreparedInlineMessage(
            user_id=user_id,
            result=result,
            allow_user_chats=allow_user_chats,
            allow_bot_chats=allow_bot_chats,
            allow_group_chats=allow_group_chats,
            allow_channel_chats=allow_channel_chats,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_gift(
        self,
        gift_id: str,
        user_id: int | None = None,
        chat_id: ChatIdUnion | None = None,
        pay_for_upgrade: bool | None = None,
        text: str | None = None,
        text_parse_mode: str | None = None,
        text_entities: list[MessageEntity] | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Sends a gift to the given user or channel chat. The gift can't be converted to Telegram Stars by the receiver. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#sendgift

        :param gift_id: Identifier of the gift; limited gifts can't be sent to channel chats
        :param user_id: Required if *chat_id* is not specified. Unique identifier of the target user who will receive the gift.
        :param chat_id: Required if *user_id* is not specified. Unique identifier for the chat or username of the channel (in the format :code:`@username`) that will receive the gift.
        :param pay_for_upgrade: Pass :code:`True` to pay for the gift upgrade from the bot's balance, thereby making the upgrade free for the receiver
        :param text: Text that will be shown along with the gift; 0-128 characters
        :param text_parse_mode: Mode for parsing entities in the text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored.
        :param text_entities: A JSON-serialized list of special entities that appear in the gift text. It can be specified instead of *text_parse_mode*. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SendGift(
            gift_id=gift_id,
            user_id=user_id,
            chat_id=chat_id,
            pay_for_upgrade=pay_for_upgrade,
            text=text,
            text_parse_mode=text_parse_mode,
            text_entities=text_entities,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_user_emoji_status(
        self,
        user_id: int,
        emoji_status_custom_emoji_id: str | None = None,
        emoji_status_expiration_date: DateTimeUnion | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the emoji status for a given user that previously allowed the bot to manage their emoji status via the Mini App method `requestEmojiStatusAccess <https://core.telegram.org/bots/webapps#initializing-mini-apps>`_. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setuseremojistatus

        :param user_id: Unique identifier of the target user
        :param emoji_status_custom_emoji_id: Custom emoji identifier of the emoji status to set. Pass an empty string to remove the status.
        :param emoji_status_expiration_date: Expiration date of the emoji status, if any
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetUserEmojiStatus(
            user_id=user_id,
            emoji_status_custom_emoji_id=emoji_status_custom_emoji_id,
            emoji_status_expiration_date=emoji_status_expiration_date,
        )
        return await self(call, request_timeout=request_timeout)

    async def remove_chat_verification(
        self,
        chat_id: ChatIdUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Removes verification from a chat that is currently verified `on behalf of the organization <https://telegram.org/verify#third-party-verification>`_ represented by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#removechatverification

        :param chat_id: Unique identifier for the target chat or username of the target bot or channel in the format :code:`@username`
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = RemoveChatVerification(
            chat_id=chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def remove_user_verification(
        self,
        user_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Removes verification from a user who is currently verified `on behalf of the organization <https://telegram.org/verify#third-party-verification>`_ represented by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#removeuserverification

        :param user_id: Unique identifier of the target user
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = RemoveUserVerification(
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def verify_chat(
        self,
        chat_id: ChatIdUnion,
        custom_description: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Verifies a chat `on behalf of the organization <https://telegram.org/verify#third-party-verification>`_ which is represented by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#verifychat

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`. Channel direct messages chats can't be verified.
        :param custom_description: Custom description for the verification; 0-70 characters. Must be empty if the organization isn't allowed to provide a custom verification description.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = VerifyChat(
            chat_id=chat_id,
            custom_description=custom_description,
        )
        return await self(call, request_timeout=request_timeout)

    async def verify_user(
        self,
        user_id: int,
        custom_description: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Verifies a user `on behalf of the organization <https://telegram.org/verify#third-party-verification>`_ which is represented by the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#verifyuser

        :param user_id: Unique identifier of the target user
        :param custom_description: Custom description for the verification; 0-70 characters. Must be empty if the organization isn't allowed to provide a custom verification description.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = VerifyUser(
            user_id=user_id,
            custom_description=custom_description,
        )
        return await self(call, request_timeout=request_timeout)

    async def convert_gift_to_stars(
        self,
        business_connection_id: str,
        owned_gift_id: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Converts a given regular gift to Telegram Stars. Requires the *can_convert_gifts_to_stars* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#convertgifttostars

        :param business_connection_id: Unique identifier of the business connection
        :param owned_gift_id: Unique identifier of the regular gift that should be converted to Telegram Stars
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ConvertGiftToStars(
            business_connection_id=business_connection_id,
            owned_gift_id=owned_gift_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_business_messages(
        self,
        business_connection_id: str,
        message_ids: list[int],
        request_timeout: int | None = None,
    ) -> bool:
        """
        Delete messages on behalf of a business account. Requires the *can_delete_sent_messages* business bot right to delete messages sent by the bot itself, or the *can_delete_all_messages* business bot right to delete any message. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletebusinessmessages

        :param business_connection_id: Unique identifier of the business connection on behalf of which to delete the messages
        :param message_ids: A JSON-serialized list of 1-100 identifiers of messages to delete. All messages must be from the same chat. See :class:`aiogram.methods.delete_message.DeleteMessage` for limitations on which messages can be deleted.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteBusinessMessages(
            business_connection_id=business_connection_id,
            message_ids=message_ids,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_story(
        self,
        business_connection_id: str,
        story_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Deletes a story previously posted by the bot on behalf of a managed business account. Requires the *can_manage_stories* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletestory

        :param business_connection_id: Unique identifier of the business connection
        :param story_id: Unique identifier of the story to delete
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteStory(
            business_connection_id=business_connection_id,
            story_id=story_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_story(
        self,
        business_connection_id: str,
        story_id: int,
        content: InputStoryContentUnion,
        caption: str | None = None,
        parse_mode: str | None = None,
        caption_entities: list[MessageEntity] | None = None,
        areas: list[StoryArea] | None = None,
        request_timeout: int | None = None,
    ) -> Story:
        """
        Edits a story previously posted by the bot on behalf of a managed business account. Requires the *can_manage_stories* business bot right. Returns :class:`aiogram.types.story.Story` on success.

        Source: https://core.telegram.org/bots/api#editstory

        :param business_connection_id: Unique identifier of the business connection
        :param story_id: Unique identifier of the story to edit
        :param content: Content of the story
        :param caption: Caption of the story, 0-2048 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the story caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param areas: A JSON-serialized list of clickable areas to be shown on the story
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.story.Story` on success.
        """

        call = EditStory(
            business_connection_id=business_connection_id,
            story_id=story_id,
            content=content,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            areas=areas,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_business_account_gifts(
        self,
        business_connection_id: str,
        exclude_unsaved: bool | None = None,
        exclude_saved: bool | None = None,
        exclude_unlimited: bool | None = None,
        exclude_limited_upgradable: bool | None = None,
        exclude_limited_non_upgradable: bool | None = None,
        exclude_unique: bool | None = None,
        exclude_from_blockchain: bool | None = None,
        sort_by_price: bool | None = None,
        offset: str | None = None,
        limit: int | None = None,
        exclude_limited: bool | None = None,
        request_timeout: int | None = None,
    ) -> OwnedGifts:
        """
        Returns the gifts received and owned by a managed business account. Requires the *can_view_gifts_and_stars* business bot right. Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.

        Source: https://core.telegram.org/bots/api#getbusinessaccountgifts

        :param business_connection_id: Unique identifier of the business connection
        :param exclude_unsaved: Pass :code:`True` to exclude gifts that aren't saved to the account's profile page
        :param exclude_saved: Pass :code:`True` to exclude gifts that are saved to the account's profile page
        :param exclude_unlimited: Pass :code:`True` to exclude gifts that can be purchased an unlimited number of times
        :param exclude_limited_upgradable: Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can be upgraded to unique
        :param exclude_limited_non_upgradable: Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can't be upgraded to unique
        :param exclude_unique: Pass :code:`True` to exclude unique gifts
        :param exclude_from_blockchain: Pass :code:`True` to exclude gifts that were assigned from the TON blockchain and can't be resold or transferred in Telegram
        :param sort_by_price: Pass :code:`True` to sort results by gift price instead of send date. Sorting is applied before pagination.
        :param offset: Offset of the first entry to return as received from the previous request; use empty string to get the first chunk of results
        :param limit: The maximum number of gifts to be returned; 1-100. Defaults to 100.
        :param exclude_limited: Pass :code:`True` to exclude gifts that can be purchased a limited number of times
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.
        """

        call = GetBusinessAccountGifts(
            business_connection_id=business_connection_id,
            exclude_unsaved=exclude_unsaved,
            exclude_saved=exclude_saved,
            exclude_unlimited=exclude_unlimited,
            exclude_limited_upgradable=exclude_limited_upgradable,
            exclude_limited_non_upgradable=exclude_limited_non_upgradable,
            exclude_unique=exclude_unique,
            exclude_from_blockchain=exclude_from_blockchain,
            sort_by_price=sort_by_price,
            offset=offset,
            limit=limit,
            exclude_limited=exclude_limited,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_business_account_star_balance(
        self,
        business_connection_id: str,
        request_timeout: int | None = None,
    ) -> StarAmount:
        """
        Returns the amount of Telegram Stars owned by a managed business account. Requires the *can_view_gifts_and_stars* business bot right. Returns :class:`aiogram.types.star_amount.StarAmount` on success.

        Source: https://core.telegram.org/bots/api#getbusinessaccountstarbalance

        :param business_connection_id: Unique identifier of the business connection
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.star_amount.StarAmount` on success.
        """

        call = GetBusinessAccountStarBalance(
            business_connection_id=business_connection_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def gift_premium_subscription(
        self,
        user_id: int,
        month_count: int,
        star_count: int,
        text: str | None = None,
        text_parse_mode: str | None = None,
        text_entities: list[MessageEntity] | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Gifts a Telegram Premium subscription to the given user. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#giftpremiumsubscription

        :param user_id: Unique identifier of the target user who will receive a Telegram Premium subscription
        :param month_count: Number of months the Telegram Premium subscription will be active for the user; must be one of 3, 6, or 12
        :param star_count: Number of Telegram Stars to pay for the Telegram Premium subscription; must be 1000 for 3 months, 1500 for 6 months, and 2500 for 12 months
        :param text: Text that will be shown along with the service message about the subscription; 0-128 characters
        :param text_parse_mode: Mode for parsing entities in the text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored.
        :param text_entities: A JSON-serialized list of special entities that appear in the gift text. It can be specified instead of *text_parse_mode*. Entities other than 'bold', 'italic', 'underline', 'strikethrough', 'spoiler', 'custom_emoji', and 'date_time' are ignored.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = GiftPremiumSubscription(
            user_id=user_id,
            month_count=month_count,
            star_count=star_count,
            text=text,
            text_parse_mode=text_parse_mode,
            text_entities=text_entities,
        )
        return await self(call, request_timeout=request_timeout)

    async def post_story(
        self,
        business_connection_id: str,
        content: InputStoryContentUnion,
        active_period: int,
        caption: str | None = None,
        parse_mode: str | None = None,
        caption_entities: list[MessageEntity] | None = None,
        areas: list[StoryArea] | None = None,
        post_to_chat_page: bool | None = None,
        protect_content: bool | None = None,
        request_timeout: int | None = None,
    ) -> Story:
        """
        Posts a story on behalf of a managed business account. Requires the *can_manage_stories* business bot right. Returns :class:`aiogram.types.story.Story` on success.

        Source: https://core.telegram.org/bots/api#poststory

        :param business_connection_id: Unique identifier of the business connection
        :param content: Content of the story
        :param active_period: Period after which the story is moved to the archive, in seconds; must be one of :code:`6 * 3600`, :code:`12 * 3600`, :code:`86400`, or :code:`2 * 86400`
        :param caption: Caption of the story, 0-2048 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the story caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param areas: A JSON-serialized list of clickable areas to be shown on the story
        :param post_to_chat_page: Pass :code:`True` to keep the story accessible after it expires
        :param protect_content: Pass :code:`True` if the content of the story must be protected from forwarding and screenshotting
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.story.Story` on success.
        """

        call = PostStory(
            business_connection_id=business_connection_id,
            content=content,
            active_period=active_period,
            caption=caption,
            parse_mode=parse_mode,
            caption_entities=caption_entities,
            areas=areas,
            post_to_chat_page=post_to_chat_page,
            protect_content=protect_content,
        )
        return await self(call, request_timeout=request_timeout)

    async def read_business_message(
        self,
        business_connection_id: str,
        chat_id: int,
        message_id: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Marks incoming message as read on behalf of a business account. Requires the *can_read_messages* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#readbusinessmessage

        :param business_connection_id: Unique identifier of the business connection on behalf of which to read the message
        :param chat_id: Unique identifier of the chat in which the message was received. The chat must have been active in the last 24 hours.
        :param message_id: Unique identifier of the message to mark as read
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ReadBusinessMessage(
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def remove_business_account_profile_photo(
        self,
        business_connection_id: str,
        is_public: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Removes the current profile photo of a managed business account. Requires the *can_edit_profile_photo* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#removebusinessaccountprofilephoto

        :param business_connection_id: Unique identifier of the business connection
        :param is_public: Pass :code:`True` to remove the public photo, which is visible even if the main photo is hidden by the business account's privacy settings. After the main photo is removed, the previous profile photo (if present) becomes the main photo.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = RemoveBusinessAccountProfilePhoto(
            business_connection_id=business_connection_id,
            is_public=is_public,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_business_account_bio(
        self,
        business_connection_id: str,
        bio: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the bio of a managed business account. Requires the *can_change_bio* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setbusinessaccountbio

        :param business_connection_id: Unique identifier of the business connection
        :param bio: The new value of the bio for the business account; 0-140 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetBusinessAccountBio(
            business_connection_id=business_connection_id,
            bio=bio,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_business_account_gift_settings(
        self,
        business_connection_id: str,
        show_gift_button: bool,
        accepted_gift_types: AcceptedGiftTypes,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the privacy settings pertaining to incoming gifts in a managed business account. Requires the *can_change_gift_settings* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setbusinessaccountgiftsettings

        :param business_connection_id: Unique identifier of the business connection
        :param show_gift_button: Pass :code:`True`, if a button for sending a gift to the user or by the business account must always be shown in the input field
        :param accepted_gift_types: Types of gifts accepted by the business account
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetBusinessAccountGiftSettings(
            business_connection_id=business_connection_id,
            show_gift_button=show_gift_button,
            accepted_gift_types=accepted_gift_types,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_business_account_name(
        self,
        business_connection_id: str,
        first_name: str,
        last_name: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the first and last name of a managed business account. Requires the *can_change_name* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setbusinessaccountname

        :param business_connection_id: Unique identifier of the business connection
        :param first_name: The new value of the first name for the business account; 1-64 characters
        :param last_name: The new value of the last name for the business account; 0-64 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetBusinessAccountName(
            business_connection_id=business_connection_id,
            first_name=first_name,
            last_name=last_name,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_business_account_profile_photo(
        self,
        business_connection_id: str,
        photo: InputProfilePhotoUnion,
        is_public: bool | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the profile photo of a managed business account. Requires the *can_edit_profile_photo* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setbusinessaccountprofilephoto

        :param business_connection_id: Unique identifier of the business connection
        :param photo: The new profile photo to set
        :param is_public: Pass :code:`True` to set the public photo, which will be visible even if the main photo is hidden by the business account's privacy settings. An account can have only one public photo.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetBusinessAccountProfilePhoto(
            business_connection_id=business_connection_id,
            photo=photo,
            is_public=is_public,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_business_account_username(
        self,
        business_connection_id: str,
        username: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the username of a managed business account. Requires the *can_change_username* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setbusinessaccountusername

        :param business_connection_id: Unique identifier of the business connection
        :param username: The new value of the username for the business account; 0-32 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetBusinessAccountUsername(
            business_connection_id=business_connection_id,
            username=username,
        )
        return await self(call, request_timeout=request_timeout)

    async def transfer_business_account_stars(
        self,
        business_connection_id: str,
        star_count: int,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Transfers Telegram Stars from the business account balance to the bot's balance. Requires the *can_transfer_stars* business bot right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#transferbusinessaccountstars

        :param business_connection_id: Unique identifier of the business connection
        :param star_count: Number of Telegram Stars to transfer; 1-10000
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = TransferBusinessAccountStars(
            business_connection_id=business_connection_id,
            star_count=star_count,
        )
        return await self(call, request_timeout=request_timeout)

    async def transfer_gift(
        self,
        business_connection_id: str,
        owned_gift_id: str,
        new_owner_chat_id: int,
        star_count: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Transfers an owned unique gift to another user. Requires the *can_transfer_and_upgrade_gifts* business bot right. Requires *can_transfer_stars* business bot right if the transfer is paid. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#transfergift

        :param business_connection_id: Unique identifier of the business connection
        :param owned_gift_id: Unique identifier of the regular gift that should be transferred
        :param new_owner_chat_id: Unique identifier of the chat which will own the gift. The chat must be active in the last 24 hours.
        :param star_count: The amount of Telegram Stars that will be paid for the transfer from the business account balance. If positive, then the *can_transfer_stars* business bot right is required.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = TransferGift(
            business_connection_id=business_connection_id,
            owned_gift_id=owned_gift_id,
            new_owner_chat_id=new_owner_chat_id,
            star_count=star_count,
        )
        return await self(call, request_timeout=request_timeout)

    async def upgrade_gift(
        self,
        business_connection_id: str,
        owned_gift_id: str,
        keep_original_details: bool | None = None,
        star_count: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Upgrades a given regular gift to a unique gift. Requires the *can_transfer_and_upgrade_gifts* business bot right. Additionally requires the *can_transfer_stars* business bot right if the upgrade is paid. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#upgradegift

        :param business_connection_id: Unique identifier of the business connection
        :param owned_gift_id: Unique identifier of the regular gift that should be upgraded to a unique one
        :param keep_original_details: Pass :code:`True` to keep the original gift text, sender and receiver in the upgraded gift
        :param star_count: The amount of Telegram Stars that will be paid for the upgrade from the business account balance. If :code:`gift.prepaid_upgrade_star_count > 0`, then pass 0, otherwise, the *can_transfer_stars* business bot right is required and :code:`gift.upgrade_star_count` must be passed.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = UpgradeGift(
            business_connection_id=business_connection_id,
            owned_gift_id=owned_gift_id,
            keep_original_details=keep_original_details,
            star_count=star_count,
        )
        return await self(call, request_timeout=request_timeout)

    async def edit_message_checklist(
        self,
        business_connection_id: str,
        chat_id: ChatIdUnion,
        message_id: int,
        checklist: InputChecklist,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to edit a checklist on behalf of a connected business account. On success, the edited :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#editmessagechecklist

        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param chat_id: Unique identifier for the target chat or username of the target bot in the format :code:`@username`
        :param message_id: Unique identifier for the target message
        :param checklist: A JSON-serialized object for the new checklist
        :param reply_markup: A JSON-serialized object for the new `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_ for the message
        :param request_timeout: Request timeout
        :return: On success, the edited :class:`aiogram.types.message.Message` is returned.
        """

        call = EditMessageChecklist(
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            message_id=message_id,
            checklist=checklist,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_my_star_balance(
        self,
        request_timeout: int | None = None,
    ) -> StarAmount:
        """
        A method to get the current Telegram Stars balance of the bot. Requires no parameters. On success, returns a :class:`aiogram.types.star_amount.StarAmount` object.

        Source: https://core.telegram.org/bots/api#getmystarbalance

        :param request_timeout: Request timeout
        :return: On success, returns a :class:`aiogram.types.star_amount.StarAmount` object.
        """

        call = GetMyStarBalance()
        return await self(call, request_timeout=request_timeout)

    async def send_checklist(
        self,
        business_connection_id: str,
        chat_id: ChatIdUnion,
        checklist: InputChecklist,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        message_effect_id: str | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: InlineKeyboardMarkup | None = None,
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send a checklist on behalf of a connected business account. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendchecklist

        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param chat_id: Unique identifier for the target chat or username of the target bot in the format :code:`@username`
        :param checklist: A JSON-serialized object for the checklist to send
        :param disable_notification: Sends the message silently. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param message_effect_id: Unique identifier of the message effect to be added to the message
        :param reply_parameters: A JSON-serialized object for description of the message to reply to
        :param reply_markup: A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendChecklist(
            business_connection_id=business_connection_id,
            chat_id=chat_id,
            checklist=checklist,
            disable_notification=disable_notification,
            protect_content=protect_content,
            message_effect_id=message_effect_id,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def approve_suggested_post(
        self,
        chat_id: int,
        message_id: int,
        send_date: DateTimeUnion | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to approve a suggested post in a direct messages chat. The bot must have the 'can_post_messages' administrator right in the corresponding channel chat. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#approvesuggestedpost

        :param chat_id: Unique identifier for the target direct messages chat
        :param message_id: Identifier of a suggested post message to approve
        :param send_date: Point in time (Unix timestamp) when the post is expected to be published; omit if the date has already been specified when the suggested post was created. If specified, then the date must be not more than 2678400 seconds (30 days) in the future.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = ApproveSuggestedPost(
            chat_id=chat_id,
            message_id=message_id,
            send_date=send_date,
        )
        return await self(call, request_timeout=request_timeout)

    async def decline_suggested_post(
        self,
        chat_id: int,
        message_id: int,
        comment: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to decline a suggested post in a direct messages chat. The bot must have the 'can_manage_direct_messages' administrator right in the corresponding channel chat. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#declinesuggestedpost

        :param chat_id: Unique identifier for the target direct messages chat
        :param message_id: Identifier of a suggested post message to decline
        :param comment: Comment for the creator of the suggested post; 0-128 characters
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeclineSuggestedPost(
            chat_id=chat_id,
            message_id=message_id,
            comment=comment,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_chat_gifts(
        self,
        chat_id: ChatIdUnion,
        exclude_unsaved: bool | None = None,
        exclude_saved: bool | None = None,
        exclude_unlimited: bool | None = None,
        exclude_limited_upgradable: bool | None = None,
        exclude_limited_non_upgradable: bool | None = None,
        exclude_from_blockchain: bool | None = None,
        exclude_unique: bool | None = None,
        sort_by_price: bool | None = None,
        offset: str | None = None,
        limit: int | None = None,
        request_timeout: int | None = None,
    ) -> OwnedGifts:
        """
        Returns the gifts owned by a chat. Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.

        Source: https://core.telegram.org/bots/api#getchatgifts

        :param chat_id: Unique identifier for the target chat or username of the target channel in the format :code:`@username`
        :param exclude_unsaved: Pass :code:`True` to exclude gifts that aren't saved to the chat's profile page. Always :code:`True`, unless the bot has the *can_post_messages* administrator right in the channel.
        :param exclude_saved: Pass :code:`True` to exclude gifts that are saved to the chat's profile page. Always :code:`False`, unless the bot has the *can_post_messages* administrator right in the channel.
        :param exclude_unlimited: Pass :code:`True` to exclude gifts that can be purchased an unlimited number of times
        :param exclude_limited_upgradable: Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can be upgraded to unique
        :param exclude_limited_non_upgradable: Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can't be upgraded to unique
        :param exclude_from_blockchain: Pass :code:`True` to exclude gifts that were assigned from the TON blockchain and can't be resold or transferred in Telegram
        :param exclude_unique: Pass :code:`True` to exclude unique gifts
        :param sort_by_price: Pass :code:`True` to sort results by gift price instead of send date. Sorting is applied before pagination.
        :param offset: Offset of the first entry to return as received from the previous request; use an empty string to get the first chunk of results
        :param limit: The maximum number of gifts to be returned; 1-100. Defaults to 100.
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.
        """

        call = GetChatGifts(
            chat_id=chat_id,
            exclude_unsaved=exclude_unsaved,
            exclude_saved=exclude_saved,
            exclude_unlimited=exclude_unlimited,
            exclude_limited_upgradable=exclude_limited_upgradable,
            exclude_limited_non_upgradable=exclude_limited_non_upgradable,
            exclude_from_blockchain=exclude_from_blockchain,
            exclude_unique=exclude_unique,
            sort_by_price=sort_by_price,
            offset=offset,
            limit=limit,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_user_gifts(
        self,
        user_id: int,
        exclude_unlimited: bool | None = None,
        exclude_limited_upgradable: bool | None = None,
        exclude_limited_non_upgradable: bool | None = None,
        exclude_from_blockchain: bool | None = None,
        exclude_unique: bool | None = None,
        sort_by_price: bool | None = None,
        offset: str | None = None,
        limit: int | None = None,
        request_timeout: int | None = None,
    ) -> OwnedGifts:
        """
        Returns the gifts owned and hosted by a user. Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.

        Source: https://core.telegram.org/bots/api#getusergifts

        :param user_id: Unique identifier of the user
        :param exclude_unlimited: Pass :code:`True` to exclude gifts that can be purchased an unlimited number of times
        :param exclude_limited_upgradable: Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can be upgraded to unique
        :param exclude_limited_non_upgradable: Pass :code:`True` to exclude gifts that can be purchased a limited number of times and can't be upgraded to unique
        :param exclude_from_blockchain: Pass :code:`True` to exclude gifts that were assigned from the TON blockchain and can't be resold or transferred in Telegram
        :param exclude_unique: Pass :code:`True` to exclude unique gifts
        :param sort_by_price: Pass :code:`True` to sort results by gift price instead of send date. Sorting is applied before pagination.
        :param offset: Offset of the first entry to return as received from the previous request; use an empty string to get the first chunk of results
        :param limit: The maximum number of gifts to be returned; 1-100. Defaults to 100.
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.owned_gifts.OwnedGifts` on success.
        """

        call = GetUserGifts(
            user_id=user_id,
            exclude_unlimited=exclude_unlimited,
            exclude_limited_upgradable=exclude_limited_upgradable,
            exclude_limited_non_upgradable=exclude_limited_non_upgradable,
            exclude_from_blockchain=exclude_from_blockchain,
            exclude_unique=exclude_unique,
            sort_by_price=sort_by_price,
            offset=offset,
            limit=limit,
        )
        return await self(call, request_timeout=request_timeout)

    async def repost_story(
        self,
        business_connection_id: str,
        from_chat_id: int,
        from_story_id: int,
        active_period: int,
        post_to_chat_page: bool | None = None,
        protect_content: bool | None = None,
        request_timeout: int | None = None,
    ) -> Story:
        """
        Reposts a story on behalf of a business account from another business account. Both business accounts must be managed by the same bot, and the story on the source account must have been posted (or reposted) by the bot. Requires the *can_manage_stories* business bot right for both business accounts. Returns :class:`aiogram.types.story.Story` on success.

        Source: https://core.telegram.org/bots/api#repoststory

        :param business_connection_id: Unique identifier of the business connection
        :param from_chat_id: Unique identifier of the chat which posted the story that should be reposted
        :param from_story_id: Unique identifier of the story that should be reposted
        :param active_period: Period after which the story is moved to the archive, in seconds; must be one of :code:`6 * 3600`, :code:`12 * 3600`, :code:`86400`, or :code:`2 * 86400`
        :param post_to_chat_page: Pass :code:`True` to keep the story accessible after it expires
        :param protect_content: Pass :code:`True` if the content of the story must be protected from forwarding and screenshotting
        :param request_timeout: Request timeout
        :return: Returns :class:`aiogram.types.story.Story` on success.
        """

        call = RepostStory(
            business_connection_id=business_connection_id,
            from_chat_id=from_chat_id,
            from_story_id=from_story_id,
            active_period=active_period,
            post_to_chat_page=post_to_chat_page,
            protect_content=protect_content,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_message_draft(
        self,
        chat_id: int,
        draft_id: int,
        message_thread_id: int | None = None,
        text: str | None = None,
        parse_mode: str | None = None,
        entities: list[MessageEntity] | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to stream a partial message to a user while the message is being generated. Note that the streamed draft is ephemeral and acts as a temporary 30-second preview - once the output is finalized, you **must** call :class:`aiogram.methods.send_message.SendMessage` with the complete message to persist it in the user's chat. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#sendmessagedraft

        :param chat_id: Unique identifier for the target private chat
        :param draft_id: Unique identifier of the message draft; must be non-zero. Changes to drafts with the same identifier are animated.
        :param message_thread_id: Unique identifier for the target message thread
        :param text: Text of the message to be sent, 0-4096 characters after entities parsing. Pass an empty text to show a 'Thinking…' placeholder.
        :param parse_mode: Mode for parsing entities in the message text. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param entities: A JSON-serialized list of special entities that appear in message text, which can be specified instead of *parse_mode*
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SendMessageDraft(
            chat_id=chat_id,
            draft_id=draft_id,
            message_thread_id=message_thread_id,
            text=text,
            parse_mode=parse_mode,
            entities=entities,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_user_profile_audios(
        self,
        user_id: int,
        offset: int | None = None,
        limit: int | None = None,
        request_timeout: int | None = None,
    ) -> UserProfileAudios:
        """
        Use this method to get a list of profile audios for a user. Returns a :class:`aiogram.types.user_profile_audios.UserProfileAudios` object.

        Source: https://core.telegram.org/bots/api#getuserprofileaudios

        :param user_id: Unique identifier of the target user
        :param offset: Sequential number of the first audio to be returned. By default, all audios are returned.
        :param limit: Limits the number of audios to be retrieved. Values between 1-100 are accepted. Defaults to 100.
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.user_profile_audios.UserProfileAudios` object.
        """

        call = GetUserProfileAudios(
            user_id=user_id,
            offset=offset,
            limit=limit,
        )
        return await self(call, request_timeout=request_timeout)

    async def remove_my_profile_photo(
        self,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Removes the profile photo of the bot. Requires no parameters. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#removemyprofilephoto

        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = RemoveMyProfilePhoto()
        return await self(call, request_timeout=request_timeout)

    async def set_my_profile_photo(
        self,
        photo: InputProfilePhotoUnion,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Changes the profile photo of the bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmyprofilephoto

        :param photo: The new profile photo to set
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetMyProfilePhoto(
            photo=photo,
        )
        return await self(call, request_timeout=request_timeout)

    async def set_chat_member_tag(
        self,
        chat_id: ChatIdUnion,
        user_id: int,
        tag: str | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to set a tag for a regular member in a group or a supergroup. The bot must be an administrator in the chat for this to work and must have the *can_manage_tags* administrator right. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setchatmembertag

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param user_id: Unique identifier of the target user
        :param tag: New tag for the member; 0-16 characters, emoji are not allowed
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetChatMemberTag(
            chat_id=chat_id,
            user_id=user_id,
            tag=tag,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_managed_bot_token(
        self,
        user_id: int,
        request_timeout: int | None = None,
    ) -> str:
        """
        Use this method to get the token of a managed bot. Returns the token as *String* on success.

        Source: https://core.telegram.org/bots/api#getmanagedbottoken

        :param user_id: User identifier of the managed bot whose token will be returned
        :param request_timeout: Request timeout
        :return: Returns the token as *String* on success.
        """

        call = GetManagedBotToken(
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def replace_managed_bot_token(
        self,
        user_id: int,
        request_timeout: int | None = None,
    ) -> str:
        """
        Use this method to revoke the current token of a managed bot and generate a new one. Returns the new token as *String* on success.

        Source: https://core.telegram.org/bots/api#replacemanagedbottoken

        :param user_id: User identifier of the managed bot whose token will be replaced
        :param request_timeout: Request timeout
        :return: Returns the new token as *String* on success.
        """

        call = ReplaceManagedBotToken(
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def save_prepared_keyboard_button(
        self,
        user_id: int,
        button: KeyboardButton,
        request_timeout: int | None = None,
    ) -> PreparedKeyboardButton:
        """
        Stores a keyboard button that can be used by a user within a Mini App. Returns a :class:`aiogram.types.prepared_keyboard_button.PreparedKeyboardButton` object.

        Source: https://core.telegram.org/bots/api#savepreparedkeyboardbutton

        :param user_id: Unique identifier of the target user that can use the button
        :param button: A JSON-serialized object describing the button to be saved. The button must be of the type *request_users*, *request_chat*, or *request_managed_bot*.
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.prepared_keyboard_button.PreparedKeyboardButton` object.
        """

        call = SavePreparedKeyboardButton(
            user_id=user_id,
            button=button,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_guest_query(
        self,
        guest_query_id: str,
        result: InlineQueryResultUnion,
        request_timeout: int | None = None,
    ) -> SentGuestMessage:
        """
        Use this method to reply to a received guest message. On success, a :class:`aiogram.types.sent_guest_message.SentGuestMessage` object is returned.

        Source: https://core.telegram.org/bots/api#answerguestquery

        :param guest_query_id: Unique identifier for the query to be answered
        :param result: A JSON-serialized object describing the message to be sent
        :param request_timeout: Request timeout
        :return: On success, a :class:`aiogram.types.sent_guest_message.SentGuestMessage` object is returned.
        """

        call = AnswerGuestQuery(
            guest_query_id=guest_query_id,
            result=result,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_all_message_reactions(
        self,
        chat_id: ChatIdUnion,
        user_id: int | None = None,
        actor_chat_id: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to remove up to 10000 recent reactions in a group or a supergroup chat added by a given user or chat. The bot must have the 'can_delete_messages' administrator right in the chat. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deleteallmessagereactions

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param user_id: Identifier of the user whose reactions will be removed, if the reactions were added by a user
        :param actor_chat_id: Identifier of the chat whose reactions will be removed, if the reactions were added by a chat
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteAllMessageReactions(
            chat_id=chat_id,
            user_id=user_id,
            actor_chat_id=actor_chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def delete_message_reaction(
        self,
        chat_id: ChatIdUnion,
        message_id: int,
        user_id: int | None = None,
        actor_chat_id: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to remove a reaction from a message in a group or a supergroup chat. The bot must have the 'can_delete_messages' administrator right in the chat. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#deletemessagereaction

        :param chat_id: Unique identifier for the target chat or username of the target supergroup in the format :code:`@username`
        :param message_id: Identifier of the target message
        :param user_id: Identifier of the user whose reaction will be removed, if the reaction was added by a user
        :param actor_chat_id: Identifier of the chat whose reaction will be removed, if the reaction was added by a chat
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = DeleteMessageReaction(
            chat_id=chat_id,
            message_id=message_id,
            user_id=user_id,
            actor_chat_id=actor_chat_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_managed_bot_access_settings(
        self,
        user_id: int,
        request_timeout: int | None = None,
    ) -> BotAccessSettings:
        """
        Use this method to get the access settings of a managed bot. Returns a :class:`aiogram.types.bot_access_settings.BotAccessSettings` object on success.

        Source: https://core.telegram.org/bots/api#getmanagedbotaccesssettings

        :param user_id: User identifier of the managed bot whose access settings will be returned
        :param request_timeout: Request timeout
        :return: Returns a :class:`aiogram.types.bot_access_settings.BotAccessSettings` object on success.
        """

        call = GetManagedBotAccessSettings(
            user_id=user_id,
        )
        return await self(call, request_timeout=request_timeout)

    async def get_user_personal_chat_messages(
        self,
        user_id: int,
        limit: int,
        request_timeout: int | None = None,
    ) -> list[Message]:
        """
        Use this method to get the last messages from the personal chat (i.e., the chat currently added to their profile) of a given user. On success, an array of :class:`aiogram.types.message.Message` objects is returned.

        Source: https://core.telegram.org/bots/api#getuserpersonalchatmessages

        :param user_id: Unique identifier for the target user
        :param limit: The maximum number of messages to return; 1-20
        :param request_timeout: Request timeout
        :return: On success, an array of :class:`aiogram.types.message.Message` objects is returned.
        """

        call = GetUserPersonalChatMessages(
            user_id=user_id,
            limit=limit,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_live_photo(
        self,
        chat_id: ChatIdUnion,
        live_photo: InputFileUnion,
        photo: InputFileUnion,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        caption: str | None = None,
        parse_mode: str | None = None,
        caption_entities: list[MessageEntity] | None = None,
        show_caption_above_media: bool | None = None,
        has_spoiler: bool | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send live photos. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendlivephoto

        :param chat_id: Unique identifier for the target chat or username of the target channel (in the format :code:`@channelusername`)
        :param live_photo: Live photo video to send. The video must be no longer than 10 seconds and must not exceed 10 MB in size. Pass a file_id as String to send a video that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported.
        :param photo: The static photo to send. Pass a file_id as String to send a photo that exists on the Telegram servers (recommended) or upload a new video using multipart/form-data. :ref:`More information on Sending Files » <sending-files>`. Sending live photos by a URL is currently unsupported.
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param caption: Video caption (may also be used when resending videos by *file_id*), 0-1024 characters after entities parsing
        :param parse_mode: Mode for parsing entities in the video caption. See `formatting options <https://core.telegram.org/bots/api#formatting-options>`_ for more details.
        :param caption_entities: A JSON-serialized list of special entities that appear in the caption, which can be specified instead of *parse_mode*
        :param show_caption_above_media: Pass :code:`True`, if the caption must be shown above the message media
        :param has_spoiler: Pass :code:`True` if the video needs to be covered with a spoiler animation
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendLivePhoto(
            chat_id=chat_id,
            live_photo=live_photo,
            photo=photo,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
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
        )
        return await self(call, request_timeout=request_timeout)

    async def set_managed_bot_access_settings(
        self,
        user_id: int,
        is_access_restricted: bool,
        added_user_ids: list[int] | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to change the access settings of a managed bot. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#setmanagedbotaccesssettings

        :param user_id: User identifier of the managed bot whose access settings will be changed
        :param is_access_restricted: Pass :code:`True`, if only selected users can access the bot. The bot's owner can always access it.
        :param added_user_ids: A JSON-serialized list of up to 10 identifiers of users who will have access to the bot in addition to its owner. Ignored if *is_access_restricted* is false.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SetManagedBotAccessSettings(
            user_id=user_id,
            is_access_restricted=is_access_restricted,
            added_user_ids=added_user_ids,
        )
        return await self(call, request_timeout=request_timeout)

    async def answer_chat_join_request_query(
        self,
        chat_join_request_query_id: str,
        result: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to process a received chat join request query. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#answerchatjoinrequestquery

        :param chat_join_request_query_id: Unique identifier of the join request query
        :param result: Result of the query. Must be either 'approve' to allow the user to join the chat, 'decline' to disallow the user to join the chat, or 'queue' to leave the decision to other administrators.
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = AnswerChatJoinRequestQuery(
            chat_join_request_query_id=chat_join_request_query_id,
            result=result,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_chat_join_request_web_app(
        self,
        chat_join_request_query_id: str,
        web_app_url: str,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to process a received chat join request query by showing a Mini App to the user before deciding the outcome. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#sendchatjoinrequestwebapp

        :param chat_join_request_query_id: Unique identifier of the join request query
        :param web_app_url: The URL of the Mini App to be opened
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SendChatJoinRequestWebApp(
            chat_join_request_query_id=chat_join_request_query_id,
            web_app_url=web_app_url,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_rich_message(
        self,
        chat_id: ChatIdUnion,
        rich_message: InputRichMessage,
        business_connection_id: str | None = None,
        message_thread_id: int | None = None,
        direct_messages_topic_id: int | None = None,
        disable_notification: bool | None = None,
        protect_content: bool | None = None,
        allow_paid_broadcast: bool | None = None,
        message_effect_id: str | None = None,
        suggested_post_parameters: SuggestedPostParameters | None = None,
        reply_parameters: ReplyParameters | None = None,
        reply_markup: ReplyMarkupUnion | None = None,
        request_timeout: int | None = None,
    ) -> Message:
        """
        Use this method to send rich messages. If the message contains a block with a media element, then the bot must have the right to send the media to the chat. On success, the sent :class:`aiogram.types.message.Message` is returned.

        Source: https://core.telegram.org/bots/api#sendrichmessage

        :param chat_id: Unique identifier for the target chat or username of the target bot, supergroup or channel in the format :code:`@username`
        :param rich_message: The message to be sent
        :param business_connection_id: Unique identifier of the business connection on behalf of which the message will be sent
        :param message_thread_id: Unique identifier for the target message thread (topic) of a forum; for forum supergroups and private chats of bots with forum topic mode enabled only
        :param direct_messages_topic_id: Identifier of the direct messages topic to which the message will be sent; required if the message is sent to a direct messages chat
        :param disable_notification: Sends the message `silently <https://telegram.org/blog/channels-2-0#silent-messages>`_. Users will receive a notification with no sound.
        :param protect_content: Protects the contents of the sent message from forwarding and saving
        :param allow_paid_broadcast: Pass :code:`True` to allow up to 1000 messages per second, ignoring `broadcasting limits <https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once>`_ for a fee of 0.1 Telegram Stars per message. The relevant Stars will be withdrawn from the bot's balance.
        :param message_effect_id: Unique identifier of the message effect to be added to the message; for private chats only
        :param suggested_post_parameters: A JSON-serialized object containing the parameters of the suggested post to send; for direct messages chats only. If the message is sent as a reply to another suggested post, then that suggested post is automatically declined.
        :param reply_parameters: Description of the message to reply to
        :param reply_markup: Additional interface options. A JSON-serialized object for an `inline keyboard <https://core.telegram.org/bots/features#inline-keyboards>`_, `custom reply keyboard <https://core.telegram.org/bots/features#keyboards>`_, instructions to remove a reply keyboard or to force a reply from the user.
        :param request_timeout: Request timeout
        :return: On success, the sent :class:`aiogram.types.message.Message` is returned.
        """

        call = SendRichMessage(
            chat_id=chat_id,
            rich_message=rich_message,
            business_connection_id=business_connection_id,
            message_thread_id=message_thread_id,
            direct_messages_topic_id=direct_messages_topic_id,
            disable_notification=disable_notification,
            protect_content=protect_content,
            allow_paid_broadcast=allow_paid_broadcast,
            message_effect_id=message_effect_id,
            suggested_post_parameters=suggested_post_parameters,
            reply_parameters=reply_parameters,
            reply_markup=reply_markup,
        )
        return await self(call, request_timeout=request_timeout)

    async def send_rich_message_draft(
        self,
        chat_id: int,
        draft_id: int,
        rich_message: InputRichMessage,
        message_thread_id: int | None = None,
        request_timeout: int | None = None,
    ) -> bool:
        """
        Use this method to stream a partial rich message to a user while the message is being generated. Note that the streamed draft is ephemeral and acts as a temporary 30-second preview - once the output is finalized, you **must** call :class:`aiogram.methods.send_rich_message.SendRichMessage` with the complete message to persist it in the user's chat. Returns :code:`True` on success.

        Source: https://core.telegram.org/bots/api#sendrichmessagedraft

        :param chat_id: Unique identifier for the target private chat
        :param draft_id: Unique identifier of the message draft; must be non-zero. Changes to drafts with the same identifier are animated.
        :param rich_message: The partial message to be streamed
        :param message_thread_id: Unique identifier for the target message thread
        :param request_timeout: Request timeout
        :return: Returns :code:`True` on success.
        """

        call = SendRichMessageDraft(
            chat_id=chat_id,
            draft_id=draft_id,
            rich_message=rich_message,
            message_thread_id=message_thread_id,
        )
        return await self(call, request_timeout=request_timeout)
