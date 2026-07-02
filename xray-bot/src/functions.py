import aiohttp
import uuid
import json
import logging
import random
from datetime import datetime, timedelta
from typing import Optional
from config import config
from urllib.parse import urlparse

logger = logging.getLogger(__name__)


def _parse(value) -> dict:
    """Return value as dict regardless of whether it arrived as str or dict."""
    if isinstance(value, dict):
        return value
    if isinstance(value, (str, bytes, bytearray)):
        return json.loads(value)
    return {}


class XUIAPI:
    def __init__(self):
        self.session: Optional[aiohttp.ClientSession] = None

    def _get_headers(self) -> dict:
        return {
            "Authorization": f"Bearer {config.XUI_API_TOKEN}",
            "Content-Type": "application/json",
        }

    def _base_url(self) -> str:
        base = config.XUI_API_URL.rstrip("/")
        path = config.XUI_BASE_PATH.strip("/")
        return f"{base}/{path}" if path else base

    async def _get_session(self) -> aiohttp.ClientSession:
        if self.session is None or self.session.closed:
            connector = aiohttp.TCPConnector(ssl=config.XUI_VERIFY_SSL)
            self.session = aiohttp.ClientSession(connector=connector)
        return self.session

    async def close(self):
        if self.session and not self.session.closed:
            await self.session.close()

    # ------------------------------------------------------------------ #
    #  Inbound helpers                                                     #
    # ------------------------------------------------------------------ #

    async def get_inbound(self, inbound_id: int) -> Optional[dict]:
        url = f"{self._base_url()}/api/inbounds/get/{inbound_id}"
        logger.info(f"ℹ️  GET inbound: {url}")
        session = await self._get_session()
        try:
            async with session.get(url, headers=self._get_headers()) as resp:
                if resp.status != 200:
                    text = await resp.text()
                    logger.error(f"🛑 get_inbound failed: status={resp.status}, body={text[:200]}")
                    return None
                data = await resp.json()
                if data.get("success"):
                    return data.get("obj")
                logger.error(f"🛑 get_inbound API error: {data.get('msg')}")
                return None
        except Exception as e:
            logger.exception(f"🛑 get_inbound exception: {e}")
            return None

    async def update_inbound(self, inbound_id: int, data: dict) -> bool:
        url = f"{self._base_url()}/api/inbounds/update/{inbound_id}"
        logger.info(f"ℹ️  POST update inbound: {url}")
        session = await self._get_session()
        try:
            async with session.post(url, headers=self._get_headers(), json=data) as resp:
                if resp.status != 200:
                    text = await resp.text()
                    logger.error(f"🛑 update_inbound failed: status={resp.status}, body={text[:200]}")
                    return False
                response = await resp.json()
                success = response.get("success", False)
                if not success:
                    logger.error(f"🛑 update_inbound API error: {response.get('msg')}")
                return success
        except Exception as e:
            logger.exception(f"🛑 update_inbound exception: {e}")
            return False

    # ------------------------------------------------------------------ #
    #  Internal utilities                                                  #
    # ------------------------------------------------------------------ #

    async def _get_flow_from_inbound(self, inbound: dict) -> str:
        try:
            settings = _parse(inbound.get("settings", {}))
            stream_settings = _parse(inbound.get("streamSettings", {}))
            reality_settings = stream_settings.get("realitySettings", {})

            clients = settings.get("clients", [])
            if clients:
                flow = clients[0].get("flow", "")
                if flow:
                    return flow

            if reality_settings:
                return reality_settings.get("flow", "")
        except Exception as e:
            logger.warning(f"⚠️ _get_flow_from_inbound: {e}")
        return ""

    def _build_update_payload(self, inbound: dict, settings: dict) -> dict:
        """Assembles the full update payload from the current inbound + modified settings."""
        payload = {
            "up": inbound["up"],
            "down": inbound["down"],
            "total": inbound["total"],
            "remark": inbound["remark"],
            "enable": inbound["enable"],
            "expiryTime": inbound["expiryTime"],
            "listen": inbound["listen"],
            "port": inbound["port"],
            "protocol": inbound["protocol"],
            "settings": json.dumps(settings, indent=2),
            "streamSettings": inbound["streamSettings"],
            "sniffing": inbound["sniffing"],
        }
        if "allocate" in inbound:
            payload["allocate"] = inbound["allocate"]
        return payload

    # ------------------------------------------------------------------ #
    #  Client management                                                   #
    # ------------------------------------------------------------------ #

    async def create_vless_profile(self, telegram_id: int, expiry_time: int = 0) -> Optional[dict]:
        logger.info(f"🔍 create_vless_profile: user={telegram_id}, expiry={expiry_time}")

        inbound = await self.get_inbound(config.INBOUND_ID)
        if not inbound:
            logger.error(f"🛑 Inbound {config.INBOUND_ID} not found")
            return None

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])

            client_id = str(uuid.uuid4())
            email = f"user_{telegram_id}_{random.randint(1000, 9999)}"
            flow = await self._get_flow_from_inbound(inbound)
            sub_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, f"user_{telegram_id}"))

            expiry_ms = _safe_expiry_ms(expiry_time)

            new_client = {
                "id": client_id,
                "flow": flow,
                "email": email,
                "limitIp": 0,
                "totalGB": 0,
                "expiryTime": expiry_ms,
                "enable": True,
                "tgId": "",
                "subId": sub_id,
                "reset": 0,
                "fingerprint": config.REALITY_FINGERPRINT,
                "publicKey": config.REALITY_PUBLIC_KEY,
                "shortId": config.REALITY_SHORT_ID,
                "spiderX": config.REALITY_SPIDER_X,
            }

            clients.append(new_client)
            settings["clients"] = clients

            if await self.update_inbound(config.INBOUND_ID, self._build_update_payload(inbound, settings)):
                return _profile_result(client_id, email, inbound, sub_id, config)
            return None
        except Exception as e:
            logger.exception(f"🛑 create_vless_profile error: {e}")
            return None

    async def create_static_client(self, profile_name: str) -> Optional[dict]:
        inbound = await self.get_inbound(config.INBOUND_ID)
        if not inbound:
            return None

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])

            client_id = str(uuid.uuid4())
            flow = await self._get_flow_from_inbound(inbound)
            sub_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, f"static_{profile_name}"))

            new_client = {
                "id": client_id,
                "flow": flow,
                "email": profile_name,
                "limitIp": 0,
                "totalGB": 0,
                "expiryTime": 0,
                "enable": True,
                "tgId": "",
                "subId": sub_id,
                "reset": 0,
                "fingerprint": config.REALITY_FINGERPRINT,
                "publicKey": config.REALITY_PUBLIC_KEY,
                "shortId": config.REALITY_SHORT_ID,
                "spiderX": config.REALITY_SPIDER_X,
            }

            clients.append(new_client)
            settings["clients"] = clients

            if await self.update_inbound(config.INBOUND_ID, self._build_update_payload(inbound, settings)):
                return _profile_result(client_id, profile_name, inbound, sub_id, config)
            return None
        except Exception as e:
            logger.exception(f"🛑 create_static_client error: {e}")
            return None

    async def delete_client(self, email: str) -> bool:
        inbound = await self.get_inbound(config.INBOUND_ID)
        if not inbound:
            return False

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])
            new_clients = [c for c in clients if c["email"] != email]

            if len(new_clients) == len(clients):
                logger.warning(f"⚠️ Client {email} not found")
                return False

            settings["clients"] = new_clients
            return await self.update_inbound(config.INBOUND_ID, self._build_update_payload(inbound, settings))
        except Exception as e:
            logger.exception(f"🛑 delete_client error: {e}")
            return False

    async def update_client_expiry(self, email: str, expiry_time: int) -> bool:
        logger.info(f"🔍 update_client_expiry: email={email}, expiry={expiry_time}")

        inbound = await self.get_inbound(config.INBOUND_ID)
        if not inbound:
            return False

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])

            updated = False
            for client in clients:
                if client["email"] == email:
                    client["expiryTime"] = _safe_expiry_ms(expiry_time)
                    updated = True
                    logger.info(f"✅ Set expiryTime={client['expiryTime']} ms for {email}")
                    break

            if not updated:
                logger.warning(f"⚠️ Client {email} not found")
                return False

            settings["clients"] = clients
            return await self.update_inbound(config.INBOUND_ID, self._build_update_payload(inbound, settings))
        except Exception as e:
            logger.exception(f"🛑 update_client_expiry error: {e}")
            return False

    async def get_all_clients(self) -> Optional[list]:
        inbound = await self.get_inbound(config.INBOUND_ID)
        if not inbound:
            return None

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])
            logger.info(f"📋 Retrieved {len(clients)} clients")
            return clients
        except Exception as e:
            logger.exception(f"🛑 get_all_clients error: {e}")
            return None

    # ------------------------------------------------------------------ #
    #  Stats                                                               #
    # ------------------------------------------------------------------ #

    async def get_user_stats(self, email: str) -> dict:
        url = f"{self._base_url()}/api/inbounds/getClientTraffics/{email}"
        session = await self._get_session()
        try:
            async with session.get(url, headers=self._get_headers()) as resp:
                if resp.status != 200:
                    return {"upload": 0, "download": 0}
                data = await resp.json()
                if data.get("success"):
                    obj = data.get("obj", {})
                    if isinstance(obj, dict):
                        return {"upload": obj.get("up", 0), "download": obj.get("down", 0)}
        except Exception as e:
            logger.error(f"🛑 get_user_stats error: {e}")
        return {"upload": 0, "download": 0}

    async def get_global_stats(self, inbound_id: int) -> dict:
        inbound = await self.get_inbound(inbound_id)
        if inbound:
            return {"upload": inbound.get("up", 0), "download": inbound.get("down", 0)}
        return {"upload": 0, "download": 0}

    async def get_online_users(self) -> int:
        url = f"{self._base_url()}/api/inbounds/onlines"
        session = await self._get_session()
        try:
            async with session.post(url, headers=self._get_headers()) as resp:
                if resp.status != 200:
                    return 0
                data = await resp.json()
                if data.get("success"):
                    users = data.get("obj", [])
                    if isinstance(users, list):
                        return sum(1 for u in users if str(u).startswith("user_"))
        except Exception as e:
            logger.error(f"🛑 get_online_users error: {e}")
        return 0

    # ------------------------------------------------------------------ #
    #  Temp profiles                                                       #
    # ------------------------------------------------------------------ #

    async def create_temp_profile(self, session_id: str) -> Optional[dict]:
        logger.info(f"🔍 create_temp_profile: session={session_id}")

        expiry_time = int((datetime.utcnow() + timedelta(minutes=30)).timestamp())

        inbound = await self.get_inbound(config.TEMP_INBOUND_ID)
        if not inbound:
            logger.error(f"🛑 Temp inbound {config.TEMP_INBOUND_ID} not found")
            return None

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])

            client_id = str(uuid.uuid4())
            email = f"temp_{session_id}_{random.randint(1000, 9999)}"
            flow = await self._get_flow_from_inbound(inbound)
            sub_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, f"temp_{session_id}"))

            new_client = {
                "id": client_id,
                "flow": flow,
                "email": email,
                "limitIp": 0,
                "totalGB": 0,
                "expiryTime": expiry_time * 1000,
                "enable": True,
                "tgId": "",
                "subId": sub_id,
                "reset": 0,
                "fingerprint": config.TEMP_REALITY_FINGERPRINT,
                "publicKey": config.TEMP_REALITY_PUBLIC_KEY,
                "shortId": config.TEMP_REALITY_SHORT_ID,
                "spiderX": config.TEMP_REALITY_SPIDER_X,
            }

            clients.append(new_client)
            settings["clients"] = clients

            if await self.update_inbound(config.TEMP_INBOUND_ID, self._build_update_payload(inbound, settings)):
                return {
                    "client_id": client_id,
                    "email": email,
                    "port": inbound["port"],
                    "security": "reality",
                    "remark": inbound["remark"],
                    "sni": config.TEMP_REALITY_SNI,
                    "pbk": config.TEMP_REALITY_PUBLIC_KEY,
                    "fp": config.TEMP_REALITY_FINGERPRINT,
                    "sid": config.TEMP_REALITY_SHORT_ID,
                    "spx": config.TEMP_REALITY_SPIDER_X,
                    "sub_id": sub_id,
                    "expiry_time": expiry_time,
                }
            return None
        except Exception as e:
            logger.exception(f"🛑 create_temp_profile error: {e}")
            return None

    async def delete_temp_profile(self, email: str) -> bool:
        inbound = await self.get_inbound(config.TEMP_INBOUND_ID)
        if not inbound:
            return False

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])
            new_clients = [c for c in clients if c["email"] != email]

            if len(new_clients) == len(clients):
                logger.warning(f"⚠️ Temp profile {email} not found")
                return False

            settings["clients"] = new_clients
            return await self.update_inbound(config.TEMP_INBOUND_ID, self._build_update_payload(inbound, settings))
        except Exception as e:
            logger.exception(f"🛑 delete_temp_profile error: {e}")
            return False

    async def cleanup_expired_temp_profiles(self) -> int:
        logger.info("🧹 Cleaning up expired temp profiles...")

        inbound = await self.get_inbound(config.TEMP_INBOUND_ID)
        if not inbound:
            return 0

        try:
            settings = _parse(inbound["settings"])
            clients = settings.get("clients", [])
            now = datetime.utcnow()

            active = []
            deleted = 0
            for client in clients:
                email = client.get("email", "")
                if not email.startswith("temp_"):
                    active.append(client)
                    continue
                expiry_ms = client.get("expiryTime", 0)
                if expiry_ms and datetime.fromtimestamp(expiry_ms / 1000) <= now:
                    logger.info(f"🗑️ Removing expired temp profile: {email}")
                    deleted += 1
                else:
                    active.append(client)

            if deleted:
                settings["clients"] = active
                result = await self.update_inbound(
                    config.TEMP_INBOUND_ID, self._build_update_payload(inbound, settings)
                )
                if result:
                    logger.info(f"✅ Deleted {deleted} expired temp profiles")
                else:
                    logger.error("🛑 Failed to save after cleanup")
            else:
                logger.info("✅ No expired temp profiles found")

            return deleted
        except Exception as e:
            logger.exception(f"🛑 cleanup_expired_temp_profiles error: {e}")
            return 0


