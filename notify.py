import asyncio
import psutil
import telegram
import schedule
import time
import socket
import cpuinfo

TOKEN = 'YOUR_BOT_TOKEN'
CHAT_ID = 'YOUR_CHAT_ID'

def get_system_info():
    cpu_usage = psutil.cpu_percent()
    cpu_freq = round(psutil.cpu_freq().current / 1000, 2)  # Convert to GHz and round to 2 decimal places
    ram = psutil.virtual_memory()
    ram_total = round(ram.total / (1024.0 **3), 2)
    ram_used = round(ram.used / (1024.0 **3), 2)
    disk = psutil.disk_usage('/')
    disk_total = round(disk.total / (1024.0 **3), 2)
    disk_used = round(disk.used / (1024.0 **3), 2)
    server_name = socket.gethostname()
    cpu_name = cpuinfo.get_cpu_info()['brand_raw']

    message = f"Tên server: {server_name}\nTên CPU: {cpu_name}\nXung nhịp CPU: {cpu_freq}GHz\nMức sử dụng CPU: {cpu_usage}%\nMức sử dụng RAM: {ram_used}GB/{ram_total}GB\nMức sử dụng ổ đĩa: {disk_used}GB/{disk_total}GB"
    
    return message

async def send_info():
    bot = telegram.Bot(token=TOKEN)
    await bot.send_message(chat_id=CHAT_ID, text=get_system_info())

if __name__ == '__main__':
    schedule.every().hour.at(":00").do(asyncio.run, send_info())

    while True:
        schedule.run_pending()
        time.sleep(1)
