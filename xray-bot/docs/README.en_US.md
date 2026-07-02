**Язык / Language:** [Русский](../README.md) **|** <ins>English</ins>

<div id="header" align="center"><h1>XRay VPN Bot [Telegram]</h1></div>

<div id="header" align="center"><img alt="GitHub last commit" src="https://img.shields.io/github/last-commit/QueenDekim/XRay-bot"> <img alt="GitHub commit activity" src="https://img.shields.io/github/commit-activity/m/QueenDekim/XRay-bot"><br><img alt="GitHub top language" src="https://img.shields.io/github/languages/top/QueenDekim/XRay-bot"> <a href="./LICENSE" target="_blank"><img alt="GitHub License" src="https://img.shields.io/github/license/QueenDekim/XRay-bot"></a></div>

## Project Description

This project is a Telegram bot for selling and managing VPN subscriptions via the 3X-UI control panel. The bot allows users to purchase VPN subscriptions, create and manage their profiles, and enables administrators to manage users and track statistics.

Key Features:

- User registration with a trial period
- Subscription renewal via Telegram's built-in payment system
- Creation and deletion of VPN profiles (VLESS) in the 3X-UI panel
- **Temporary 30-minute profiles for testing**
- Subscription expiration notifications
- **QR code generation for quick connection**
- **New quick access commands: /renew, /connect, /stats, /help**
- Administrative menu for user management and broadcast messages
- Traffic usage statistics
- **Automatic subscription date and profile fixing**
- **Subscription verification and synchronization between 3x-ui and database**

## Installation and Setup

### Prerequisites

- Python 3.10+
- 3X-UI control panel
   - An inbound created with the security setting set to `Reality`
   - **Optional: separate inbound for temporary profiles**
- A Telegram bot (created via `@BotFather`)
- **SSL certificates for HTTPS (for temporary profiles)**

### Installation Steps

1. Clone the repository:

```bash
git clone https://github.com/QueenDekim/XRay-bot
cd XRay-bot
```

2. Install dependencies:

```bash
python -m venv .venv # use python3 on Linux
.venv\Scripts\activate
# source .venv/bin/activate on Linux
pip install -r requirements.txt
```

3. Configure environment variables:

```bash
cp src\.env.example src\.env # use "/" instead of "\" on Linux
# Edit the .env file with your values
```

4. Run the bot:

```bash
python src\app.py # use python3 and "/" instead of "\" on Linux
```

**To run the temporary profile web server (optional):**

```bash
python src\temp_profile_server.py # use python3 and "/" instead of "\" on Linux
```

### Environment Variables Configuration

Mandatory parameters in `.env`:

