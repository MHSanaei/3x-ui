import base64
from operator import itemgetter
from urllib.parse import parse_qsl

from cryptography.exceptions import InvalidSignature
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PublicKey

from .web_app import WebAppInitData, parse_webapp_init_data

PRODUCTION_PUBLIC_KEY = bytes.fromhex(
    "e7bf03a2fa4602af4580703d88dda5bb59f32ed8b02a56c187fe7d34caed242d",
)
TEST_PUBLIC_KEY = bytes.fromhex("40055058a4ee38156a06562e52eece92a771bcd8346a8c4615cb7376eddf72ec")


def check_webapp_signature(
    bot_id: int,
    init_data: str,
    public_key_bytes: bytes = PRODUCTION_PUBLIC_KEY,
) -> bool:
    """
    Check incoming WebApp init data signature without bot token using only bot id.

    Source: https://core.telegram.org/bots/webapps#validating-data-for-third-party-use

    :param bot_id: Bot ID
    :param init_data: WebApp init data
    :param public_key: Public key
    :return: True if signature is valid, False otherwise
    """
    try:
        parsed_data = dict(parse_qsl(init_data, strict_parsing=True))
    except ValueError:
        return False

    signature_b64 = parsed_data.pop("signature", None)
    if not signature_b64:
        return False

    parsed_data.pop("hash", None)

    data_check_string = f"{bot_id}:WebAppData\n" + "\n".join(
        f"{k}={v}" for k, v in sorted(parsed_data.items(), key=itemgetter(0))
    )
    message = data_check_string.encode()

    padding = "=" * (-len(signature_b64) % 4)
    signature = base64.urlsafe_b64decode(signature_b64 + padding)

    public_key = Ed25519PublicKey.from_public_bytes(public_key_bytes)

    try:
        public_key.verify(signature, message)
    except InvalidSignature:
        return False
    else:
        return True


def safe_check_webapp_init_data_from_signature(
    bot_id: int,
    init_data: str,
    public_key_bytes: bytes = PRODUCTION_PUBLIC_KEY,
) -> WebAppInitData:
    """
    Validate raw WebApp init data using only bot id and return it as WebAppInitData object

    :param bot_id: bot id
    :param init_data: data from frontend to be parsed and validated
    :param public_key_bytes: public key
    :return: WebAppInitData object
    """
    if check_webapp_signature(bot_id, init_data, public_key_bytes):
        return parse_webapp_init_data(init_data)
    msg = "Invalid init data signature"
    raise ValueError(msg)
