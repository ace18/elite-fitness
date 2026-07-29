# ELITE — un'immagine sola: il binario Go serve sia l'API sia la SPA compilata.
# Un'origine, un container, un hostname da esporre nel tunnel.

# --- 1. frontend ------------------------------------------------------------
FROM node:24-alpine AS web

WORKDIR /src
RUN corepack enable
# Senza CI=true pnpm si aspetta un TTY quando decide di rifare node_modules,
# e in build fallisce invece di procedere.
ENV CI=true

# Un COPY solo: copiare prima i manifest per l'install e poi il resto rimetteva
# lockfile e package.json sopra l'albero già installato, e pnpm li vedeva più
# recenti di node_modules — quindi voleva reinstallare comunque. L'install qui
# dura un paio di secondi: non vale un layer di cache che si invalida da solo.
COPY web/ ./
RUN pnpm install --frozen-lockfile

# VITE_API_URL resta volutamente non impostata: in build di produzione api.js
# usa URL relativi, perché l'API sta sulla stessa origine.
RUN pnpm build

# --- 2. backend -------------------------------------------------------------
FROM golang:1.24-alpine AS api

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

# Solo i sorgenti Go, non `COPY . .`: così una modifica al frontend non
# invalida la cache di questo stage.
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# CGO spento: binario statico, così l'immagine finale non ha bisogno di libc.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- 3. runtime -------------------------------------------------------------
FROM alpine:3.20

# ca-certificates per le chiamate HTTPS in uscita (Resend, Anthropic, OAuth).
# tzdata perché l'app decide "che giorno è oggi" con time.Now(): senza fuso
# orario il container sta in UTC e fra mezzanotte e le 2 mostrerebbe agli utenti
# italiani l'allenamento del giorno prima.
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Europe/Rome

# Utente non privilegiato: il processo non ha motivo di girare da root.
RUN adduser -D -u 10001 elite
USER elite

WORKDIR /app
COPY --from=api /out/server /app/server
COPY --from=web /src/build /app/static

ENV STATIC_DIR=/app/static \
    PORT=8080
EXPOSE 8080

# Coolify interroga /healthz per sapere se il container è vivo.
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

CMD ["/app/server"]
