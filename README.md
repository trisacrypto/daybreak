# Daybreak

Management system for counterparty email addresses used for the Sunrise protocol.

## Local Development

Local development runs the server behind [Caddy](https://caddyserver.com/) so that the
app is served over TLS at `https://localhost`, and uses [Air](https://github.com/air-verse/air)
to rebuild and restart the server on every change.

1. Create your environment file from the template and fill in the required values:

   ```
   cp .env.template .env
   ```

   `.env` is gitignored and is watched by Air, so edits to it trigger a restart.

2. Install the tools (once):

   ```
   brew install caddy
   go install github.com/air-verse/air@latest
   ```

3. In one terminal, start Caddy with the repository `Caddyfile`. It terminates TLS on
   `https://localhost` and reverse proxies to the server on `localhost:8000`:

   ```
   caddy run --config Caddyfile
   ```

   The first run asks for permission to install Caddy's local CA so the browser trusts
   the certificate (you can also run `caddy trust` ahead of time).

4. In another terminal, start the server with Air using the `.air.toml` config:

   ```
   air
   ```

   Air runs `go generate ./...`, builds `./cmd/daybreak`, and runs `daybreak serve` on
   `:8000`, rebuilding whenever Go, template, static, or `.env` files change.

5. Open [https://localhost](https://localhost).

> **Note:** The Google OAuth credentials (`GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`)
> are configured for local development over TLS: the authorized redirect URI is
> `https://localhost/auth/callback`. Make sure `DAYBREAK_ORIGIN` (and
> `DAYBREAK_ALLOW_ORIGINS`) in your `.env` are set to `https://localhost` and that you
> access the app through Caddy rather than `http://localhost:8000`, otherwise the OAuth
> redirect will be rejected.
