# X-UI API Documentation

## Overview

This document provides instructions for interacting with the X-UI panel API. The API allows for programmatic management of inbounds (connections), users, and server settings.

### Authentication

The API uses a **cookie-based session** for authentication.

1.  You must first send a `POST` request to the `/login` endpoint with your credentials.
2.  If successful, the server will respond with a `Set-Cookie` header containing a session cookie (e.g., `session=...`).
3.  Your HTTP client **must** store this cookie and include it in the `Cookie` header for all subsequent API requests.

While many headers sent by a browser are not strictly required, including a `User-Agent` and `Referer` is good practice to better emulate a real client.

## Endpoints

### 1. Login

Authenticates the user and initiates a session by returning a session cookie.

*   **Endpoint:** `POST {base_url}/login`
*   **Body:** `application/x-www-form-urlencoded`

| Parameter  | Type   | Description            |
| :--------- | :----- | :--------------------- |
| `username` | string | Your panel username.   |
| `password` | string | Your panel password.   |

*   **Success Response (200 OK):**
    *   The response will include a `Set-Cookie` header in the HTTP response.
    ```json
    {
        "success": true,
        "msg": "Login successful",
        "obj": null
    }
    ```

### 2. Get All Inbounds

Fetches a list of all configured inbound connections.

*   **Endpoint:** `POST {base_url}/panel/inbound/list`
*   **Body:** None
*   **Success Response (200 OK):**

    ```json
    {
        "success": true,
        "msg": "",
        "obj": [
            {
                "id": 1,
                "up": 40912424334,
                "down": 537061968966,
                "total": 0,
                "remark": "My-First-Inbound",
                "enable": true,
                "expiryTime": 0,
                "listen": "YOUR_SERVER_IP",
                "port": 19271,
                "protocol": "vless",
                "settings": "{\"clients\":[{\"id\":\"user-uuid-goes-here\",\"email\":\"user-email-goes-here\",...}]}",
                "streamSettings": "{\"network\":\"tcp\",\"security\":\"reality\",\"realitySettings\":{\"privateKey\":\"REALITY_PRIVATE_KEY\",\"settings\":{\"publicKey\":\"REALITY_PUBLIC_KEY\"},...}}",
                "tag": "inbound-YOUR_SERVER_IP:19271",
                "sniffing": "{\"enabled\":false,...}"
            }
        ]
    }
    ```

### 3. Get Online Clients

Fetches a list of emails corresponding to currently online clients.

*   **Endpoint:** `POST {base_url}/panel/inbound/onlines`
*   **Body:** None
*   **Success Response (200 OK):**
    ```json
    {
        "success": true,
        "msg": "",
        "obj": [
            "client1-email",
            "online-user@example.com"
        ]
    }
    ```

### 4. Get New Reality Certificate

Generates a new X25519 key pair for use with VLESS Reality.

*   **Endpoint:** `POST {base_url}/server/getNewX25519Cert`
*   **Body:** None
*   **Success Response (200 OK):**
    ```json
    {
        "success": true,
        "msg": "",
        "obj": {
            "privateKey": "a_very_long_private_key_string",
            "publicKey": "a_shorter_public_key_string"
        }
    }
    ```

### 5. Add VLESS Reality Inbound

Creates a new VLESS Reality inbound connection.

*   **Endpoint:** `POST {base_url}/panel/inbound/add`
*   **Body:** `application/x-www-form-urlencoded`

This endpoint requires a complex body where several parameters are URL-encoded JSON strings.

| Parameter        | Description                                                                     | Example Value                                  |
| :--------------- | :------------------------------------------------------------------------------ | :--------------------------------------------- |
| `remark`         | A name or comment for the inbound.                                              | `My-Test-Key`                                  |
| `listen`         | The server IP address to listen on. Leave empty for all IPs.                    | `YOUR_SERVER_IP`                               |
| `port`           | The port to listen on. `0` for a random port.                                   | `48673`                                        |
| `protocol`       | The protocol type.                                                              | `vless`                                        |
| `expiryTime`     | Expiration timestamp in milliseconds. `0` for no expiration.                    | `1753969593148`                                |
| `total`          | Data limit in bytes. `0` for unlimited.                                         | `0`                                            |
| `settings`       | URL-encoded JSON string with client details (UUID, email, etc.).                | (See structure below)                          |
| `streamSettings` | URL-encoded JSON string with transport and security settings (e.g., Reality).   | (See structure below)                          |
| `sniffing`       | URL-encoded JSON string for traffic sniffing settings.                          | `{"enabled":false,...}`                        |

