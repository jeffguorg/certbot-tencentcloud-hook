# quick start

## help

```bash
auth-hook --help
```

## certbot auth hook script generation

```bash
# you can set the options with environment variables
AUTHHOOK_ROOT_DOMAIN=example.com
auth-hook --wrap-self [options] | sudo tee /opt/auth-hooks/$AUTHHOOK_ROOT_DOMAIN
sudo chmod +700 /opt/auth-hooks/$AUTHHOOK_ROOT_DOMAIN
# edit settings, like secrets
```

## use the script

```bash
sudo certbot certonly --manual --preferred-challenges=dns  --manual-auth-hook=/opt/auth-hooks/$AUTHHOOK_ROOT_DOMAIN -d "*.$AUTHHOOK_ROOT_DOMAIN" --email xxxxxx@example.com --agree-tos
```