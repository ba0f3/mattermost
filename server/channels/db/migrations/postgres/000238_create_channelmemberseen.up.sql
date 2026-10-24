CREATE TABLE IF NOT EXISTS ChannelMemberSeen (
    ChannelId varchar(26) NOT NULL,
    UserId varchar(26) NOT NULL,
    SeenUpTo bigint NOT NULL DEFAULT 0,
    PRIMARY KEY (ChannelId, UserId)
);

-- Start every pointer at the member's current LastViewedAt, capped like the
-- runtime write path by the channel's LastPostAt and by now.
INSERT INTO ChannelMemberSeen (ChannelId, UserId, SeenUpTo)
SELECT
    ChannelMembers.ChannelId,
    ChannelMembers.UserId,
    LEAST(ChannelMembers.LastViewedAt, Channels.LastPostAt, (EXTRACT(EPOCH FROM now()) * 1000)::bigint)
FROM ChannelMembers
INNER JOIN Channels ON Channels.Id = ChannelMembers.ChannelId
WHERE ChannelMembers.LastViewedAt > 0
ON CONFLICT (ChannelId, UserId) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_channelmemberseen_userid ON ChannelMemberSeen (UserId);
