# Mattermost (dev fork)

This repository is a Mattermost development fork. It contains the Go server and React
web client, plus a root `Makefile` that wraps the development workflow:

- start/stop the local dev environment (server + web client + Docker services)
- create a local admin and install the Agents plugin
- build a production runtime image for **linux/amd64** and push it to GHCR

The upstream project documentation is still the reference for Mattermost features and
configuration: <https://docs.mattermost.com/>.

## Requirements

| Tool | Version / notes |
|---|---|
| Go | 1.26.7+ (`server/.go-version`) |
| Node | **24.11.1** and npm **11.6.2** — exact pins, `webapp/.npmrc` uses `engine-strict=true` |
| Docker | Docker Engine with Compose v2 and `docker buildx` |
| Other | `git`, `make`, `curl`, `python3`, `openssl` |

The root `Makefile` automatically prefers a locally installed Node at
`~/.local/share/mattermost/node/bin`. To install that exact toolchain on Linux:

```bash
NODE_VERSION=24.11.1
case "$(uname -m)" in
  x86_64) NODE_ARCH=x64 ;;
  aarch64|arm64) NODE_ARCH=arm64 ;;
  *) echo "unsupported arch"; exit 1 ;;
esac

mkdir -p "$HOME/.local/share/mattermost"
curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODE_ARCH}.tar.xz" \
  | tar -xJ -C "$HOME/.local/share/mattermost"
ln -sfn "$HOME/.local/share/mattermost/node-v${NODE_VERSION}-linux-${NODE_ARCH}" \
  "$HOME/.local/share/mattermost/node"
```

Alternatively install Node 24.11.1 with `nvm`, `fnm`, or `mise`; do not use a newer
Node with this webapp, npm will refuse to install (`EBADENGINE`).

## Quick start

```bash
cp .env.example .env
# edit .env at least DEV_ADMIN_PASSWORD; optionally OIDC and LLM settings

make deps      # start postgres (pgvector), redis, inbucket
make start     # start server + web client in the background
make status    # wait until the app reports "running"
make admin     # create DEV_ADMIN_USERNAME / DEV_ADMIN_PASSWORD
make plugin-install   # optional: build & deploy the Agents plugin
```

Open `MM_SERVICESETTINGS_SITEURL` from `.env` (default `http://localhost:8065`).

The first `make start` is slow: it runs `npm install` for the webapp and compiles the
Go server. Follow progress with:

```bash
make logs       # app + web
make logs-app
make logs-web
```

## Configuration (`.env`)

`.env` is gitignored and sourced by the Makefile with `set -a`, so every value reaches
the server and the sub-Makefiles. `.env.example` documents all supported variables.

Key values:

| Variable | Purpose |
|---|---|
| `MM_SERVICESETTINGS_SITEURL` | Public URL of the dev server (`http://localhost:8065`) |
| `MM_SQLSETTINGS_DATASOURCE` | Postgres DSN used by the server |
| `ENABLED_DOCKER_SERVICES` | Docker services started by `make deps` / `make start` |
| `MM_USE_PGVECTOR` | Use the `pgvector/pgvector` Postgres image |
| `MM_PLUGINSETTINGS_ENABLEUPLOADS` | Allow plugin deployment from the API |
| `DEV_ADMIN_*` | Local admin created by `make admin` |
| `MM_OPENIDSETTINGS_*` | External OpenID Connect provider |
| `DEV_LLM_*` | Notes for seeding an OpenAI-compatible service (the plugin stores LLM config in its own config, not in `.env`) |

Notes:

- If host port `5432` is already used by another project, add a gitignored
  `server/docker-compose.override.yaml` that publishes Postgres on another port, and
  update the DSN in `.env`. Example:

  ```yaml
  services:
    postgres:
      ports: !override
        - "5433:5432"
  ```

- OpenID Connect requires an Enterprise Edition build plus the private enterprise
  module. The default Team/sourceavailable build does not include the OpenID provider,
  even if the `MM_OPENIDSETTINGS_*` variables are set.
- The Agents plugin is configured in **System Console → Plugins → Agents**
  (`Add an AI Service`). `DEV_LLM_*` in `.env` is only a convenience for local
  tooling; the plugin does not read it directly.

## Development server

```bash
make start              # app + web, background
make status             # app/web state and URLs
make stop               # stop web + app; Docker services stay up
make stop-all           # also stop Docker services
make restart            # stop + start both
make restart-app        # rebuild/restart the Go server
make restart-web        # restart the web client
```

The web client runs in webpack watch mode and rebuilds automatically when files under
`webapp/` change. The Go server is not watched; restart it after changing Go code:

```bash
make restart-app
```

Web modes:

```bash
make start-web                        # webpack --watch, served by the app on :8065
make start-web WEB_MODE=dev-server    # webpack-dev-server + HMR on :9005
```

`start-app` / `start-web` only start a process if it is not already running. They do
not rebuild an already-running server; use `restart-*` for that.

### Admin and plugins

```bash
make admin            # creates DEV_ADMIN_USERNAME with DEV_ADMIN_PASSWORD
make plugin-install   # clones PLUGIN_GIT_URL, builds and deploys the Agents plugin
```

