# Native mobile login through the OIDC plugin

This fork connects the official Mattermost mobile app's OpenID login entry point
to the `mattermost-oidc` plugin. The server handles the two compatibility routes
previously handled by the plugin repository's standalone `mobile-bridge`.

The plugin still performs OIDC discovery, PKCE, token verification, account
linking, session creation, and the final mobile deep-link handoff. This change
does not require a separate bridge process or special reverse-proxy rules.

## Requirements and provider selection

- Install a build of
  [r2d-ai/mattermost-oidc-plugin](https://github.com/r2d-ai/mattermost-oidc-plugin)
  with the mobile redirect support, under plugin ID `mattermost-oidc`.
- Configure its issuer, client credentials, and OIDC settings. Enable the plugin
  itself and its `Enable` setting. Confirm web SSO works first.
- Set Mattermost `ServiceSettings.SiteURL` to its public URL, including any path
  prefix. The existing IdP redirect URI remains
  `<SiteURL>/plugins/mattermost-oidc/oauth2/callback`.
- Leave the built-in `OpenIdSettings.Enable` disabled when using this plugin.
  An enabled built-in OpenID configuration takes precedence over the plugin
  integration; other built-in SSO providers are unchanged.

There are no additional client secrets, settings, or credentials to configure
for this mobile integration. Button text and color come from the plugin's
effective settings, including manifest defaults.

## Request flow

1. The app requests `/api/v4/config/client`. For User-Agents containing
   `Mattermost Mobile/`, the server advertises `EnableSignUpWithOpenId="true"`
   only when the plugin is enabled and running. It also adds the plugin's
   `OpenIdButtonText` and `OpenIdButtonColor` values. The existing distinction
   between anonymous and authenticated client config is preserved.
2. The app opens `/oauth/openid/mobile_login?redirect_to=...` in its authentication
   browser. This route does not depend on the browser's User-Agent.
3. The server validates the callback and redirects to the plugin's
   `/oauth2/connect` endpoint, preserving it as `mobile_redirect`.
4. The plugin performs OIDC login and returns the mobile session through the
   `mmauth://callback` or `mmauthbeta://callback` deep link.

Only the three public button fields are added to the mobile response. Plugin
credentials are never serialized into client config. Web and desktop clients
continue to receive their original authentication settings.

If the plugin stops or is disabled, new client-config requests stop advertising
its mobile button. A pending login that already selected the plugin receives an
unavailable error instead of being redirected to an inactive plugin. A running
plugin does not imply that its identity provider is reachable; OIDC failures are
still handled by the plugin.

The compatibility overlay applies to the client-config HTTP endpoint, as the
standalone bridge did. It does not rewrite the global `config_changed` WebSocket
event payload.

## Deployment and removal of the bridge

1. Build and deploy the patched Mattermost server with the OIDC plugin installed.
2. Route `/api/v4/config/client` and `/oauth/openid/mobile_login` directly to
   Mattermost, together with the other normal application routes. Remove body
   substitutions and bridge-specific redirects from the ingress.
3. Verify the checks below before removing the old bridge container.

Keep the usual TLS, forwarding-header, upload, and WebSocket proxy configuration.
No changes to the native app or the IdP callback are needed for this flow.

## Verification

Set `MM_SITE_URL` to the public Mattermost URL, including a subpath when used:

```bash
MM_SITE_URL=https://chat.example.com

curl -fsS -A 'Mattermost Mobile/2.42.0 (Android)' \
  "$MM_SITE_URL/api/v4/config/client" | \
  jq '{EnableSignUpWithOpenId, OpenIdButtonText, OpenIdButtonColor}'

curl -fsS -A 'Mozilla/5.0' "$MM_SITE_URL/api/v4/config/client" | \
  jq '{EnableSignUpWithOpenId, OpenIdButtonText, OpenIdButtonColor}'

curl -sS -D - -o /dev/null \
  "$MM_SITE_URL/oauth/openid/mobile_login?redirect_to=mmauth%3A%2F%2Fcallback"
```

For a Team server using only this plugin, the first response should enable
OpenID, the second should retain the native disabled value, and the third
should redirect to the plugin connect endpoint on the correct SiteURL/subpath.

Check both Android and iOS: add the server, select SSO, authenticate at the IdP,
return to the app, reconnect, and log out. Also check the beta callback if using
the beta app, and verify that disabling the plugin removes the advertised
button on a fresh client-config request. Missing or invalid callback URLs must
return an error rather than redirecting.

Focused automated checks, from `server/` with the standard Go workspace using
both `.` and `./public`:

```bash
go test -race ./channels/app/oidcmobile
go test -short ./channels/app -run '^TestGetMobileOIDCPluginConfig'
go test -short ./channels/web -run '^TestRedirectMobileLoginToOIDCPlugin$'
go build ./channels/api4 ./channels/web
```

The focused tests do not replace a real-device OIDC login check.
