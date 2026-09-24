FROM node:20-alpine AS base
WORKDIR /app

FROM base AS builder
# Change context to root, so we copy from the correct relative path
COPY dashboard/package*.json ./
RUN npm install
COPY dashboard/ .
RUN npm run build

FROM base AS runner
ENV NODE_ENV=production
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
EXPOSE 3000
ENV PORT=3000
CMD ["node", "server.js"]