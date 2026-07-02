from sqlalchemy import create_engine, Column, Integer, String, DateTime, Boolean, func
from sqlalchemy.orm import declarative_base, sessionmaker
from datetime import datetime, timedelta
import logging

logger = logging.getLogger(__name__)

Base = declarative_base()

class User(Base):
    __tablename__ = 'users'
    id = Column(Integer, primary_key=True)
    telegram_id = Column(Integer, unique=True)
    full_name = Column(String)
    username = Column(String)
    registration_date = Column(DateTime, default=datetime.utcnow)
    subscription_end = Column(DateTime)
    vless_profile_id = Column(String)
    vless_profile_data = Column(String)
    is_admin = Column(Boolean, default=False)
    notified = Column(Boolean, default=False)

class StaticProfile(Base):
    __tablename__ = 'static_profiles'
    id = Column(Integer, primary_key=True)
    name = Column(String)
    vless_url = Column(String)
    created_at = Column(DateTime, default=datetime.utcnow)

engine = create_engine('sqlite:///users.db', echo=False)
Session = sessionmaker(bind=engine)

async def init_db():
    Base.metadata.create_all(engine)
    logger.info("✅ Database tables created")

async def get_user(telegram_id: int):
    with Session() as session:
        user = session.query(User).filter_by(telegram_id=telegram_id).first()
        if user:
            # Проверяем и исправляем дату подписки если нужно
            original_end = user.subscription_end
            user.subscription_end = validate_and_fix_subscription_date(user.subscription_end)
            if user.subscription_end != original_end:
                session.commit()
                logger.info(f"✅ Fixed subscription date for user {telegram_id}: {original_end} -> {user.subscription_end}")
        return user

async def create_user(telegram_id: int, full_name: str, username: str = None, is_admin: bool = False):
    with Session() as session:
        # Создаем пользователя с корректной датой подписки
        subscription_end = validate_and_fix_subscription_date(datetime.utcnow() + timedelta(days=3))
        user = User(
            telegram_id=telegram_id,
            full_name=full_name,
            username=username,
            subscription_end=subscription_end,
            is_admin=is_admin
        )
        session.add(user)
        session.commit()
        logger.info(f"✅ New user created: {telegram_id} with subscription_end: {subscription_end}")
        return user

async def delete_user_profile(telegram_id: int):
    with Session() as session:
        user = session.query(User).filter_by(telegram_id=telegram_id).first()
        if user:
            user.vless_profile_data = None
            user.notified = False
            session.commit()
            logger.info(f"✅ User profile deleted: {telegram_id}")

async def update_subscription(telegram_id: int, months: int):
    """Обновляет подписку с учетом текущего состояния"""
    with Session() as session:
        user = session.query(User).filter_by(telegram_id=telegram_id).first()
        if user:
            now = datetime.utcnow()
            
            # Сначала проверяем и исправляем текущую дату если нужно
            user.subscription_end = validate_and_fix_subscription_date(user.subscription_end)
            
            # Если подписка активна, добавляем к текущей дате окончания
            if user.subscription_end > now:
                user.subscription_end += timedelta(days=months * 30)
            else:
                # Если подписка истекла, начинаем с текущей даты
                user.subscription_end = now + timedelta(days=months * 30)
            
            # Проверяем и исправляем итоговую дату
            user.subscription_end = validate_and_fix_subscription_date(user.subscription_end)
            
            # Сбрасываем флаг уведомления
            user.notified = False
            session.commit()
            logger.info(f"✅ Subscription updated for {telegram_id}: +{months} months, new end: {user.subscription_end}")
            return True
        return False

async def get_all_users(with_subscription: bool = None):
    with Session() as session:
        query = session.query(User)
        if with_subscription is not None:
            if with_subscription:
                query = query.filter(User.subscription_end > datetime.utcnow())
            else:
                query = query.filter(User.subscription_end <= datetime.utcnow())
        return query.all()

