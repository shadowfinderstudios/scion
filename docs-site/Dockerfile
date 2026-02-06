# Build stage
FROM node:20-slim AS build

# Install dependencies for building, including D2 for diagrams
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    make \
    && curl -fsSL https://d2lang.com/install.sh | sh -s -- \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy package files and install dependencies
COPY package*.json ./
RUN npm install

# Copy the rest of the source code
COPY . .

# Build the site
RUN npm run build

# Runtime stage - use unprivileged nginx for security
FROM nginxinc/nginx-unprivileged:stable-alpine

# Copy the built site from the build stage
COPY --from=build /app/dist /usr/share/nginx/html

# Nginx unprivileged image listens on 8080 by default
EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]