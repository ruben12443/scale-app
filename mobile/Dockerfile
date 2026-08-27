# Builds the vendor app as a static PWA build (see the "Progressive Web App"
# section in README.md) and serves it over plain nginx. This is for local/dev
# distribution to test devices on the same network — see README.md for what
# a real production deployment (HTTPS, a real domain, a designed icon set)
# still needs beyond this.
FROM ghcr.io/cirruslabs/flutter:stable AS build
WORKDIR /app
COPY . .

# Defaults match config.dart's own defaults (the local docker-compose
# stack); override at build time for a different backend/issuer.
# ZITADEL_REDIRECT_URL isn't here: it's the *native* redirect scheme
# (config.dart only reads it when not running on web), so it doesn't apply
# to this web-only build — the web redirect (ZITADEL_WEB_REDIRECT_PATH)
# resolves against whatever origin nginx ends up serving this from, so it
# doesn't need to be baked in here either.
ARG CORE_API_BASE_URL=http://localhost:8081
ARG SCALE_GATEWAY_BASE_URL=http://localhost:8082
ARG ZITADEL_ISSUER=http://localhost:8080
ARG ZITADEL_CLIENT_ID

RUN flutter pub get && flutter build web \
    --dart-define=CORE_API_BASE_URL=$CORE_API_BASE_URL \
    --dart-define=SCALE_GATEWAY_BASE_URL=$SCALE_GATEWAY_BASE_URL \
    --dart-define=ZITADEL_ISSUER=$ZITADEL_ISSUER \
    --dart-define=ZITADEL_CLIENT_ID=$ZITADEL_CLIENT_ID

FROM nginx:1.27-alpine
COPY --from=build /app/build/web /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
