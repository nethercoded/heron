create table expiry (
    host       text    not null,
    kind       text    not null,
    expires_at integer not null default 0,
    checked_at integer not null default 0,
    issuer     text    not null default '',
    error      text    not null default '',
    warned     integer not null default 0,
    primary key (host, kind)
) without rowid;
