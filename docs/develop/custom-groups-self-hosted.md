# Custom Groups without a paid Mattermost license

Custom User Groups (e.g. `@tech` or `@hr`) work on this Mattermost self-hosted fork without a Professional/Enterprise license. Group DM remains a separate multi-person chat channel type.

- Built-in create/edit/archive/restore, member management and mentions are available when `ServiceSettings.EnableCustomGroups` is true (default).
- The original RBAC, audit, user visibility, channel permissions and notification preferences remain enforced.
- LDAP/AD group synchronization and syncable group APIs are **not** unlocked.
- On servers without a paid license, list/by-name/by-user group lookups and mention resolution exclude LDAP/plugin groups, including old data.
- This change does not generate a fake enterprise license or touch `server/enterprise`.

Rebuild the Go server and the webapp, and reload the client. To turn custom groups off, use `MM_SERVICESETTINGS_ENABLECUSTOMGROUPS=false` or `ServiceSettings.EnableCustomGroups=false` in config. There is no schema migration.

## Smoke test

1. Without a license, create `@tech`, add two users, and confirm CRUD and membership work from UI and API.
2. Mention the group in a channel. Confirm only its **existing channel members** receive a notification.
3. Confirm an ordinary user without group management permissions cannot change membership.
4. Confirm LDAP group-sync API endpoints remain licensed and source-filtered group lookups don't return LDAP groups.
5. Disable `EnableCustomGroups`; verify group operations are denied.
