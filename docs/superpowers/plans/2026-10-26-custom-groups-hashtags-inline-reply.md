# Mattermost 12 fork — Custom Groups, hashtag 2 ký tự, Telegram-style Quote/Reply

**Date:** 2026-10-26  
**Repo:** `ba0f3/mattermost` (`master` examined at `537962b8122c31abf352588808ac3a2a3405f3d6`)  
**Status:** Implementation plan only; no runtime changes in this document.  
**Principle:** Reuse OSS Mattermost code, keep ACL/RBAC/thread semantics, no new service, no touching `server/enterprise` source-available implementation.

## Decision

1. **Group DM:** already works in free Mattermost; nothing to implement.
2. **Custom User Groups** (`@tech`, `@hr`, `@leaders`): existing model, SQL store, API, UI, permission and notifications; implement a narrowly scoped local capability without claiming a paid Mattermost SKU. **Priority P1.**
3. **LDAP/AD Group Sync:** remains out of scope. Keycloak group-claim import can be a separate follow-up. A group's @mention must **not** give anyone new channel/team permissions.
4. **Hashtag length 2:** supported without patching. Set `ServiceSettings.MinimumHashtagLength=2` in runtime configuration; verify two-character search. **Priority P0 / ops.**
5. **Telegram-style reply + quote:** add independent inline reply reference, keep existing Mattermost thread reply unmodified. **Priority P1**.

## 1 — Custom User Groups: code findings

- `server/channels/api4/group.go`: group APIs call `requireLicense(c)`; `licensedAndConfiguredForGroupBySource()` rejects nil license and uses `model.MinimumProfessionalLicense(lic)` for `model.GroupSourceCustom`. LDAP/plugin-sourced groups are gated separately by `Features.LDAPGroups`.
- `server/public/model/config.go`: `ServiceSettings.EnableCustomGroups` already defaults to `true`.
- Existing functionality: `server/channels/app/group.go`; `server/channels/store/sqlstore/group_store.go`; `webapp/channels/src/components/create_user_groups_modal/`; `user_groups_modal/`; `user_group_popover/`.
- `webapp/channels/src/actions/views/create_comment.tsx` consults both LDAP and custom group mention settings; removing an API gate alone does not guarantee mentions/notifications work.
- Important license boundary: only modify code under repository's OSS-compatible licensing terms and preserve notices; do not copy/modify production feature implementations under `server/enterprise` (Mattermost Source Available License). Review applicable fork redistribution and production-use obligations.

### Implementation

1. **Inventory all entry points:** create/get/list/patch/delete/restore/members; group @mention autocomplete, highlighting, websocket, notifications, permission grants, UI navigation, feature selectors.
2. Add fork-owned capability setting, proposed `ServiceSettings.R2DCustomGroupsEnabled`, default `false`. When `false`, upstream behavior remains unchanged; also respect existing `EnableCustomGroups`. Do not forge an Enterprise license or widen unrelated licensed features.
3. In group API gates, authorize only `GroupSourceCustom` when fork flag enabled. For source-unqualified list queries, filter to eligible sources; never implicitly unlock LDAP/plugin sources. Keep session checks, `PermissionCreateCustomGroup`, `PermissionEditCustomGroup`, `PermissionDeleteCustomGroup`, `PermissionManageCustomGroupMembers` and group visibility.
4. Expose a **custom-groups-capable** client property; update relevant UI selectors and `useCustomGroupMentions`-style gates to use this narrow capability, not global `IsLicensed=true` or a fake SKU.
5. Audit server mention resolution/notification: only notify members that can already read the same channel; observe mute and other notification preferences, hard limit for mass mentions, archived group semantics, no duplicate notifications. Group ID is stable; name/mention must be unique/validated.
6. Preserve existing management UX rather than introducing an extra plugin or portal service. Make user-group creation and member management permission-sensitive.
7. Optional **P2** Keycloak group mapping uses stable `sub` / trusted group claims and explicit sync policy; no linking accounts by email alone, no automatic access grant from @mention.

### Mandatory tests

- With **no Mattermost license**: flag off -> original forbidden behavior; flag on -> custom group CRUD, membership and @mention work for authorized roles.
- Direct API attempts by a normal member lacking rights fail; archived group cannot notify; private-channel nonmembers are never notified or admitted by an @mention.
- LDAP sync APIs and professional/enterprise-only unrelated capabilities remain gated.
- Restore/delete concurrency, duplicate handles, huge memberships, empty groups, renamed groups, Web/Desktop visibility, mobile API compatibility.
- Run Go unit/integration + TS component/selector tests before deployment.

## 2 — Hashtag length 2: apply config now

Existing source:

- `server/public/model/config.go`: `MinimumHashtagLength` defaults to `3`, allows positive values.
- `server/config/client.go`: publishes `MinimumHashtagLength` to clients.
- `webapp/channels/src/utils/text_formatting.tsx`: renders hashtags according to configured minimum.
- `webapp/channels/src/utils/text_formatting_hashtags.test.ts`: includes a test with `minimumHashtagLength: 2`.

Apply either:

```json
{ "ServiceSettings": { "MinimumHashtagLength": 2 } }
```

