import asyncio
import json
import logging
import uuid
import random
from datetime import datetime, timedelta
from typing import Optional

from fastapi import FastAPI, Request, Response, Cookie
from fastapi.responses import HTMLResponse, RedirectResponse
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
import qrcode
from io import BytesIO
import base64

from config import config
from functions import XUIAPI

# Настройка логирования
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Temp Profile Server")

# Настройка шаблонов
templates = Jinja2Templates(directory="templates")

# Хранилище временных профилей в памяти
temp_profiles = {}

# Время жизни временного профиля (30 минут)
TEMP_PROFILE_LIFETIME = timedelta(minutes=30)


class TempProfileAPI(XUIAPI):
    """API для работы с временными профилями"""
    
    async def create_temp_profile(self, session_id: str) -> Optional[dict]:
        """Создание временного профиля"""
        logger.info(f"🔍 Creating temp profile for session {session_id}")
        
        if not await self.login():
            logger.error("🛑 Login failed before creating temp profile")
            return None
        
        # Вычисляем время истечения (текущее + 30 минут)
        expiry_time = int((datetime.utcnow() + TEMP_PROFILE_LIFETIME).timestamp())
        logger.info(f"🔍 Temp profile expiry time: {expiry_time} ({datetime.fromtimestamp(expiry_time)})")
        
        try:
            inbound = await self.get_inbound(config.TEMP_INBOUND_ID)
            if not inbound:
                logger.error(f"🛑 Temp inbound {config.TEMP_INBOUND_ID} not found")
                return None
            
            settings = json.loads(inbound["settings"])
            clients = settings.get("clients", [])
            
            client_id = str(uuid.uuid4())
            email = f"temp_{session_id}_{random.randint(1000, 9999)}"
            
            # Получаем flow из инбаунда
            flow = await self._get_flow_from_inbound(inbound)
            
            # Генерируем sub_id
            sub_id = str(uuid.uuid5(uuid.NAMESPACE_DNS, f"temp_{session_id}"))
            
            new_client = {
                "id": client_id,
                "flow": flow,
                "email": email,
                "limitIp": 0,
                "totalGB": 0,
                "expiryTime": expiry_time * 1000,  # 3x-ui ожидает миллисекунды!
                "enable": True,
                "tgId": "",
                "subId": sub_id,
                "reset": 0,
                "fingerprint": config.TEMP_REALITY_FINGERPRINT,
                "publicKey": config.TEMP_REALITY_PUBLIC_KEY,
                "shortId": config.TEMP_REALITY_SHORT_ID,
                "spiderX": config.TEMP_REALITY_SPIDER_X
            }
            
            logger.info(f"🔍 Creating temp client: {email}, expiryTime: {new_client['expiryTime']}")
            
            clients.append(new_client)
            settings["clients"] = clients
            
            update_data = {
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
            
            if await self.update_inbound(config.TEMP_INBOUND_ID, update_data):
                logger.info(f"✅ Temp profile created successfully: {email}")
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
                    "expiry_time": expiry_time
                }
            return None
        except Exception as e:
            logger.exception(f"🛑 Create temp profile error: {e}")
            return None
    
    async def delete_temp_profile(self, email: str) -> bool:
        """Удаление временного профиля"""
        if not await self.login():
            return False
        
        try:
            inbound = await self.get_inbound(config.TEMP_INBOUND_ID)
            if not inbound:
                return False
            
            settings = json.loads(inbound["settings"])
            clients = settings.get("clients", [])
            
            # Фильтруем клиентов
            new_clients = [c for c in clients if c["email"] != email]
            
            # Если не было изменений
            if len(new_clients) == len(clients):
                return False
            
            settings["clients"] = new_clients
            
            update_data = {
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
            
            return await self.update_inbound(config.TEMP_INBOUND_ID, update_data)
        except Exception as e:
            logger.exception(f"🛑 Delete temp profile error: {e}")
            return False


def generate_vless_url_temp(profile_data: dict) -> str:
    """Генерирует VLESS URL для временного профиля"""
    remark = profile_data.get('remark', 'Temp Profile')
    email = profile_data['email']
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


def generate_qr_code(vless_url: str) -> str:
    """Генерирует QR код и возвращает base64 изображение"""
    qr = qrcode.QRCode(
        version=1,
        error_correction=qrcode.constants.ERROR_CORRECT_L,
        box_size=10,
        border=4,
    )
    qr.add_data(vless_url)
    qr.make(fit=True)
    
    img = qr.make_image(fill_color="black", back_color="white")
    
    # Конвертируем в base64
    buffered = BytesIO()
    img.save(buffered, format="PNG")
    img_str = base64.b64encode(buffered.getvalue()).decode()
    
    return f"data:image/png;base64,{img_str}"


async def cleanup_expired_profiles():
    """Фоновая задача для очистки истекших профилей"""
    while True:
        try:
            now = datetime.utcnow()
            expired_sessions = []
            
            for session_id, profile_data in temp_profiles.items():
                expiry_time = profile_data.get('expiry_time', 0)
                expiry_datetime = datetime.fromtimestamp(expiry_time)
                
                if expiry_datetime <= now:
                    expired_sessions.append(session_id)
                    logger.info(f"🗑️ Cleaning up expired temp profile: {session_id}")
            
            # Удаляем истекшие профили из 3x-ui
            api = TempProfileAPI()
            try:
                for session_id in expired_sessions:
                    email = temp_profiles[session_id].get('email')
                    if email:
                        await api.delete_temp_profile(email)
                        logger.info(f"✅ Deleted expired temp profile from 3x-ui: {email}")
            finally:
                await api.close()
            
            # Удаляем из памяти
            for session_id in expired_sessions:
                del temp_profiles[session_id]
            
            if expired_sessions:
                logger.info(f"🧹 Cleaned up {len(expired_sessions)} expired temp profiles")
            
        except Exception as e:
            logger.error(f"🛑 Cleanup error: {e}")
        
        # Проверяем каждую минуту
        await asyncio.sleep(60)


@app.get("/", response_class=HTMLResponse)
async def index(request: Request, temp_session: Optional[str] = Cookie(None)):
    """Главная страница с QR кодом"""
    
    # Проверяем наличие cookie с сессией
    if temp_session and temp_session in temp_profiles:
        # Используем существующий профиль
        profile_data = temp_profiles[temp_session]
        vless_url = generate_vless_url_temp(profile_data)
        qr_code = generate_qr_code(vless_url)
        
        return templates.TemplateResponse("temp_profile.html", {
            "request": request,
            "qr_code": qr_code,
            "show_cookie_warning": False,
            "session_id": temp_session
        })
    
    # Создаем новый временный профиль
    session_id = str(uuid.uuid4())
    api = TempProfileAPI()
    
    try:
        profile_data = await api.create_temp_profile(session_id)
        
        if not profile_data:
            return templates.TemplateResponse("error.html", {
                "request": request,
                "error": "Не удалось создать временный профиль. Попробуйте позже."
            })
        
        # Сохраняем профиль в памяти
        temp_profiles[session_id] = profile_data
        
        # Генерируем VLESS URL и QR код
        vless_url = generate_vless_url_temp(profile_data)
        qr_code = generate_qr_code(vless_url)
        
        # Создаем ответ с cookie
        response = templates.TemplateResponse("temp_profile.html", {
            "request": request,
            "qr_code": qr_code,
            "show_cookie_warning": True,
            "session_id": session_id
        })
        
        # Определяем_secure флаг для cookie
        ssl_enabled = config.TEMP_SSL_CERT_PATH and config.TEMP_SSL_KEY_PATH
        
        # Устанавливаем cookie на 30 минут
        response.set_cookie(
            key="temp_session",
            value=session_id,
            max_age=1800,  # 30 минут
            httponly=True,
            secure=ssl_enabled,
            samesite="lax"
        )
        
        return response
        
    finally:
        await api.close()


@app.get("/connect")
async def connect(temp_session: Optional[str] = Cookie(None)):
    """Редирект на страницу подписки"""
    
    if temp_session and temp_session in temp_profiles:
        profile_data = temp_profiles[temp_session]
        sub_id = profile_data.get('sub_id')
        
        if sub_id:
            # Определяем протокол на основе наличия SSL сертификатов
            ssl_enabled = config.TEMP_SSL_CERT_PATH and config.TEMP_SSL_KEY_PATH
            scheme = "https" if ssl_enabled else "http"
            sub_url = f"{scheme}://{config.SUBSCRIPTION_URL_BASE}:{config.XUI_SUB_PORT}/sub/{sub_id}"
            logger.info(f"🔗 Redirecting to subscription: {sub_url}")
            return RedirectResponse(url=sub_url)
    
    # Если нет активной сессии, редиректим на главную
    return RedirectResponse(url="/")


@app.get("/refresh")
async def refresh_profile(temp_session: Optional[str] = Cookie(None)):
    """Обновление временного профиля"""
    
    # Удаляем старый cookie
    response = RedirectResponse(url="/")
    response.delete_cookie(key="temp_session")
    
    # Если была старая сессия, удаляем профиль
    if temp_session and temp_session in temp_profiles:
        email = temp_profiles[temp_session].get('email')
        if email:
            api = TempProfileAPI()
            try:
                await api.delete_temp_profile(email)
                logger.info(f"✅ Deleted old temp profile: {email}")
            finally:
                await api.close()
        
        del temp_profiles[temp_session]
    
    return response


@app.get("/health")
async def health_check():
    """Проверка здоровья сервера"""
    return {"status": "healthy", "active_profiles": len(temp_profiles)}


# Запуск фоновых задач при старте
@app.on_event("startup")
async def startup_event():
    """Запуск фоновых задач"""
    port = int(config.TEMP_WEB_SERVER_PORT) if isinstance(config.TEMP_WEB_SERVER_PORT, str) else config.TEMP_WEB_SERVER_PORT
    logger.info("🚀 Starting temp profile server...")
    logger.info(f"📋 Temp inbound ID: {config.TEMP_INBOUND_ID}")
    logger.info(f"📋 Web server port: {port}")
    
    # Проверяем SSL сертификаты
    ssl_enabled = config.TEMP_SSL_CERT_PATH and config.TEMP_SSL_KEY_PATH
    if ssl_enabled:
        logger.info(f"🔐 SSL enabled: {config.TEMP_SSL_CERT_PATH}")
    else:
        logger.warning("⚠️ SSL certificates not configured")
    
    # Запускаем фоновую задачу очистки
    asyncio.create_task(cleanup_expired_profiles())
    logger.info("✅ Cleanup task started")


if __name__ == "__main__":
    import uvicorn
    import os
    
    port = int(config.TEMP_WEB_SERVER_PORT) if isinstance(config.TEMP_WEB_SERVER_PORT, str) else config.TEMP_WEB_SERVER_PORT
    
    # Проверяем наличие SSL сертификатов
    ssl_keyfile = config.TEMP_SSL_KEY_PATH if config.TEMP_SSL_KEY_PATH and os.path.exists(config.TEMP_SSL_KEY_PATH) else None
    ssl_certfile = config.TEMP_SSL_CERT_PATH if config.TEMP_SSL_CERT_PATH and os.path.exists(config.TEMP_SSL_CERT_PATH) else None
    
    if ssl_keyfile and ssl_certfile:
        logger.info(f"🚀 Starting temp profile server with HTTPS on port {port}")
        logger.info(f"🔐 Using SSL certificate: {ssl_certfile}")
        uvicorn.run(
            app,
            host="0.0.0.0",
            port=port,
            ssl_keyfile=ssl_keyfile,
            ssl_certfile=ssl_certfile,
            log_level="info"
        )
    else:
        logger.info(f"🚀 Starting temp profile server with HTTP on port {port}")
        logger.warning("⚠️ SSL certificates not found, running in HTTP mode")
        uvicorn.run(
            app,
            host="0.0.0.0",
            port=port,
            log_level="info"
        )