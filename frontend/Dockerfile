# Static frontend build served by nginx (see docker-compose.prod.yml).
# VITE_API_URL is baked in at build time: pass it as a build arg.
FROM node:20-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
ARG VITE_API_URL=http://localhost:8080
ENV VITE_API_URL=$VITE_API_URL
ARG VITE_ENABLE_LANDING=false
ARG VITE_ENABLE_ACTIVE_SESSIONS=false
ARG VITE_REQUIRE_LOGIN_ON_ENTRY=false
ARG VITE_ENABLE_ANALYTICS=true
ENV VITE_ENABLE_LANDING=$VITE_ENABLE_LANDING
ENV VITE_ENABLE_ACTIVE_SESSIONS=$VITE_ENABLE_ACTIVE_SESSIONS
ENV VITE_REQUIRE_LOGIN_ON_ENTRY=$VITE_REQUIRE_LOGIN_ON_ENTRY
ENV VITE_ENABLE_ANALYTICS=$VITE_ENABLE_ANALYTICS
RUN npm run build

FROM nginx:1.27-alpine
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
