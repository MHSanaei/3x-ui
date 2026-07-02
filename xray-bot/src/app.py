import json
import asyncio
import logging
import warnings
import coloredlogs
from config import config
from aiogram import Bot, Dispatcher
from aiogram.types import PreCheckoutQuery
from handlers import setup_handlers
from datetime import datetime, timedelta
from functions import delete_client_by_email
from database import Session, User, init_db, get_all_users, delete_user_profile

warnings.filterwarnings("ignore", category=DeprecationWarning)

# Настройка логирования
coloredlogs.install(level='info')
logger = logging.getLogger(__name__)

async def check_subscriptions(bot: Bot):
    """Проверка статуса подписок"""
    while True:
        try:
            now = datetime.utcnow()
            users = await get_all_users()
            
            for user in users:
                # Проверка за 1 день до окончания
                if user.subscription_end - now < timedelta(days=1) and user.subscription_end >= now and not user.notified:
                    try:
                        await bot.send_message(
                            user.telegram_id,
                            "⚠️ Ваша подписка истекает через 24 часа! Продлите подписку, чтобы сохранить доступ."
                        )
                        # Помечаем как уведомленного
                        with Session() as session:
                            db_user = session.query(User).filter_by(telegram_id=user.telegram_id).first()
                            if db_user:
                                db_user.notified = True
                                session.commit()
                    except Exception as e:
                        logger.warning(f"⚠️ Notification error: {e}")
                
                # Проверка истечения подписки
                if user.subscription_end <= now and user.vless_profile_data:
                    try:
                        profile = json.loads(user.vless_profile_data)
                        # Удаляем из инбаунда
                        success = await delete_client_by_email(profile["email"])
                        if success:
                            # Удаляем профиль из БД
                            await delete_user_profile(user.telegram_id)
                            
                            await bot.send_message(
                                user.telegram_id,
                                "❌ Ваша подписка истекла! Профиль VPN был удален. Продлите подписку, чтобы создать новый."
                            )
                        else:
                            logger.warning(f"⚠️ Failed to delete client {profile['email']} from inbound")
                    except Exception as e:
                        logger.warning(f"⚠️ Deletion error: {e}")
        except Exception as e:
            logger.warning(f"⚠️ Subscription check error: {e}")
        
        await asyncio.sleep(3600)

async def update_admins_status():
    """Обновляет статус администраторов в базе данных"""
    with Session() as session:
        # Сбрасываем статус администратора у всех пользователей
        session.query(User).update({User.is_admin: False})
        
        # Устанавливаем статус администратора для пользователей из config.ADMINS
        for admin_id in config.ADMINS:
            user = session.query(User).filter_by(telegram_id=admin_id).first()
            if user:
                user.is_admin = True
            else:
                # Если администратора нет в базе, создаем запись
                new_admin = User(
                    telegram_id=admin_id,
                    full_name=f"Admin {admin_id}",
                    is_admin=True,
                    subscription_end=datetime.now() + timedelta(days=365)
                )
                session.add(new_admin)
        
        session.commit()
    logger.info("✅ Admin status updated in database")

async def setup_bot_commands(bot: Bot):
    """Регистрация команд бота в меню Telegram"""
    from aiogram.types import BotCommand
    
    commands = [
        BotCommand(command="start", description="🚀 Запуск бота"),
        BotCommand(command="menu", description="📋 Главное меню"),
        BotCommand(command="renew", description="💵 Продлить подписку"),
        BotCommand(command="connect", description="✅ Подключить VPN"),
        BotCommand(command="stats", description="📊 Статистика"),
        BotCommand(command="help", description="ℹ️ Справка"),
    ]
    
    try:
        await bot.set_my_commands(commands)
        logger.info("✅ Bot commands registered successfully")
    except Exception as e:
        logger.error(f"❌ Failed to register bot commands: {e}")

async def main():
    bot = Bot(token=config.BOT_TOKEN)
    dp = Dispatcher()
    
    try:
        await init_db()
        logger.info("✅ Database initialized")

        # Обновляем статус администраторов
        await update_admins_status()
    except Exception as e:
        logger.error(f"❌ Database initialization error: {e}")
        return
    
    try:
        # Регистрируем команды бота
        await setup_bot_commands(bot)
    except Exception as e:
        logger.error(f"❌ Bot commands setup error: {e}")
    
    try:
        setup_handlers(dp)
        logger.info("✅ Handlers registered")
    except Exception as e:
        logger.error(f"❌ Handler registration error: {e}")
        return
    
    # Обработчик для предварительной проверки платежа
    @dp.pre_checkout_query()
    async def process_pre_checkout_query(pre_checkout_query: PreCheckoutQuery):
        await bot.answer_pre_checkout_query(pre_checkout_query.id, ok=True)
    
    # Запускаем фоновую задачу проверки подписок
    try:
        asyncio.create_task(check_subscriptions(bot))
    except Exception as e:
        logger.error(f"❌ Subscription check task failed to start: {e}")
    
    logger.info("ℹ️  Starting bot...")
    try:
        await dp.start_polling(bot)
    except Exception as e:
        logger.error(f"❌ Bot start error: {e}")
        return

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("👋 Stopping bot...")
        exit(0)
    except Exception as e:
        logger.error(f"❌ Main loop error: {e}")
        exit(1)