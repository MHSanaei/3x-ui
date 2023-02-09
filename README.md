# 3x-ui
> **Disclaimer: This project is only for personal learning and communication, please do not use it for illegal purposes, please do not use it in a production environment**

# Install & Upgrade

```
bash <(curl -Ls https://raw.githubusercontent.com/mhsanaei/3x-ui/master/install.sh)
```

# SSL
```
apt-get install certbot -y
certbot certonly --standalone --agree-tos --register-unsafely-without-email -d yourdomain.com
certbot renew --dry-run
```

# Default settings

- Port: 2053
- user: admin
- password: admin
- database path: /etc/x-ui/x-ui.db

before you set ssl on settings
- http:// ip or domain:2053/xui

After you set ssl on settings 
- https://yourdomain:2053/xui

# suggestion system
- Ubuntu 20.04+

# pic

![1](https://raw.githubusercontent.com/MHSanaei/3x-ui/main/media/1.png)
![2](https://raw.githubusercontent.com/MHSanaei/3x-ui/main/media/2.png)
![3](https://raw.githubusercontent.com/MHSanaei/3x-ui/main/media/3.png)
![4](https://raw.githubusercontent.com/MHSanaei/3x-ui/main/media/4.png)



