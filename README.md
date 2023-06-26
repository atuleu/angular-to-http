# angular-to-http ![GitHub tag (latest SemVer pre-release)](https://img.shields.io/github/v/tag/atuleu/angular-to-http?style=flat-square&label=Version) ![coverage badge](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/atuleu/eda3658d1543e5a68a2070a33ba73ddd/raw/coverage.json&style=flat-square) ![report badge](https://goreportcard.com/badge/github.com/atuleu/angular-to-http?style=flat-square) ![GitHub](https://img.shields.io/github/license/atuleu/angular-to-http?style=flat-square&color=008b8b)


Lightweight zero-configuration SPA HTTP server that do not compromise on security. Serves an Angular SPA bundle on a given http port. This is strongly inspired from https://github.com/devforth/spa-to-http . So you will not have an excuse to leave a `Content-Security-Policy: style-src 'unsafe-inline'` 

> :warning: This is currently a first alpha release, use it as your own risks on production.

## Benefit

* Zero-configuration in Docker compared to a classic http server (nginx, Apache)
* Produced images are small (an order of magnitude compared to Nginx)
* Cache limit are expressed in memory size footprint of assets, not number of cached data. You can reliably target a wanted memory usage with your containers.
* Do not sacrifice security. Configure a nonce on your `index.html` files if they contain a magic pattern.
* Supports file compression (go green on Lighthouse)

## Hello World and Usage

Create a `Dockerfile` in you spa directory (near `package.json`):

```Dockerfile
FROM node:20.2-alpine as build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

ADD . .
RUN ng build

FROM ghcr.io/atuleu/angular-to-http:latest

WORKDIR /srv
COPY --from=build /app/dist/app-name/ .

CMD [ '/app/angular-to-http', '/srv']
```

## CSP Nonce generation

This is lilely where a very minimal configuration should be done. If a nonceable file (default '/index.html') contains the special token `ng_csp_nonced`, it:
 * Will become templated and `ng_csp_nonced` will be replaced with `ngCspNonce="randomNonce"`
 * When serving the file, a default CSP header will be added to the response with the Value `Content-Security-Policy: default-src 'self'; style-src 'self' 'nonce-randomNonce'; script-src 'self' 'nonce-randomNonce'`
 * `randomNonce` will be a base64 cryptographic-strong 32 bytes random number generated for each request to a nonced files.

So to enable the default CSP described [here](https://angular.io/guide/security#content-security-policy) for your angular app, one would simply replace in its `src/index.html` its `<app-root></app-root>` with `<app-root ng_csp_nonced></app-root>`.

## Cache-Control strategies

Any served files will fall into three categories regarding cache-control.

* Nonced files, which are unique for each request, will use a `no-store` configuration.
* Versionned files, i.e. containing an hexadecimal hash or a version number ( `style.abcdef.css` or `logo.v123.png`) will be served with `max-age=31536000; immutable`
* Other files, will be served with `max-age=0; must-revalidate` by default. max-age could manually be increased. All files will be served with `Last-Modified` to the Modtime of the file for revalidation.

## Options

```
$ sudo docker run ghcr.io/atuleu/angular-to-http:latest --help
Usage:
  angular-to-http [OPTIONS] [directory]

Application Options:
  -a, --address=                 address to listen to (default: 0.0.0.0)
  -p, --port=                    port to listen on (default: 80)

compression:
      --compression.no-gzip      disable gzip compression
      --compression.no-deflate   disable deflate compression
      --compression.no-brotli    disable brotli compression
      --compression.threshold=   file size threshold to enable compression (default: 1k)

cache-control:
      --cache.max-age=           cache max age on cacheable file, i.e. files with an hash (default: 1 week)
  -N, --cache.no-store=          additional files to set Cache-Control: no-store
  -m, --cache.max-size=          maximal size of the cache in bytes (default: 50M)
      --cache.no-in-memory-root  by default all cacheable root file (non-asset files) are always cached in memory, this option
                                 disable it and put it in the LRU cache

csp-nonce:
      --csp.nonce-disable        Disable CSP Nonce generation
  -O, --csp.nonced=              list of nonced file (default: /index.html)
      --csp.policy=              CSP to use (default: default-src 'self'; style-src 'self' 'nonce-CSP_NONCE'; script-src 'self'
                                 'nonce-CSP_NONCE')

Help Options:
  -h, --help                     Show this help message

Arguments:
  directory:                     directory to serve (default: '.')


```



