```bash
certbot certonly --manual --preferred-challenges=dns  --manual-auth-hook=/usr/local/bin/auth-hook -d '*.example.com' --email xxxxxx@qq.com --agree-tos
```