# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm install --production

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Production stage
FROM node:20-alpine

WORKDIR /app

# Copy built files from builder
COPY --from=builder /app/dist ./dist

# Install serve globally
RUN npm install -g serve

# Expose port
EXPOSE 3000

# Start production server
CMD ["serve", "-s", "dist", "-l", "3000"] 