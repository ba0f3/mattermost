# Telegram-style inline quote and reply

Unlike Mattermost's original threaded replies, **Reply in chat** creates a normal top-level post (`root_id=""`) with an optional versioned reference in `props.r2d_inline_reply`.

## Reference format

```json
{"v":1,"post_id":"26-character-post-id","start":0,"end":10}
```

`start` and `end` are optional UTF-16 code-unit offsets for **Quote selection**. The message body is **never** copied into props. The server validates the reference to an existing non-deleted normal message in the same channel and rejects malformed or overlong selections. A deleted/inaccessible source is shown as unavailable. The legacy **Reply in thread** and **Forward** actions remain unchanged.

## Testing

- From a message menu, choose **Reply in chat**. Composer shows a quote chip; send a message and confirm it remains in the current chat, not in a thread.
- Choose **Quote selection** after selecting visible raw message text. Cancel/replace chip, and verify that changing channels doesn't attach the wrong quote.
- Test DM, group DM, public/private channel and permission-denied cross-channel reference.
- Verify an edited/deleted original gets a safe preview; no text snapshot survives in the reply props.
- Built-in mobile still sees the message text, but needs a separately ported renderer/menu to show the quote card.

Go and webapp tests must pass before merge. No new service, plugin or DB migration.
