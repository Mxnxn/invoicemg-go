#!/bin/sh
# Turns HTTPS on, but only once there is a certificate to turn it on with.
#
# nginx refuses to start when ssl_certificate points at a file that does not exist. On a fresh
# droplet there is no certificate yet, and there cannot be one until certbot can answer an
# HTTP-01 challenge over port 80 - which needs nginx running. Shipping the 443 block
# unconditionally deadlocks that: the container crash-loops and the challenge is never served.
#
# So the site comes up on HTTP first, certbot obtains the certificate through the
# .well-known location, and the next start of this container finds it and enables TLS.
#
# The port-80 config is REPLACED rather than extended when that happens: nginx rejects two
# `location /` blocks in one server, so the redirect cannot live alongside the SPA config.
#
# nginx:alpine runs everything in /docker-entrypoint.d before starting, so this needs no
# custom command.
set -e

DOMAIN="${CERT_DOMAIN:-invoicemg.in}"
LIVE="/etc/letsencrypt/live/$DOMAIN"

if [ -f "$LIVE/fullchain.pem" ] && [ -f "$LIVE/privkey.pem" ]; then
    echo "[tls] certificate found for $DOMAIN - enabling HTTPS"
    cp /etc/nginx/ssl.conf.disabled /etc/nginx/conf.d/ssl.conf
    cp /etc/nginx/http-redirect.conf.disabled /etc/nginx/conf.d/default.conf
else
    echo "[tls] no certificate for $DOMAIN yet - serving HTTP only"
    echo "[tls] obtain one, then restart this container:"
    echo "[tls]   docker compose -f docker-compose.prod.yml run --rm certbot"
    echo "[tls]   docker compose -f docker-compose.prod.yml restart web"
    # Restore the plain-HTTP config and drop any 443 block a previous run left behind. A
    # revoked or deleted certificate would otherwise leave this container redirecting to an
    # HTTPS listener that no longer exists - the whole site unreachable, from a file nobody
    # remembers writing.
    cp /etc/nginx/http-serve.conf.disabled /etc/nginx/conf.d/default.conf
    rm -f /etc/nginx/conf.d/ssl.conf
fi
