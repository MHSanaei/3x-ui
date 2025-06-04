# Stage 1: Build the Next.js application
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN corepack enable
RUN yarn install --frozen-lockfile --network-timeout 600000

COPY . .
RUN yarn build

# Stage 2: Production environment
FROM node:20-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production

COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package.json ./package.json
COPY --from=builder /app/yarn.lock ./yarn.lock

RUN corepack enable
RUN yarn install --production --frozen-lockfile --network-timeout 600000

EXPOSE 3000

CMD ["yarn", "start"]
