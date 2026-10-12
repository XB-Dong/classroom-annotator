# Copy the locally built Vite dist; no cloud Node pull.
ARG NGINX_IMAGE=nginx:1.28.0@sha256:3b03dfd827515a07ccf4946000f0bbdd2a4fb6aca7499249fcb6b1f9726823d6
FROM ${NGINX_IMAGE}
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY frontend/dist/ /usr/share/nginx/html/
EXPOSE 80
