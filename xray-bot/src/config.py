import os
from dotenv import load_dotenv
from pydantic import BaseModel, Field, field_validator
from typing import List, Dict

load_dotenv()

class Config(BaseModel):
    BOT_TOKEN: str = os.getenv("BOT_TOKEN", "")
    ADMINS: List[int] = Field(default_factory=list)
    XUI_API_URL: str = os.getenv("XUI_API_URL", "http://localhost:54321")
    XUI_BASE_PATH: str = os.getenv("XUI_BASE_PATH", "/panel")
    XUI_SUB_PORT: str = os.getenv("XUI_SUB_PORT", "54321")
    XUI_API_TOKEN: str = os.getenv("XUI_API_TOKEN", "TOKEN")
    XUI_USERNAME: str = os.getenv("XUI_USERNAME", "admin")
    XUI_PASSWORD: str = os.getenv("XUI_PASSWORD", "admin")
    XUI_HOST: str = os.getenv("XUI_HOST", "your-server.com")
    XUI_SERVER_NAME: str = os.getenv("XUI_SERVER_NAME", "domain.com")
    XUI_VERIFY_SSL: bool = Field(default=os.getenv("XUI_VERIFY_SSL", "True").lower() == "true")
    PAYMENT_TOKEN: str = os.getenv("PAYMENT_TOKEN", "")
    INBOUND_ID: int = Field(default=os.getenv("INBOUND_ID", 1))
    REALITY_PUBLIC_KEY: str = os.getenv("REALITY_PUBLIC_KEY", "")
    REALITY_FINGERPRINT: str = os.getenv("REALITY_FINGERPRINT", "chrome")
    REALITY_SNI: str = os.getenv("REALITY_SNI", "example.com")
    REALITY_SHORT_ID: str = os.getenv("REALITY_SHORT_ID", "1234567890")
    REALITY_SPIDER_X: str = os.getenv("REALITY_SPIDER_X", "/")
    # Временные профили (30 минут)
    TEMP_INBOUND_ID: int = Field(default=os.getenv("TEMP_INBOUND_ID", 2))
    TEMP_REALITY_PUBLIC_KEY: str = os.getenv("TEMP_REALITY_PUBLIC_KEY", "")
    TEMP_REALITY_FINGERPRINT: str = os.getenv("TEMP_REALITY_FINGERPRINT", "chrome")
    TEMP_REALITY_SNI: str = os.getenv("TEMP_REALITY_SNI", "example.com")
    TEMP_REALITY_SHORT_ID: str = os.getenv("TEMP_REALITY_SHORT_ID", "1234567890")
    TEMP_REALITY_SPIDER_X: str = os.getenv("TEMP_REALITY_SPIDER_X", "/")
    TEMP_WEB_SERVER_PORT: int = Field(default=os.getenv("TEMP_WEB_SERVER_PORT", 8080))
    TEMP_SSL_CERT_PATH: str = os.getenv("TEMP_SSL_CERT_PATH", "")
    TEMP_SSL_KEY_PATH: str = os.getenv("TEMP_SSL_KEY_PATH", "")

    # Настройки цен и скидок
    PRICES: Dict[int, Dict[str, int]] = {
        1: {"base_price": 100, "discount_percent": 0},
        3: {"base_price": 300, "discount_percent": 10},
        6: {"base_price": 600, "discount_percent": 20},
        12: {"base_price": 1200, "discount_percent": 30}
    }
    SUBSCRIPTION_URL_BASE: str = os.getenv("SUBSCRIPTION_URL_BASE", "")

    @field_validator('ADMINS', mode='before')
    def parse_admins(cls, value):
        if isinstance(value, str):
            return [int(admin) for admin in value.split(",") if admin.strip()]
        return value or []
    
    @field_validator('INBOUND_ID', mode='before')
    def parse_inbound_id(cls, value):
        if isinstance(value, str):
            return int(value)
        return value or 15
    
    @field_validator('TEMP_INBOUND_ID', mode='before')
    def parse_temp_inbound_id(cls, value):
        if isinstance(value, str):
            return int(value)
        return value or 2
    
    @field_validator('TEMP_WEB_SERVER_PORT', mode='before')
    def parse_temp_web_server_port(cls, value):
        if isinstance(value, str):
            return int(value)
        return value or 8080
    
    def calculate_price(self, months: int) -> int:
        """Вычисляет итоговую стоимость с учетом скидки"""
        if months not in self.PRICES:
            return 0
        
        price_info = self.PRICES[months]
        base_price = price_info["base_price"]
        discount_percent = price_info["discount_percent"]
        
        discount_amount = (base_price * discount_percent) // 100
        return base_price - discount_amount

config = Config(
    ADMINS=os.getenv("ADMINS", ""),
    INBOUND_ID=os.getenv("INBOUND_ID", 15)
)