# ------------------------------------------------------------------ #
#  Pure helpers (no I/O)                                              #
# ------------------------------------------------------------------ #

def _safe_expiry_ms(expiry_time: int) -> int:
    """Convert a seconds timestamp to milliseconds, with sanity checks."""
    if expiry_time <= 0:
        return 0
    if expiry_time < 1_577_836_800:   # before 2020-01-01
        logger.error(f"🚨 expiry_time too small ({expiry_time}), zeroing")
        return 0
    if expiry_time > 2_000_000_000:   # after ~2033
        logger.error(f"🚨 expiry_time too large ({expiry_time}), zeroing")
        return 0
    return expiry_time * 1000


def _profile_result(client_id: str, email: str, inbound: dict, sub_id: str, cfg) -> dict:
    return {
        "client_id": client_id,
        "email": email,
        "port": inbound["port"],
        "security": "reality",
        "remark": inbound["remark"],
        "sni": cfg.REALITY_SNI,
        "pbk": cfg.REALITY_PUBLIC_KEY,
        "fp": cfg.REALITY_FINGERPRINT,
        "sid": cfg.REALITY_SHORT_ID,
        "spx": cfg.REALITY_SPIDER_X,
        "sub_id": sub_id,
    }


def get_safe_expiry_timestamp(subscription_end) -> int:
    logger.info(f"🔍 get_safe_expiry_timestamp: {subscription_end!r} ({type(subscription_end).__name__})")

    if subscription_end is None:
        return 0

    if isinstance(subscription_end, str):
        try:
            subscription_end = datetime.fromisoformat(subscription_end)
        except Exception as e:
            logger.error(f"🛑 Cannot parse date string: {e}")
            return 0

    if not isinstance(subscription_end, datetime):
        logger.error(f"🛑 Unexpected type: {type(subscription_end)}")
        return 0

    now = datetime.utcnow()

    if subscription_end < datetime(2020, 1, 1):
        logger.warning(f"⚠️ Date too old: {subscription_end}")
        return 0

    if subscription_end > now + timedelta(days=3650):
        logger.warning(f"⚠️ Date too far in future: {subscription_end}")
        return 0

    if subscription_end <= now:
        return 0

    try:
        ts = int(subscription_end.timestamp())
        if ts < 1_577_836_800:
            logger.warning(f"⚠️ Timestamp too small: {ts}")
            return 0
        logger.info(f"✅ timestamp={ts}")
        return ts
    except Exception as e:
        logger.error(f"🛑 timestamp conversion error: {e}")
        return 0


