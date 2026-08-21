FROM node:24.14.0-bookworm-slim@sha256:d8e448a56fc63242f70026718378bd4b00f8c82e78d20eefb199224a4d8e33d8

WORKDIR /app
COPY --chown=node:node web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY --chown=node:node web/ ./
RUN npm run build
USER node
EXPOSE 5173
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
