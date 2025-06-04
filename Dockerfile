# Stage 1: Build the Next.js application
FROM node:20-alpine AS builder

# Set working directory
WORKDIR /app

# Install dependencies
# Copy package.json and yarn.lock (or package-lock.json if using npm)
COPY package.json yarn.lock ./
# Ensure corepack is enabled to use yarn specified in package.json
RUN corepack enable
RUN yarn install --frozen-lockfile --network-timeout 600000

# Copy the rest of the application source code
COPY . .

# Build the Next.js application
# NEXT_PUBLIC_API_BASE_URL can be set here if it's fixed,
# or passed as an ARG during docker build, or as an ENV var at runtime.
# For flexibility, runtime ENV var is often preferred.
# ARG NEXT_PUBLIC_API_BASE_URL
# ENV NEXT_PUBLIC_API_BASE_URL=${NEXT_PUBLIC_API_BASE_URL}
RUN yarn build

# Stage 2: Production environment
FROM node:20-alpine AS runner

WORKDIR /app

# Set environment variables
# ENV NODE_ENV=production # Already set by `next start`
# NEXT_PUBLIC_API_BASE_URL will be set at runtime via docker-compose or run command
# ENV PORT=3000 # Next.js default port is 3000, can be overridden

# Copy built assets from the builder stage
# This includes the .next folder (production build) and public folder.
# For a standard Next.js build (not standalone or static export),
# we also need node_modules and package.json to run `next start`.
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public
COPY --from=builder /app/package.json ./package.json
# If yarn.lock is needed for `yarn start` with specific versions, copy it too.
# Usually for `yarn start` just package.json and production node_modules are needed.
# For yarn, yarn.lock is good practice to ensure consistent prod dependencies if any are direct.
COPY --from=builder /app/yarn.lock ./

# Install production dependencies only
# Ensure corepack is enabled
RUN corepack enable
RUN yarn install --production --frozen-lockfile --network-timeout 600000

# Expose port 3000 (default for Next.js)
EXPOSE 3000

# The "start" script in package.json runs "next start"
# This will serve the application from the .next folder.
CMD ["yarn", "start"]