# ------------------------------------------------------------------ #
#  Public async API (module-level conveniences)                        #
# ------------------------------------------------------------------ #

async def _run(coro):
    api = XUIAPI()
    try:
        return await coro(api)
    finally:
        await api.close()


async def create_vless_profile(telegram_id: int, expiry_time: int = 0):
    return await _run(lambda api: api.create_vless_profile(telegram_id, expiry_time))

async def create_static_client(profile_name: str):
    return await _run(lambda api: api.create_static_client(profile_name))

async def delete_client_by_email(email: str):
    return await _run(lambda api: api.delete_client(email))

async def update_client_expiry(email: str, expiry_time: int):
    return await _run(lambda api: api.update_client_expiry(email, expiry_time))

async def get_global_stats():
    return await _run(lambda api: api.get_global_stats(config.INBOUND_ID))

async def get_online_users():
    return await _run(lambda api: api.get_online_users())

async def get_user_stats(email: str):
    return await _run(lambda api: api.get_user_stats(email))

async def create_temp_profile(session_id: str) -> Optional[dict]:
    return await _run(lambda api: api.create_temp_profile(session_id))

async def delete_temp_profile(email: str) -> bool:
    return await _run(lambda api: api.delete_temp_profile(email))

async def cleanup_expired_temp_profiles() -> int:
    return await _run(lambda api: api.cleanup_expired_temp_profiles())