*   **Decoded `settings` Structure:**
    ```json
    {
      "clients": [
        {
          "id": "a-generated-uuid",
          "email": "a-generated-email",
          "enable": true,
          "flow": "",
          "limitIp": 0,
          "totalGB": 0,
          "expiryTime": 0,
          "subId": "a-generated-sub-id"
        }
      ],
      "decryption": "none",
      "fallbacks": []
    }
    ```
*   **Decoded `streamSettings` Structure:**
    ```json
    {
      "network": "tcp",
      "security": "reality",
      "realitySettings": {
        "dest": "example.com:443",
        "serverNames": ["example.com", "www.example.com"],
        "privateKey": "THE_PRIVATE_KEY_FROM_STEP_4",
        "publicKey": "THE_PUBLIC_KEY_FROM_STEP_4",
        "shortIds": ["generated_id1", "generated_id2"],
        "fingerprint": "chrome",
        "spiderX": "/"
      }
    }
    ```

*   **Success Response (200 OK):** The response object contains the full configuration of the newly created inbound.
    ```json
    {
        "success": true,
        "msg": "Create Successfully",
        "obj": {
            "id": 16,
            "remark": "Test test",
            "port": 48673,
            "listen": "YOUR_SERVER_IP",
            "settings": "{\"clients\":[{\"id\":\"60f3a042-7f0c-43a5-a9e6-de76f66703dd\",...}]}",
            "streamSettings": "{\"network\":\"tcp\",\"security\":\"reality\",\"realitySettings\":{...}}",
            // ... and other fields
        }
    }
    ```

## Code Examples

### Python (`requests`)

The `requests.Session` object is perfect for this task as it automatically handles cookies across requests.

```python
import requests

# --- Configuration ---
BASE_URL = "http://127.0.0.1:PORT/YOUR_SECRET_PATH"
USERNAME = "your_username"
PASSWORD = "your_password"

# 1. Create a session that will store cookies
session = requests.Session()

# 2. Login to establish the session
try:
    login_url = f"{BASE_URL}/login"
    login_data = {"username": USERNAME, "password": PASSWORD}
    response = session.post(login_url, data=login_data)
    response.raise_for_status()
    
    if response.json().get("success"):
        print("Login successful!")
        
        # 3. Now you can make other API calls with the same session
        inbounds_url = f"{BASE_URL}/panel/inbound/list"
        inbounds_response = session.post(inbounds_url)
        inbounds_data = inbounds_response.json()
        
        print(f"Successfully fetched {len(inbounds_data.get('obj', []))} inbounds.")
    else:
        print(f"Login failed: {response.json().get('msg')}")

except requests.exceptions.RequestException as e:
    print(f"An error occurred: {e}")
```

### Node.js (`axios`)

In Node.js, `axios` does not handle cookies by default. You need to use a cookie jar helper library like `axios-cookiejar-support` and `tough-cookie`.

First, install the dependencies:
`npm install axios axios-cookiejar-support tough-cookie`

```javascript
const axios = require('axios');
const { HttpsCookieAgent } = require('http-cookie-agent/http');
const tough = require('tough-cookie');

// --- Configuration ---
const BASE_URL = 'http://127.0.0.1:PORT/YOUR_SECRET_PATH';
const USERNAME = 'your_username';
const PASSWORD = 'your_password';

async function main() {
    // 1. Create a cookie jar to store session cookies
    const cookieJar = new tough.CookieJar();

    // 2. Create an axios instance that uses the cookie jar
    const apiClient = axios.create({
        httpAgent: new HttpsCookieAgent({ cookies: { jar: cookieJar } }),
        httpsAgent: new HttpsCookieAgent({ cookies: { jar: cookieJar } }),
    });

    try {
        // 3. Login
        console.log('Attempting to log in...');
        const loginUrl = `${BASE_URL}/login`;
        // For x-www-form-urlencoded, use a URLSearchParams object
        const loginData = new URLSearchParams({
            username: USERNAME,
            password: PASSWORD
        });

        const loginResponse = await apiClient.post(loginUrl, loginData);

        if (loginResponse.data.success) {
            console.log('Login successful!');

            // 4. Make another request. The cookie is sent automatically.
            const inboundsUrl = `${BASE_URL}/panel/inbound/list`;
            const inboundsResponse = await apiClient.post(inboundsUrl);
            
            if (inboundsResponse.data.success) {
                console.log(`Successfully fetched ${inboundsResponse.data.obj.length} inbounds.`);
            } else {
                 console.error('Failed to fetch inbounds:', inboundsResponse.data.msg);
            }
        } else {
            console.error('Login failed:', loginResponse.data.msg);
        }
    } catch (error) {
        console.error('An API error occurred:', error.message);
    }
}

main();
```

