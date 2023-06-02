import telegram
import subprocess

# Replace YOUR_TOKEN with your bot token
bot = telegram.Bot(token='6150347379:AAEKH1l5RIuiNtW9Xa4chKkRUzOUwEUxPlI')

# Define a function to send the VLESS config to the user
def send_vless_config(chat_id):
    # Generate the VLESS config using the command-line tool v2ctl
    vless_config = subprocess.check_output(["v2ctl", "config"], universal_newlines=True)
    
    # Send the VLESS config as a message to the user
    bot.send_message(chat_id=chat_id, text=vless_config)

# Add a handler for the /start command
def start_handler(update: telegram.Update, context: telegram.ext.CallbackContext):
    chat_id = update.message.chat_id
    
    # Call the send_vless_config function to send the VLESS config to the user
    send_vless_config(chat_id)
    
# Create a dispatcher and add the start_handler to handle the /start command
dispatcher = telegram.ext.Dispatcher(bot, None, workers=0)
dispatcher.add_handler(telegram.ext.CommandHandler('start', start_handler))

# Start the bot
updater = telegram.ext.Updater(bot.token, use_context=True, workers=0)
updater.dispatcher = dispatcher
updater.start_polling()