async def create_static_profile(name: str, vless_url: str):
    with Session() as session:
        profile = StaticProfile(name=name, vless_url=vless_url)
        session.add(profile)
        session.commit()
        logger.info(f"✅ Static profile created: {name}")
        return profile

async def get_static_profiles():
    with Session() as session:
        return session.query(StaticProfile).all()

async def get_user_stats():
    with Session() as session:
        total = session.query(func.count(User.id)).scalar()
        with_sub = session.query(func.count(User.id)).filter(User.subscription_end > datetime.utcnow()).scalar()
        without_sub = total - with_sub
        return total, with_sub, without_sub

async def get_users_with_profiles():
    """Получает всех пользователей с профилями"""
    with Session() as session:
        return session.query(User).filter(User.vless_profile_data.isnot(None)).all()

async def fix_all_subscription_dates():
    """Исправляет все некорректные даты подписок в базе данных"""
    with Session() as session:
        users = session.query(User).all()
        fixed_count = 0
        
        for user in users:
            original_end = user.subscription_end
            user.subscription_end = validate_and_fix_subscription_date(user.subscription_end)
            
            if user.subscription_end != original_end:
                fixed_count += 1
                logger.info(f"✅ Fixed subscription date for user {user.telegram_id}: {original_end} -> {user.subscription_end}")
        
        session.commit()
        logger.info(f"📊 Fixed {fixed_count} subscription dates out of {len(users)} users")
        return fixed_count

async def delete_user(telegram_id: int) -> bool:
    """Удаляет пользователя из базы данных
    
    Args:
        telegram_id: Telegram ID пользователя для удаления
        
    Returns:
        True если пользователь был найден и удалён, False если пользователь не найден
    """
    with Session() as session:
        user = session.query(User).filter_by(telegram_id=telegram_id).first()
        if user:
            # Сначала удаляем профиль из 3x-ui если он есть
            if user.vless_profile_data:
                try:
                    from functions import delete_client_by_email
                    import json
                    profile_data = json.loads(user.vless_profile_data)
                    email = profile_data.get("email")
                    if email:
                        delete_result = await delete_client_by_email(email)
                        if delete_result:
                            logger.info(f"✅ Deleted profile from 3x-ui for user {telegram_id}")
                        else:
                            logger.warning(f"⚠️ Failed to delete profile from 3x-ui for user {telegram_id}")
                except Exception as e:
                    logger.error(f"🛑 Error deleting profile from 3x-ui: {e}")
            
            # Удаляем пользователя из базы данных
            session.delete(user)
            session.commit()
            logger.info(f"✅ User {telegram_id} deleted from database")
            return True
        else:
            logger.warning(f"⚠️ User {telegram_id} not found in database")
            return False

def validate_and_fix_subscription_date(subscription_end: datetime) -> datetime:
    """Проверяет и исправляет дату окончания подписки
    
    Args:
        subscription_end: Текущая дата окончания подписки (datetime или str)
        
    Returns:
        Исправленная дата окончания подписки (datetime)
    """
    now = datetime.utcnow()
    
    # Конвертируем строку в datetime если нужно
    if isinstance(subscription_end, str):
        try:
            subscription_end = datetime.fromisoformat(subscription_end)
            logger.debug(f"🔄 Конвертирована строка в datetime: {subscription_end}")
        except Exception as e:
            logger.error(f"🛑 Ошибка конвертации строки в datetime: {e}, value: {subscription_end}")
            return now + timedelta(days=3)
    
    # Проверяем, что subscription_end является datetime объектом
    if not isinstance(subscription_end, datetime):
        logger.error(f"🛑 subscription_end не является datetime: {type(subscription_end)}, value: {subscription_end}")
        return now + timedelta(days=3)
    
    # Если дата слишком старая (до 2020 года) или в будущем более чем на 10 лет
    if subscription_end < datetime(2020, 1, 1) or subscription_end > now + timedelta(days=3650):
        logger.warning(f"⚠️ Invalid subscription date detected: {subscription_end}, resetting to current time")
        return now + timedelta(days=3)  # Даем 3 дня тестового периода
    
    return subscription_end