### Browser JavaScript (`fetch`)

In a web browser context, `fetch` can automatically handle cookies if the request is to the same origin, or if you specify `credentials: 'include'` for cross-origin requests.

```javascript
// --- Configuration ---
const BASE_URL = 'http://127.0.0.1:PORT/YOUR_SECRET_PATH';
const USERNAME = 'your_username';
const PASSWORD = 'your_password';

async function runApiFlow() {
    try {
        // 1. Login
        console.log('Logging in...');
        const loginResponse = await fetch(`${BASE_URL}/login`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/x-www-form-urlencoded',
            },
            body: new URLSearchParams({
                username: USERNAME,
                password: PASSWORD
            }),
            // This is crucial for the browser to store and send cookies
            credentials: 'include' 
        });

        const loginResult = await loginResponse.json();
        if (!loginResult.success) {
            throw new Error(`Login failed: ${loginResult.msg}`);
        }
        console.log('Login successful!');

        // 2. Fetch inbounds. The browser automatically includes the cookie.
        const inboundsResponse = await fetch(`${BASE_URL}/panel/inbound/list`, {
            method: 'POST',
            credentials: 'include'
        });

        const inboundsResult = await inboundsResponse.json();
        if (inboundsResult.success) {
            console.log(`Found ${inboundsResult.obj.length} inbounds.`);
        } else {
            throw new Error(`Failed to fetch inbounds: ${inboundsResult.msg}`);
        }

    } catch (error) {
        console.error('API Flow Error:', error);
    }
}

runApiFlow();
```

## Generating the VLESS URL

After creating an inbound using the `/panel/inbound/add` endpoint, you can construct a standard `vless://` URL from the `obj` field in the JSON response.

The URL format is:
`vless://{UUID}@{ADDRESS}:{PORT}?{PARAMETERS}#{REMARK}`

Here is how to map the JSON response fields to the URL components:

| URL Part    | JSON Path from `obj`                                            | Example                                         |
| :---------- | :-------------------------------------------------------------- | :---------------------------------------------- |
| `UUID`      | `settings.clients[0].id` (after JSON parsing)                   | `60f3a042-7f0c-43a5-a9e6-de76f66703dd`          |
| `ADDRESS`   | `listen` (or your server's domain name)                         | `YOUR_SERVER_IP`                                |
| `PORT`      | `port`                                                          | `48673`                                         |
| `REMARK`    | `remark` (URL-encoded)                                          | `Test%20test`                              |

**Parameters (in the query string):**

| Parameter | JSON Path from `obj`                                            | Description                                     |
| :-------- | :-------------------------------------------------------------- | :---------------------------------------------- |
| `type`      | `streamSettings.network`                                        | Transport protocol, e.g., `tcp`.                |
| `security`  | `streamSettings.security`                                       | Security layer, e.g., `reality`.                |
| `pbk`       | `streamSettings.realitySettings.settings.publicKey`             | The public key for Reality.                     |
| `fp`        | `streamSettings.realitySettings.settings.fingerprint`           | The browser fingerprint, e.g., `chrome`.        |
| `sni`       | `streamSettings.realitySettings.serverNames[0]`                 | The Server Name Indication to use.              |
| `sid`       | `streamSettings.realitySettings.shortIds[0]`                    | The first short ID from the list.               |
| `spx`       | `streamSettings.realitySettings.settings.spiderX`              | The SpiderX path.                               |