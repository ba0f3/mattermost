ALTER TABLE Channels
    ADD COLUMN IF NOT EXISTS PostingPolicy varchar(16) NOT NULL DEFAULT 'all',
    ADD COLUMN IF NOT EXISTS PostingUserIDs jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS Mandatory boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_channels_teamid_mandatory ON Channels (TeamId)
    WHERE Mandatory = true AND DeleteAt = 0;
