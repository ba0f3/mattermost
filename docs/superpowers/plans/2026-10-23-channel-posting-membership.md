# Channel posting and mandatory membership implementation plan

> **For agentic workers:** Use superpowers:executing-plans to implement and verify each task.

**Goal:** Admins can limit channel posting to admins or selected members, and make team channels mandatory for all team members.

**Architecture:** Persist `posting_policy` (`all`, `admins`, `selected`), `posting_user_ids` and `mandatory` on channels. Enforce changes and posting at the server, reconcile mandatory memberships on enable and team join, and use the same channel fields in the webapp.

**Tech stack:** Go, PostgreSQL, React, TypeScript, existing Mattermost test helpers.

**Spec:** In-chat design approved by the user with “ok triển khai”. Admin means system admin or channel admin. Selected posters must be channel members. Mandatory applies to members of the containing team, including new members, and prevents channel removal while they remain in the team. Team departure can still clean up memberships.

## Constraints

- Existing channels default to unrestricted posting and optional membership.
- Settings are available without a license and editable only by system or channel admins.
- Public and private team channels are supported. DMs, groups and internal channels are excluded.
- Keep mandatory membership incompatible with group-constrained or access-controlled membership to avoid contradictory admission rules.
- Disabling mandatory membership keeps existing channel members.
- Follow server and webapp AGENTS.md instructions.

## Review focus

- Regular members cannot change policy through API calls or alternate channel updates.
- Replies, scheduled posts and integrations cannot bypass posting restrictions.
- Mandatory enable fills existing memberships; new team joins also join mandatory channels.
- Users cannot leave or be removed from mandatory channels while remaining team members.
- Existing settings, archived channels and membership cleanup on team departure continue to work.

## Tasks

- [x] Add model fields, validation, PostgreSQL migration, storage round-trip tests and mandatory membership helpers.
- [x] Enforce admin-only policy changes, posting rules and mandatory leave protection in application/API paths; reject conflicting membership configurations.
- [x] Add settings UI with selected-member picker, read-only composer behavior and mandatory leave-menu behavior; add focused UI tests.
- [x] Add API tests covering role combinations, selected users, persistence, replies, mandatory backfill/new joins/removal, invalid inputs and unchanged defaults.
- [x] Run focused server and webapp tests, type/lint checks, review the full diff and resolve discovered gaps.

## Verification

- Model suite passed; SQL channel persistence, mandatory lookup and cursor pagination tests passed.
- Final API feature and compatibility run passed (130.427s).
- Final application run passed (53.639s), covering policy enforcement, stale-cache updates, enrollment failure/retry, OAuth activation, imports, moves, concurrent team departures, membership cache cleanup and WebSocket compensation events.
- Webapp posting selector tests (13), settings tests (9), affected existing component tests, TypeScript build, ESLint and Stylelint passed.
- Independent review findings were addressed. Full repository and browser end-to-end suites were not run.
