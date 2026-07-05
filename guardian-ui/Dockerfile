# syntax=docker/dockerfile:1

FROM node:20-alpine AS builder
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build -- --configuration production

FROM nginx:1.27-alpine
COPY --from=builder /src/dist/guardian-ui/browser /usr/share/nginx/html
COPY nginx.conf.template /etc/nginx/templates/default.conf.template

EXPOSE 80
