CREATE TABLE IF NOT EXISTS OIDCIdentities (
    IdentityKey VARCHAR(72) PRIMARY KEY,
    UserId VARCHAR(26) NOT NULL UNIQUE,
    Issuer VARCHAR(1024) NOT NULL,
    Subject VARCHAR(255) NOT NULL,
    CreatedAt BIGINT NOT NULL,
    UpdatedAt BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_oidcidentities_userid ON OIDCIdentities (UserId);
