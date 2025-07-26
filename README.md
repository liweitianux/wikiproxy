Wikipedia Proxy
===============

A simple Wikipedia proxy in Go.

Features
--------
* Easy to deploy: one single domain to proxy the Wikipedia site.

  For example, domain `en.wikiproxy.org` is used to proxy the English
  Wikipedia site `en.wikipedia.org`; all the links to other `*.wikipedia.org`
  or `*.wikimedia.org` domains will be translated to their own path prefixes
  in the hosting domain `en.wikiproxy.org`.

* Simple authenticator: help protect from crawling and/or abusing.

* Proxy support to help access Wikipedia.

  For example, a SOCKS5 proxy `socks5h://127.0.0.1:1080` can be used to access
  the Wikipedia.

* Minimal and self-contained.

* Good unit tests.

Deployment
----------
OS: Debian Linux (12/bookworm)

1. Build the binary and copy to `/opt/wikiproxy/wikiproxy`

2. Create the config file at `/opt/wikiproxy/wikiproxy.toml`.

   In particular, set the `proxy` for accessing Wikipedia if required.

3. Install the `wikiproxy.service` and start:

   ```
   $ sudo cp wikiproxy.service /etc/systemd/system/
   $ sudo systemctl daemon-reload
   $ sudo systemctl enable --now wikiproxy.service
   ```

4. Configure Nginx with [acmetool](https://github.com/hlandau/acmetool)
   for TLS certificate:

   ```
   server {
       listen      80;
       listen [::]:80;
       server_name en.wikiproxy.org zh.wikiproxy.org;
       location / {
           return 301 https://$host$request_uri;                                     
       }   
       include /etc/nginx/snippets/acmetool.conf;
   }
   server {
       listen      443 ssl http2;
       listen [::]:443 ssl http2;
       server_name wikiproxy.org zh.wikiproxy.org;
       ssl_certificate /var/lib/acme/live/en.wikiproxy.org/fullchain;
       ssl_certificate_key /var/lib/acme/live/en.wikiproxy.org/privkey;
       ssl_protocols TLSv1.2 TLSv1.3;
       location / {
           proxy_http_version 1.1;
           proxy_set_header Host $http_host;
           proxy_set_header X-Forwarded-Proto $scheme;
           proxy_set_header X-Forwarded-Host $http_host;
           proxy_pass http://127.0.0.1:2012;
       }
   }
   ```

License
-------
MIT License