`make admin` uses `mmctl --local`, so the app must be running and
`MM_SERVICESETTINGS_ENABLELOCALMODE=true` must be set (default in `.env.example`).
`make plugin-install` deploys with `MM_SERVICESETTINGS_ENABLEDEVELOPER=true` and uses
`DEV_ADMIN_USERNAME` / `DEV_ADMIN_PASSWORD`.

Overrides:

```bash
make plugin-install PLUGIN_DIR=../my-plugin
make plugin-install PLUGIN_GIT_URL=https://github.com/org/my-plugin.git
```

### Building the server manually

`make start-app` uses `go run` and compiles from source. To produce binaries without
starting the server:

```bash
cd server
make build-linux-amd64
# amd64 host:  server/bin/mattermost, server/bin/mmctl
# arm64 host:  server/bin/linux_amd64/mattermost, server/bin/linux_amd64/mmctl
```

For the web client:

```bash
make -C webapp dist   # production build into webapp/channels/dist
```

## Build a production image

```bash
make image
```

Defaults:

| Variable | Default | Meaning |
|---|---|---|
| `IMAGE` | `ghcr.io/<git-remote-owner>/mattermost-team-edition` | Image repository |
| `IMAGE_TAG` | `dev` | Image tag |
| `PLATFORM` | `linux/amd64` | Target platform |
| `IMAGE_BUILD_CLIENT` | `true` | Rebuild the production web client first |
| `IMAGE_HTTP_PORT` | `18765` | Temporary local port used to feed the package to Docker |

What `make image` does:

1. cross-compiles the server for `linux/amd64` (works on arm64 hosts);
2. builds the production web client unless `IMAGE_BUILD_CLIENT=false`;
3. packages Mattermost (`server/dist/mattermost-team-linux-amd64.tar.gz`);
4. builds `server/build/Dockerfile` with `docker buildx --platform linux/amd64` and
   loads the image locally.

Examples:

```bash
# custom tag and repository
make image IMAGE=ghcr.io/ba0f3/mattermost-team-edition IMAGE_TAG=12.1.0

# reuse the existing dev webapp dist (faster, larger image)
make image IMAGE_BUILD_CLIENT=false

# build for another platform
make image PLATFORM=linux/arm64
```

Push to GHCR (requires a token with `write:packages`):

```bash
docker login ghcr.io
make image-push IMAGE=ghcr.io/ba0f3/mattermost-team-edition IMAGE_TAG=12.1.0
```

Notes:

- Building `linux/amd64` on an arm64 Linux host needs binfmt/QEMU for the Ubuntu
  builder stage:

  ```bash
  docker run --privileged --rm tonistiigi/binfmt --install amd64
  ```

  Docker Desktop (Apple Silicon) includes this automatically.
- `IMAGE_BUILD_CLIENT=true` replaces `webapp/channels/dist` with a production build.
  If the dev server is running, restart the web client afterwards:
  `make restart-web`.
- The image does not bundle prepackaged plugins (`package-linux-amd64` runs with an
  empty `PLUGIN_PACKAGES`); install the Agents plugin after deployment.
- The runtime image runs as the `mattermost` user and exposes ports `8065`, `8067`,
  and `8074`.

To run the image, set `MATTERMOST_IMAGE` in the production Compose file, for example:

```bash
export MATTERMOST_IMAGE=ghcr.io/ba0f3/mattermost-team-edition:12.1.0
export MM_SERVICESETTINGS_SITEURL=https://chat.example.com
docker compose -f deploy/production/docker-compose.yml up -d
```

## Make targets

| Target | Description |
|---|---|
| `make help` | List all targets |
| `make deps` | Start Docker services only |
| `make start` / `make stop` / `make restart` | Manage app + web |
| `make start-app` / `stop-app` / `restart-app` | Manage the Go server |
| `make start-web` / `stop-web` / `restart-web` | Manage the web client |
| `make status` | Show app/web state and URLs |
| `make logs` / `logs-app` / `logs-web` | Follow logs |
| `make admin` | Create the local admin user |
| `make plugin-install` | Build and deploy the Agents plugin |
| `make stop-all` | Stop app, web and Docker services |
| `make image` | Build the runtime image |
| `make image-push` | Push the runtime image |

## Troubleshooting

**`npm install` fails with `EBADENGINE`** — install Node 24.11.1 exactly (see
Requirements). The Makefile prefers `~/.local/share/mattermost/node/bin`.

**`server/client/root.html: no such file or directory`** — the web client has not been
built yet. Check `make logs-web`; the first build takes a few minutes. If it is already
built, restart the server so the client symlink is refreshed:
`make restart-app`.

**Plugin menus / App bar disappear** — plugin webapp bundles under
`webapp/channels/dist/plugins` can be removed by a webapp rebuild. Restart the app to
copy them again:

```bash
make restart-app
curl -o /dev/null -w '%{http_code}\n' \
  http://localhost:8065/static/plugins/mattermost-ai/<bundle>.js
```

**Port already in use** — change the host port in
`server/docker-compose.override.yaml` (Postgres) or override `MM_SERVICESETTINGS_*`
ports in `.env`.

## License

See [LICENSE.txt](LICENSE.txt). Upstream Mattermost resources:

- [Mattermost documentation](https://docs.mattermost.com/)
- [Developer documentation](https://developers.mattermost.com/)
- [Contributing](CONTRIBUTING.md)
