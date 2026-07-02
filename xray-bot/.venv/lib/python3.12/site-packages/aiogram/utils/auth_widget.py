import hashlib
import hmac
from typing import Any


def check_signature(token: str, hash: str, **kwargs: Any) -> bool:
    """
    Generate hexadecimal representation
    of the HMAC-SHA-256 signature of the data-check-string
    with the SHA256 hash of the bot's token used as a secret key

    :param token:
    :param hash:
    :param kwargs: all params received on auth
    :return:
    """
    secret = hashlib.sha256(token.encode("utf-8"))
    check_string = "\n".join(f"{k}={kwargs[k]}" for k in sorted(kwargs))
    hmac_string = hmac.new(
        secret.digest(),
        check_string.encode("utf-8"),
        digestmod=hashlib.sha256,
    ).hexdigest()
    return hmac_string == hash


def check_integrity(token: str, data: dict[str, Any]) -> bool:
    """
    Verify the authentication and the integrity
    of the data received on user's auth

    :param token: Bot's token
    :param data: all data that came on auth
    :return:
    """
    return check_signature(token, **data)