async def force_update_profile_expiry(email: str, subscription_end) -> bool:
    expiry_time = get_safe_expiry_timestamp(subscription_end)
    logger.info(f"🔄 force_update_profile_expiry: {email} → {expiry_time}")
    result = await update_client_expiry(email, expiry_time)
    if result:
        logger.info(f"✅ Updated {email}")
    else:
        logger.error(f"🛑 Failed to update {email}")
    return result


async def check_and_fix_subscriptions() -> dict:
    api = XUIAPI()
    try:
        clients_3xui = await api.get_all_clients()
        if not clients_3xui:
            return {"error": "Failed to get clients from 3x-ui"}

        from database import get_users_with_profiles

        users_db = await get_users_with_profiles()

        users_map = {}
        for user in users_db:
            if user.vless_profile_data:
                try:
                    profile_data = json.loads(user.vless_profile_data) if isinstance(user.vless_profile_data, str) else user.vless_profile_data
                    email = profile_data.get("email")
                    if email:
                        users_map[email] = user
                except Exception as e:
                    logger.error(f"🛑 Profile parse error for {user.telegram_id}: {e}")

        stats = {
            "total_3xui": len(clients_3xui),
            "total_db": len(users_db),
            "matched": 0,
            "mismatch": 0,
            "fixed": 0,
            "not_in_db": 0,
            "details": [],
        }

        for client in clients_3xui:
            email = client.get("email")
            if not email or email == "Base":
                continue

            expiry_ms = client.get("expiryTime", 0)
            expiry_3xui = expiry_ms // 1000 if expiry_ms > 0 else 0

            if email not in users_map:
                stats["not_in_db"] += 1
                stats["details"].append({"email": email, "status": "not_in_db", "expiry_3xui": expiry_3xui})
                continue

            user = users_map[email]
            try:
                sub_end = user.subscription_end
                if isinstance(sub_end, str):
                    sub_end = datetime.fromisoformat(sub_end)

                now = datetime.utcnow()
                expiry_db = int(sub_end.timestamp()) if sub_end and sub_end > now else 0
                diff = abs(expiry_3xui - expiry_db)

                entry = {
                    "email": email,
                    "telegram_id": user.telegram_id,
                    "expiry_3xui": expiry_3xui,
                    "expiry_db": expiry_db,
                    "diff": diff,
                }

                if diff <= 60:
                    stats["matched"] += 1
                    entry["status"] = "matched"
                else:
                    stats["mismatch"] += 1
                    logger.warning(f"⚠️ Mismatch {email}: 3xui={expiry_3xui}, db={expiry_db}, diff={diff}")
                    try:
                        ok = await force_update_profile_expiry(email, user.subscription_end)
                        entry["status"] = "fixed" if ok else "fix_failed"
                        if ok:
                            stats["fixed"] += 1
                    except Exception as e:
                        entry["status"] = "fix_error"
                        entry["error"] = str(e)

                stats["details"].append(entry)
            except Exception as e:
                logger.error(f"🛑 Error processing {user.telegram_id}: {e}")

        logger.info(f"📊 check_and_fix_subscriptions: {stats}")
        return stats

    except Exception as e:
        logger.exception(f"🛑 check_and_fix_subscriptions error: {e}")
        return {"error": str(e)}
    finally:
        await api.close()


