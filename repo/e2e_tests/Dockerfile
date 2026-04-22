FROM mcr.microsoft.com/playwright:v1.44.0-jammy

# Pre-installed browsers in this image; skip re-download.
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

WORKDIR /e2e

COPY package.json ./
RUN npm install --quiet

COPY . .

CMD ["npx", "playwright", "test", "--reporter=line"]
