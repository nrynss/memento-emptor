# Shopper frontend image (T3.8, deployment probe-gw).
#
# F1: this image stays ROOT on purpose. Production routes the frontend
# hostname to port 80, and binding :80 needs root — the T3.7 probe proved
# this exact shape (root + EXPOSE 80) serves 200 anonymously. Do NOT add
# a USER switch here; the reference agent's Dockerfile (USER bun,
# outside this task's owns) is the file that would need the change if the
# page ever moves onto that deployment.
FROM oven/bun:1

WORKDIR /app

COPY package.json ./
RUN bun install

COPY . .

EXPOSE 80

CMD ["bun", "run", "server/index.ts"]
