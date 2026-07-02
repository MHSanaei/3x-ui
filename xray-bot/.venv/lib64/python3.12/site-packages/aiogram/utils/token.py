from functools import lru_cache


class TokenValidationError(Exception):
    pass


@lru_cache
def validate_token(token: str) -> bool:
    """
    Validate Telegram token

    :param token:
    :return:
    """
    if not isinstance(token, str):
        msg = f"Token is invalid! It must be 'str' type instead of {type(token)} type."
        raise TokenValidationError(msg)

    if any(x.isspace() for x in token):
        message = "Token is invalid! It can't contains spaces."
        raise TokenValidationError(message)

    left, sep, right = token.partition(":")
    if (not sep) or (not left.isdigit()) or (not right):
        msg = "Token is invalid!"
        raise TokenValidationError(msg)

    return True


@lru_cache
def extract_bot_id(token: str) -> int:
    """
    Extract bot ID from Telegram token

    :param token:
    :return:
    """
    validate_token(token)
    raw_bot_id, *_ = token.split(":")
    return int(raw_bot_id)