# ------------------------------------------------------------------ #
#  URL generators                                                      #
# ------------------------------------------------------------------ #

def generate_vless_url(profile_data: dict) -> str:
    remark = profile_data.get("remark", "")
    email = profile_data["email"]
    fragment = f"{remark}-{email}" if remark else email
    return (
        f"vless://{profile_data['client_id']}@{config.XUI_HOST}:{profile_data['port']}"
        f"?type=tcp&security=reality"
        f"&pbk={config.REALITY_PUBLIC_KEY}"
        f"&fp={config.REALITY_FINGERPRINT}"
        f"&sni={config.REALITY_SNI}"
        f"&sid={config.REALITY_SHORT_ID}"
        f"&spx={config.REALITY_SPIDER_X}"
        f"#{fragment}"
    )


def generate_vless_url_temp(profile_data: dict) -> str:
    remark = profile_data.get("remark", "Temp Profile")
    email = profile_data["email"]
    fragment = f"{remark}-{email}" if remark else email
    return (
        f"vless://{profile_data['client_id']}@{config.XUI_HOST}:{profile_data['port']}"
        f"?type=tcp&security=reality"
        f"&pbk={config.TEMP_REALITY_PUBLIC_KEY}"
        f"&fp={config.TEMP_REALITY_FINGERPRINT}"
        f"&sni={config.TEMP_REALITY_SNI}"
        f"&sid={config.TEMP_REALITY_SHORT_ID}"
        f"&spx={config.TEMP_REALITY_SPIDER_X}"
        f"#{fragment}"
    )


def generate_sub_url(sub_id: str) -> str:
    if not config.SUBSCRIPTION_URL_BASE:
        parsed = urlparse(config.XUI_API_URL)
        
        host = parsed.hostname or "localhost"
        return f"http://{host}:{config.XUI_SUB_PORT}/sub/{sub_id}"
    return f"{config.SUBSCRIPTION_URL_BASE.rstrip('/')}:{config.XUI_SUB_PORT}/sub/{sub_id}"
