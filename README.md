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

License
-------
MIT License
