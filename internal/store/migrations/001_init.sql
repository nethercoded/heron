create table users (
    id            integer primary key,
    username      text    not null unique collate nocase,
    password_hash text    not null,
    created_at    integer not null
);

create table sessions (
    token_hash blob    primary key,
    user_id    integer not null references users (id) on delete cascade,
    created_at integer not null,
    expires_at integer not null
) without rowid;

create index sessions_expires_at on sessions (expires_at);

create table monitors (
    id              integer primary key,
    slug            text    not null unique,
    name            text    not null,
    description     text    not null default '',
    group_name      text    not null default '',
    kind            text    not null,
    target          text    not null default '',
    interval_s      integer not null,
    timeout_s       integer not null,
    retries         integer not null default 0,
    public          integer not null default 1,
    paused          integer not null default 0,
    position        integer not null default 0,
    settings        text    not null default '{}',
    push_token      text    unique,
    status          text    not null default 'pending',
    status_since    integer not null,
    last_checked_at integer not null default 0,
    last_latency_ms integer not null default 0,
    last_message    text    not null default '',
    created_at      integer not null,
    updated_at      integer not null
);

create table results (
    id         integer primary key,
    monitor_id integer not null references monitors (id) on delete cascade,
    at         integer not null,
    status     text    not null,
    latency_ms integer not null,
    message    text    not null default ''
);

create index results_monitor_at on results (monitor_id, at);
create index results_at on results (at);

create table rollups (
    monitor_id    integer not null references monitors (id) on delete cascade,
    bucket        integer not null,
    total         integer not null default 0,
    down          integer not null default 0,
    degraded      integer not null default 0,
    latency_sum   integer not null default 0,
    latency_count integer not null default 0,
    latency_max   integer not null default 0,
    primary key (monitor_id, bucket)
) without rowid;

create table incidents (
    id          integer primary key,
    title       text    not null,
    impact      text    not null,
    status      text    not null,
    automatic   integer not null default 0,
    started_at  integer not null,
    resolved_at integer,
    created_at  integer not null,
    updated_at  integer not null
);

create index incidents_started_at on incidents (started_at desc);
create index incidents_open on incidents (resolved_at) where resolved_at is null;

create table incident_updates (
    id          integer primary key,
    incident_id integer not null references incidents (id) on delete cascade,
    status      text    not null,
    body        text    not null,
    created_at  integer not null
);

create index incident_updates_incident on incident_updates (incident_id, created_at);

create table incident_monitors (
    incident_id integer not null references incidents (id) on delete cascade,
    monitor_id  integer not null references monitors (id) on delete cascade,
    primary key (incident_id, monitor_id)
) without rowid;

create table channels (
    id         integer primary key,
    name       text    not null,
    kind       text    not null,
    config     text    not null default '{}',
    enabled    integer not null default 1,
    created_at integer not null,
    updated_at integer not null
);

create table monitor_channels (
    monitor_id integer not null references monitors (id) on delete cascade,
    channel_id integer not null references channels (id) on delete cascade,
    primary key (monitor_id, channel_id)
) without rowid;

create table settings (
    key   text primary key,
    value text not null
) without rowid;