- `BOT_TOKEN` - Your Telegram bot token from @BotFather
- `PAYMENT_TOKEN` - Payment token from @BotFather
- `ADMINS` - Administrator IDs, comma-separated
- `XUI_API_URL` - 3X-UI panel URL (e.g., http://ip:54321)
- `XUI_USERNAME` and `XUI_PASSWORD` - Panel credentials
- `INBOUND_ID` - Inbound ID in the 3X-UI panel
- Reality parameters (public key, fingerprint, SNI, etc.)

**Optional parameters for temporary profiles:**

- `TEMP_INBOUND_ID` - Inbound ID for temporary profiles (default: 2)
- `TEMP_REALITY_PUBLIC_KEY` - Public key for temporary profiles
- `TEMP_REALITY_FINGERPRINT` - Fingerprint for temporary profiles
- `TEMP_REALITY_SNI` - SNI for temporary profiles
- `TEMP_REALITY_SHORT_ID` - Short ID for temporary profiles
- `TEMP_REALITY_SPIDER_X` - Spider X for temporary profiles
- `TEMP_WEB_SERVER_PORT` - Web server port for temporary profiles (default: 8080)
- `TEMP_SSL_CERT_PATH` - Path to SSL certificate (fullchain.pem)
- `TEMP_SSL_KEY_PATH` - Path to SSL private key (privkey.pem)

## Bot Commands

### User Commands

- `/start` - Start the bot and register
- `/menu` - Main menu
- `/renew` - Renew subscription
- `/connect` - Connect to VPN with QR code
- `/stats` - View usage statistics
- `/help` - Help

### Administrative Functions

Administrators have access to a special menu with functions:

- Adding/removing subscription time
- **Deleting users with profile cleanup in 3x-ui**
- Viewing the user list
- **Checking and fixing subscription discrepancies**
- **Fixing all profiles with incorrect dates**
- Network usage statistics
- Broadcasting messages to users
- Managing static profiles

## Technical Architecture

### File Structure

```
./
├── src
│   ├── .env.example              # Example configuration file
│   ├── app.py                    # Main application file
│   ├── config.py                 # Application configuration
│   ├── database.py               # Database models and functions
│   ├── functions.py              # Functions for 3X-UI API interaction
│   ├── handlers.py               # Command and callback handlers
│   └── temp_profile_server.py    # Temporary profile web server
├── templates                     # Templates for temporary profiles
│   ├── temp_profile.html         # Temporary profile page
│   └── error.html                # Error page
├── docs                          # Documentation in other languages
│   └── README.en_US              # Documentation in English
├── users.db                      # SQLite database file
├── README.md                     # Documentation in Russian
└── requirements.txt              # Project dependencies
```

### Database

The project uses `SQLite` with `SQLAlchemy ORM`. Main tables:

1. **`users`** - User information:
   - `telegram_id` - User's Telegram ID
   - `subscription_end` - Subscription end date
   - `vless_profile_data` - VPN profile data in JSON
   - `is_admin` - Administrator flag
2. **`static_profiles`** - Static VPN profiles:
   - `name` - Profile name
   - `vless_url` - VLESS URL

### Core Components

#### 1. `app.py`

The main application file that:
- Initializes the database
- Starts the background task for subscription checks
- Handles payment pre-checkout and successful payment queries
- **Registers bot commands in Telegram menu**
- Starts the bot's polling

#### 2. `config.py`

Loads and validates configuration using `Pydantic`. Contains:
- 3X-UI panel connection settings
- Reality protocol parameters
- **Parameters for temporary profiles**
- Subscription prices and discounts
- Functions for cost calculation

#### 3. `database.py`

Models and functions for database interaction:
- `User` model for storing users
- `StaticProfile` model for static profiles
- Functions for managing subscriptions and profiles
- **validate_and_fix_subscription_date function for fixing dates**
- **delete_user function for deleting users**
- **get_users_with_profiles and fix_all_subscription_dates functions**

#### 4. `functions.py`

The `XUIAPI` class for interacting with the **3X-UI** panel:
- Panel authentication
- Creating and deleting clients
- **Updating profile expiry times**
- Retrieving usage statistics
- Generating VLESS URLs
- **get_safe_expiry_timestamp function for safe timestamp retrieval**
- **check_and_fix_subscriptions function for subscription verification**
- **force_update_profile_expiry function for forced updates**

#### 5. `handlers.py`

Command and callback handlers:
- `/start`, `/menu`, `/renew`, `/connect`, `/stats`, `/help` commands
- Payment processing
- Administrative functions
- Profile management
- **Handlers for new admin functions**

#### 6. `temp_profile_server.py`

Web server for temporary profiles:
- FastAPI application
- Creating 30-minute temporary profiles
- Automatic deletion upon expiration
- HTTPS support with SSL certificates

## Payment Processing

The bot uses Telegram's built-in payment system. When a subscription is selected:

1. The user selects a subscription period
2. The bot creates an invoice via `bot.send_invoice()`
3. After successful payment, it is processed by `process_successful_payment()`
4. The user's subscription is extended
5. **Automatically updates expiry_time in 3x-ui**

## Administrative Functions

Administrators have access to a special menu with functions:

- Adding/removing subscription time
- **Deleting users with full profile cleanup in 3x-ui**
- Viewing the user list
- **Checking subscriptions - identifying discrepancies between 3x-ui and DB**
- **Fixing profiles - automatic fixing of all dates**
- Network usage statistics
- Broadcasting messages to users
- Managing static profiles

## Integration with **3X-UI**

The bot interacts with the **3X-UI** panel via its API:

1. Authentication via login/password
2. Retrieving inbound data
3. Adding clients to the inbound settings
4. Updating the inbound configuration
5. **Updating expiry_time for existing clients**

## VLESS URL Generation

VLESS URL format for Reality:

```
vless://{client_id}@{host}:{port}?type=tcp&security=reality&pbk={public_key}&fp={fingerprint}&sni={sni}&sid={short_id}&spx={spider_x}#{remark}
```

## Monitoring and Notifications

The bot automatically checks subscriptions every hour and:

- Notifies users 24 hours before expiration
- Deletes profiles with expired subscriptions
- Sends payment notifications to administrators
- **Fixes incorrect subscription dates**

## QR Code Generation

The bot automatically generates QR codes for profiles:
- Uses the `qrcode` library
- Creates a QR code with the profile subscription
- Sends the image to the user

## Temporary Profiles

The temporary profile functionality allows:
- Creating 30-minute profiles for testing
- Using a separate inbound for temporary profiles
- Automatically deleting profiles upon expiration
- Providing access through a web interface

## Prices and Discounts

The bot supports a flexible pricing system:
- 1 month - 100 rub.
- 3 months - 300 rub. (10% discount)
- 6 months - 600 rub. (20% discount)
- 12 months - 1200 rub. (30% discount)

## Security

- All sensitive data is stored in environment variables
- Configuration validation is done via Pydantic
- Restricted access to administrative functions
- Secure storage of payment information through Telegram
- **Validation and fixing of subscription dates**
- **Verification of discrepancies between 3x-ui and database**

## Potential Issues and Solutions

1. **3X-UI Connection Errors** - Check the URL and credentials
2. **Payment Issues** - Ensure the payment token is correct
3. **Database Errors** - Check write permissions in the directory
4. **Notifications Not Working** - Check time and timezone settings
5. **Incorrect Subscription Dates** - Use the "Fix Profiles" function in the admin menu
6. **Date Discrepancies** - Use the "Check Subscriptions" function in the admin menu

---

*For additional information, refer to the [aiogram](https://docs.aiogram.dev/en/latest/) and [3X-UI](https://github.com/MHSanaei/3x-ui/wiki) documentation.*

---

## Donation USDT (TON Network):

| QR Code                      | Address                                            |
| ---------------------------- | -------------------------------------------------- |
| ![QR-code](./qr-code.jpg)    | `UQA9SigQDdUlZhFj3C5L71gFwjs2kSZu1b9g7Huu1PQujrVS` |

| Demo - Fully functional bot                            | Communication with the developer                 |
| ------------------------------------------------------ | ------------------------------------------------ |
| Telegram: [@Dekim_vpn_bot](https://t.me/Dekim_vpn_bot) | Telegram: [@QueenDek1m](https://t.me/QueenDek1m) |
|                                                        | Discord: `from_russia_with_love`                 |