or env `MM_SERVICESETTINGS_MINIMUMHASHTAGLENGTH=2`, respecting the active config management method. No rebuild required; restart/reload according to deployment setup. Confirm `#HR`, `#IT`, `#QA`, `#M5` are clickable, their search finds posts, and `#1` / `#12` do not become unintended tags. Search behavior may depend on PostgreSQL tokenization and index configuration; test end-to-end.

## 3 — Telegram-style inline reply and quote

### UX contract

- Existing Mattermost **Reply in thread** remains available and continues setting `root_id`, using RHS/CRT. Do not change old thread shortcut silently.
- New **Reply in chat**: choose any eligible post in a channel or DM. Composer displays a compact chip with original author + excerpt + dismiss icon. On send, the new message appears in the **current channel timeline**, **not a thread**. Sent post displays a compact quoted preview; click preview scrolls/jumps to original.
- New **Quote selection**: selection of part of a text post produces the same inline reply with only selected excerpt highlighted/displayed. Clear cancel/replace. Truncation and multiline support; keyboard, accessibility and vi/en i18n.
- **Forward** action must remain independent and untouched. One level of preview; avoid recursively rendering quotes inside quotes.
- Rendering layout example:

  ```text
  ↩ Lan · “Cho xin quyền repo game?”
  Đã duyệt, kiểm tra lại nhé.
  ```

### Data model / API

- Reuse `Post.Props` (no DB migration), with a versioned fork namespace such as `r2d_inline_reply: {v:1, post_id, selection?: {start, end}}`. Keep `RootId=""` for inline replies. Do **not** replace `RootId` or `OriginalId` and do not create a second Post record.
- On create/update validate the referenced post exists, requester can view it, it is in **the same channel**, valid post status, allowed type, and selection is in range; reject malformed IDs, recursive references, unacceptable sizes. Do not allow reference to deleted/ephemeral/burn-on-read/sensitive content.
- Store a source reference (and bounded selection coordinates), **not a full persisted snapshot of private/deleted message text**, to avoid content surviving access removal/data retention. When original is edited and offsets no longer match, gracefully fall back to current permitted snippet. If deleted/not accessible show **“Tin nhắn không còn khả dụng”** and do not expose quoted text in the payload.
- Reuse existing posts cache/selectors and authenticated permalink navigation; avoid 1 fetch per visible message, circular render, unbounded history fetch or requesting source messages the viewer cannot access. For legacy/mobile clients without this feature, it remains a normal readable post.

### Touch points

- Menu: `webapp/channels/src/components/dot_menu/dot_menu.tsx` and related post-actions/hotkeys: separate Reply in chat / Quote selection / Reply in thread / Forward.
- Composer: `webapp/channels/src/components/advanced_text_editor/`, `use_submit.tsx`, and `webapp/channels/src/types/store/draft.ts`: quote chip, persist/remove quote per channel draft, send failure/retry, focus/keyboard.
- Submission: `webapp/channels/src/actions/views/create_comment.tsx` already maps `draft.props` into post props; keep behavior for regular posts.
- Message renderer: `webapp/channels/src/components/post_view/post_message_view/`, optional reuse of `post_message_preview/`, plus source lookup and jump-to-post.
- Backend validation: `server/channels/api4/post.go` / `server/channels/app/post.go` / `server/public/model/post.go` according to actual creation/patch pipeline; ensure all clients using API are checked, not web UI only.
- Notifications (P1b): sender of quoted post may get a reply notification only when entitled and per current notification settings. Do not auto-insert an uncontrolled `@mention`; keep standard thread notification paths unchanged.

### Acceptance tests

- Reply in chat in public/private channel, direct message and group DM -> new top-level message, same channel, quote renders, source click works; doesn't increment thread reply count.
- Reply in thread stays completely unchanged; edits, scheduled messages, forwards and permalinks continue to work.
- Selected text: Vietnamese combining marks, emoji, multiline, Markdown, highlight selection and truncation.
- Source edited, deleted, unavailable, authorization revoked, post moved or missing from redux cache -> safe UI and no unauthorized content.
- Cross-channel ID injection, spoofed author, oversized/malformed props, quoted burn-on-read/ephemeral -> forbidden by backend.
- Channel switch, refresh/draft reload, cancel, send error, duplicate retry -> correct quote attached to correct channel only.
- Unit Go/TS, integration for save/read, and web E2E with mention/notification expectations.

## 4 — Delivery order / merge gates

| Milestone | Deliverable | Merge gate |
|---|---|---|
| P0 | Runtime hashtag config = 2 | Clickable and searchable #HR/#IT; no regressions |
| P1a | Custom Groups on OSS fork | Full no-license CRUD/mentions/notification/RBAC tests |
| P1b | Inline reply and whole-message quote | Web/desktop flow + backend reference validation + safe previews |
| P1c | Selected-text quote and notification | Selection integrity + notification policies + E2E |
| P2 | Keycloak group sync and rebuilt mobile UX | Explicit ACL mapping, mobile compatibility |

**Operational rules:** make one PR per milestone; keep feature flags and rollback; no unrelated refactor; no third-party service, dependency or schema migration without a documented need. No claim of functionality until build/tests and runtime verification on an unlicensed instance.
