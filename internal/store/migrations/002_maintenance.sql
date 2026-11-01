create table maintenance_windows (
    id         integer primary key,
    title      text    not null,
    body       text    not null default '',
    starts_at  integer not null,
    duration_s integer not null,
    repeat     text    not null default 'none',
    enabled    integer not null default 1,
    created_at integer not null,
    updated_at integer not null
);

create table maintenance_monitors (
    window_id  integer not null references maintenance_windows (id) on delete cascade,
    monitor_id integer not null references monitors (id) on delete cascade,
    primary key (window_id, monitor_id)
) without rowid;
